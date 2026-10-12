package docker

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (collector *EngineCollector) ReadContainerLogs(ctx context.Context, request ContainerLogRequest) ([]ContainerLogLine, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil || len(request.ContainerIDs) > domain.MaxLogTargets ||
		request.LineLimit < 1 || request.LineLimit > domain.MaxLogLines || request.ByteLimit < 1 || request.ByteLimit > domain.MaxLogBytes ||
		request.Start.Location() != time.UTC || request.End.Location() != time.UTC || !request.Start.Before(request.End) {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	result := make([]ContainerLogLine, 0, request.LineLimit)
	seen := make(map[string]struct{}, len(request.ContainerIDs))
	bytesUsed := 0
	for _, identifier := range request.ContainerIDs {
		if !safeDockerID(identifier) {
			return nil, ErrUnavailable
		}
		if _, exists := seen[identifier]; exists {
			return nil, ErrUnavailable
		}
		seen[identifier] = struct{}{}
		var inspection engineContainerInspect
		if err := collector.getJSON(ctx, "/containers/"+identifier+"/json", &inspection); err != nil {
			return nil, err
		}
		values := url.Values{
			"follow": {"false"}, "stdout": {"true"}, "stderr": {"true"}, "timestamps": {"true"},
			"since": {strconv.FormatInt(request.Start.Unix(), 10)}, "until": {strconv.FormatInt(request.End.Unix(), 10)},
			"tail": {strconv.Itoa(request.LineLimit)},
		}
		remaining := request.ByteLimit - bytesUsed
		data, responseTruncated, err := collector.getRaw(ctx, "/containers/"+identifier+"/logs?"+values.Encode(), remaining)
		if err != nil {
			return nil, err
		}
		lines := parseDockerLogPayload(identifier, data, inspection.Config.TTY)
		if responseTruncated && len(lines) > 0 {
			lines[len(lines)-1].Truncated = true
		}
		for _, line := range lines {
			if line.Timestamp.Before(request.Start) || !line.Timestamp.Before(request.End) {
				continue
			}
			lineBytes := len(line.Message)
			if len(result) >= request.LineLimit || lineBytes > request.ByteLimit-bytesUsed {
				return result, nil
			}
			bytesUsed += lineBytes
			result = append(result, line)
		}
		if len(result) >= request.LineLimit || bytesUsed >= request.ByteLimit {
			break
		}
	}
	return result, nil
}

func (collector *EngineCollector) getRaw(ctx context.Context, path string, maximum int) ([]byte, bool, error) {
	if maximum < 1 {
		return nil, false, ErrUnavailable
	}
	maximum = min(maximum, int(collector.maximumResponse))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, collector.baseURL+path, nil)
	if err != nil {
		return nil, false, ErrUnavailable
	}
	request.Header.Set("Accept", "application/vnd.docker.raw-stream, text/plain")
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, errors.Join(ErrUnavailable, ctx.Err())
		}
		return nil, false, errors.Join(ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, false, ErrPermissionDenied
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, false, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(maximum)+1))
	if err != nil {
		return nil, false, ErrUnavailable
	}
	truncated := len(data) > maximum
	if truncated {
		data = data[:maximum]
	}
	return data, truncated, nil
}

func parseDockerLogPayload(containerID string, data []byte, tty bool) []ContainerLogLine {
	if tty {
		return parseStampedLogLines(containerID, domain.LogStreamUnknown, data)
	}
	result := make([]ContainerLogLine, 0)
	for len(data) >= 8 {
		length := int(binary.BigEndian.Uint32(data[4:8]))
		if length < 0 || length > len(data)-8 {
			break
		}
		stream := domain.LogStreamSystem
		switch data[0] {
		case 1:
			stream = domain.LogStreamStdout
		case 2:
			stream = domain.LogStreamStderr
		}
		result = append(result, parseStampedLogLines(containerID, stream, data[8:8+length])...)
		data = data[8+length:]
	}
	return result
}

func parseStampedLogLines(containerID string, stream domain.LogStreamKind, data []byte) []ContainerLogLine {
	parts := strings.Split(string(data), "\n")
	result := make([]ContainerLogLine, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		timestampText, message, found := strings.Cut(part, " ")
		if !found {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
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
	return result
}

var _ LogCollector = (*EngineCollector)(nil)
