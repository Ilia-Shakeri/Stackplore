package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/client"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (collector *EngineCollector) ReadContainerLogs(ctx context.Context, request ContainerLogRequest) ([]ContainerLogLine, error) {
	batch, err := collector.ReadContainerLogBatch(ctx, request)
	return batch.Lines, err
}

type ContainerLogBatch struct {
	Lines     []ContainerLogLine
	Truncated bool
}

func (collector *EngineCollector) ReadContainerLogBatch(ctx context.Context, request ContainerLogRequest) (ContainerLogBatch, error) {
	if collector == nil || collector.sdk == nil || ctx == nil || ctx.Err() != nil || len(request.ContainerIDs) > domain.MaxLogTargets ||
		request.LineLimit < 1 || request.LineLimit > domain.MaxLogLines || request.ByteLimit < 1 || request.ByteLimit > domain.MaxLogBytes ||
		request.Start.Location() != time.UTC || request.End.Location() != time.UTC || !request.Start.Before(request.End) {
		return ContainerLogBatch{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	ctx = engineObservationContext(ctx)
	result := make([]ContainerLogLine, 0, request.LineLimit)
	seen := make(map[string]struct{}, len(request.ContainerIDs))
	bytesUsed := 0
	wireBytes := 0
	truncated := false
	for _, identifier := range request.ContainerIDs {
		if !safeDockerID(identifier) {
			return ContainerLogBatch{}, ErrUnavailable
		}
		if _, exists := seen[identifier]; exists {
			return ContainerLogBatch{}, ErrUnavailable
		}
		seen[identifier] = struct{}{}
		inspection, err := collector.sdk.ContainerInspect(ctx, identifier, client.ContainerInspectOptions{})
		if err != nil || inspection.Container.Config == nil {
			return ContainerLogBatch{}, engineError(ctx, firstError(err, ErrUnavailable))
		}
		if expected := request.ContainerNames[identifier]; expected != "" && expected != strings.TrimPrefix(inspection.Container.Name, "/") {
			return ContainerLogBatch{}, ErrUnsupported
		}
		remaining := request.ByteLimit - wireBytes
		if remaining <= 0 {
			return ContainerLogBatch{Lines: result, Truncated: true}, nil
		}
		stream, err := collector.sdk.ContainerLogs(ctx, identifier, client.ContainerLogsOptions{
			ShowStdout: true, ShowStderr: true, Timestamps: true,
			Since: request.Start.Format(time.RFC3339Nano), Until: request.End.Format(time.RFC3339Nano), Tail: strconv.Itoa(request.LineLimit),
		})
		if err != nil {
			return ContainerLogBatch{}, engineError(ctx, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, int64(remaining)+1))
		_ = stream.Close()
		responseTruncated := len(data) > remaining || errors.Is(readErr, errEngineResponseLimit)
		if readErr != nil && !responseTruncated {
			return ContainerLogBatch{}, engineError(ctx, readErr)
		}
		if len(data) > remaining {
			data = data[:remaining]
		}
		wireBytes += len(data)
		truncated = truncated || responseTruncated
		lines, malformed := parseDockerLogPayloadChecked(identifier, data, inspection.Container.Config.Tty)
		truncated = truncated || malformed
		if responseTruncated && len(lines) > 0 {
			lines[len(lines)-1].Truncated = true
		}
		for _, line := range lines {
			if line.Timestamp.Before(request.Start) || !line.Timestamp.Before(request.End) {
				continue
			}
			lineBytes := len(line.Message)
			if len(result) >= request.LineLimit || lineBytes > request.ByteLimit-bytesUsed {
				return ContainerLogBatch{Lines: result, Truncated: true}, nil
			}
			bytesUsed += lineBytes
			result = append(result, line)
		}
		if len(result) >= request.LineLimit || bytesUsed >= request.ByteLimit {
			truncated = true
			break
		}
	}
	return ContainerLogBatch{Lines: result, Truncated: truncated}, nil
}

func parseDockerLogPayload(containerID string, data []byte, tty bool) []ContainerLogLine {
	lines, _ := parseDockerLogPayloadChecked(containerID, data, tty)
	return lines
}

func parseDockerLogPayloadChecked(containerID string, data []byte, tty bool) ([]ContainerLogLine, bool) {
	if tty {
		return parseStampedLogLinesChecked(containerID, domain.LogStreamUnknown, data)
	}
	var stdout, stderr bytes.Buffer
	malformed := false
	for len(data) >= 8 {
		length := int(binary.BigEndian.Uint32(data[4:8]))
		if length < 0 || length > len(data)-8 {
			malformed = true
			break
		}
		if data[1] != 0 || data[2] != 0 || data[3] != 0 {
			malformed = true
			break
		}
		switch data[0] {
		case 1:
			_, _ = stdout.Write(data[8 : 8+length])
		case 2:
			_, _ = stderr.Write(data[8 : 8+length])
		default:
			malformed = true
			data = nil
			break
		}
		if malformed {
			break
		}
		data = data[8+length:]
	}
	result, stdoutMalformed := parseStampedLogLinesChecked(containerID, domain.LogStreamStdout, stdout.Bytes())
	errorLines, stderrMalformed := parseStampedLogLinesChecked(containerID, domain.LogStreamStderr, stderr.Bytes())
	return append(result, errorLines...), malformed || len(data) > 0 || stdoutMalformed || stderrMalformed
}

func parseStampedLogLinesChecked(containerID string, stream domain.LogStreamKind, data []byte) ([]ContainerLogLine, bool) {
	parts := strings.Split(string(data), "\n")
	result := make([]ContainerLogLine, 0, len(parts))
	malformed := false
	for _, part := range parts {
		if part == "" {
			continue
		}
		timestampText, message, found := strings.Cut(part, " ")
		if !found {
			malformed = true
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
			malformed = true
			continue
		}
		truncated := len(message) > domain.MaxLogLineBytes
		if truncated {
			message = message[:domain.MaxLogLineBytes]
		}
		result = append(result, ContainerLogLine{
			ContainerID: containerID, Stream: stream,
			Timestamp: timestamp.UTC().Truncate(time.Microsecond), Message: message, Truncated: truncated,
		})
	}
	return result, malformed
}

var _ LogCollector = (*EngineCollector)(nil)
