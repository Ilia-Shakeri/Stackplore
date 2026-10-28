package assistantevidence

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestTwoBlockedProviderReadsCannotStarveMetadataAuthority(t *testing.T) {
	f := newFixture(t)
	entered := make(chan struct{}, MaximumConcurrent)
	f.live.check = func(ctx context.Context) error { entered <- struct{}{}; <-ctx.Done(); return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, MaximumConcurrent)
	for range MaximumConcurrent {
		go func() { _, err := f.reader.Read(ctx, f.request); finished <- err }()
	}
	for range MaximumConcurrent {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("provider reads did not occupy their bounded lane")
		}
	}
	metadata, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	if err := f.reader.Authorize(metadata, f.request.Actor, f.request.Run, f.request.Scope); err != nil {
		t.Fatal("provider reads starved metadata gate")
	}
	if len(f.reader.slots) != MaximumConcurrent || len(f.reader.metadataSlots) != 0 {
		t.Fatal("metadata used a provider slot")
	}
	cancel()
	for range MaximumConcurrent {
		select {
		case err := <-finished:
			if !errors.Is(err, ErrCancelled) {
				t.Fatal("cancel lost safe code")
			}
		case <-time.After(time.Second):
			t.Fatal("cancel waited for provider slots")
		}
	}
}

func TestSealedAuthorizationQueueHonorsRemainingLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFixture(t)
		base, start := *f.now, time.Now()
		f.reader.now = func() time.Time { return base.Add(time.Since(start)) }
		f.request.Run.LeaseUntil = base.Add(10 * time.Millisecond)
		evidence, err := f.reader.Read(context.Background(), f.request)
		if err != nil {
			t.Fatal(err)
		}
		entered := make(chan struct{}, MaximumConcurrent)
		f.live.check = func(ctx context.Context) error { entered <- struct{}{}; <-ctx.Done(); return ctx.Err() }
		blocking, cancel := context.WithCancel(context.Background())
		defer cancel()
		finished := make(chan error, MaximumConcurrent)
		for index := range MaximumConcurrent {
			request := f.request
			request.Run.ID = fmt.Sprintf("blocked_%d", index)
			request.Scope.RunID = domain.RequestID(request.Run.ID)
			request.Run.LeaseUntil = base.Add(30 * time.Second)
			go func() { _, err := f.reader.Read(blocking, request); finished <- err }()
		}
		for range MaximumConcurrent {
			<-entered
		}
		before := time.Now()
		if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, evidence); !errors.Is(err, ErrCancelled) {
			t.Fatal("lease did not bound provider-slot wait")
		}
		if time.Since(before) > 10*time.Millisecond {
			t.Fatal("provider slot wait outlived lease")
		}
		cancel()
		for range MaximumConcurrent {
			<-finished
		}
	})
}
