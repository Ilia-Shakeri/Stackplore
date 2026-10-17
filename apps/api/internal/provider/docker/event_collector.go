package docker

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/moby/moby/client"
)

func (collector *EngineCollector) EngineEvents(ctx context.Context, start, end time.Time) (DockerEventBatch, error) {
	if collector == nil || collector.sdk == nil || ctx == nil || ctx.Err() != nil ||
		start.Location() != time.UTC || end.Location() != time.UTC || !start.Before(end) {
		return DockerEventBatch{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	now := collector.now().UTC().Truncate(time.Microsecond)
	if end.After(now) {
		end = now
	}
	if !start.Before(end) {
		return DockerEventBatch{Truncated: true}, nil
	}
	streamContext, cancel := context.WithCancel(ctx)
	defer cancel()
	// Inspect the unfiltered retained ring. Filtering at the daemon can conceal
	// overflow from other event types and incorrectly certify an earlier window.
	stream := collector.sdk.Events(streamContext, client.EventsListOptions{Since: "0", Until: end.Format(time.RFC3339Nano)})
	batch := DockerEventBatch{Events: make([]DockerEvent, 0, min(collector.maximumItems, 256))}
	var earliest time.Time
	invalidTime := false
	for {
		select {
		case <-ctx.Done():
			return DockerEventBatch{}, engineError(ctx, ctx.Err())
		case err, open := <-stream.Err:
			if !open || errors.Is(err, io.EOF) {
				// An empty ring, a newer earliest event, and pre-1.46 omissions
				// cannot establish complete historical coverage.
				batch.Truncated = invalidTime || earliest.IsZero() || earliest.After(start) || collector.sdk.ClientVersion() < "1.46"
				return batch, nil
			}
			if errors.Is(err, errEngineResponseLimit) && len(batch.Events) > 0 {
				batch.Truncated = true
				return batch, nil
			}
			return DockerEventBatch{}, engineError(ctx, err)
		case item := <-stream.Messages:
			if len(batch.Events) >= collector.maximumItems {
				batch.Truncated = true
				return batch, nil
			}
			occurredAt := time.Unix(item.Time, 0)
			if item.TimeNano > 0 {
				occurredAt = time.Unix(0, item.TimeNano)
			}
			occurredAt = occurredAt.UTC().Truncate(time.Microsecond)
			if occurredAt.Unix() <= 0 || occurredAt.After(now) {
				invalidTime = true
				continue
			}
			if earliest.IsZero() || occurredAt.Before(earliest) {
				earliest = occurredAt
			}
			if occurredAt.Before(start) || !occurredAt.Before(end) {
				continue
			}
			switch string(item.Type) {
			case "container", "image", "network", "volume", "daemon":
			default:
				continue
			}
			batch.Events = append(batch.Events, DockerEvent{Type: string(item.Type), Action: string(item.Action), ActorID: item.Actor.ID, OccurredAt: occurredAt})
		}
	}
}

var _ EventCollector = (*EngineCollector)(nil)
