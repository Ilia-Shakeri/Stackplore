package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"time"
)

const dockerEventTypeFilter = `{"type":["container","image","network","volume","daemon"]}`

type engineEvent struct {
	Type     string `json:"Type"`
	Action   string `json:"Action"`
	Time     int64  `json:"time"`
	TimeNano int64  `json:"timeNano"`
	Actor    struct {
		ID string `json:"ID"`
	} `json:"Actor"`
}

func (collector *EngineCollector) EngineEvents(ctx context.Context, start, end time.Time) (DockerEventBatch, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil ||
		start.Location() != time.UTC || end.Location() != time.UTC || !start.Before(end) {
		return DockerEventBatch{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	now := collector.now().UTC().Truncate(time.Microsecond)
	if end.After(now) {
		end = now
	}
	if !start.Before(end) {
		return DockerEventBatch{}, nil
	}
	values := url.Values{
		"since":   {strconv.FormatInt(start.Unix(), 10)},
		"until":   {strconv.FormatInt(end.Unix(), 10)},
		"filters": {dockerEventTypeFilter},
	}
	data, responseTruncated, err := collector.getRaw(ctx, "/events?"+values.Encode(), int(collector.maximumResponse))
	if err != nil {
		return DockerEventBatch{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	batch := DockerEventBatch{Events: make([]DockerEvent, 0, min(collector.maximumItems, 256)), Truncated: responseTruncated}
	for {
		var item engineEvent
		err := decoder.Decode(&item)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if responseTruncated && len(batch.Events) > 0 {
				batch.Truncated = true
				break
			}
			return DockerEventBatch{}, ErrUnavailable
		}
		if len(batch.Events) >= collector.maximumItems {
			batch.Truncated = true
			break
		}
		occurredAt := time.Unix(item.Time, 0)
		if item.TimeNano > 0 {
			occurredAt = time.Unix(0, item.TimeNano)
		}
		batch.Events = append(batch.Events, DockerEvent{
			Type: item.Type, Action: item.Action, ActorID: item.Actor.ID,
			OccurredAt: occurredAt.UTC().Truncate(time.Microsecond),
		})
	}
	return batch, nil
}

var _ EventCollector = (*EngineCollector)(nil)
