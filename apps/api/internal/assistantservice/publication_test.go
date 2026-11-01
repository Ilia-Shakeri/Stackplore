package assistantservice

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestDurableCompletionWaitsForOwnedPublicationWithoutFalseContentLoss(t *testing.T) {
	t.Parallel()
	persisted, release := make(chan struct{}), make(chan struct{})
	service, _, _, _ := fixtureService(t, func(options *Options, _ *modelFixture) {
		base := options.Repository.(*fakeRepository)
		options.Repository = &faultRepository{fakeRepository: base, afterFinish: func() { close(persisted); <-release }}
	})
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.runOne(context.Background(), "worker"); done <- err }()
	waitSignal(t, persisted)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := service.Get(ctx, input.Admission.Actor, input.Admission.ID)
	close(release)
	if err != assistant.ErrUnavailable {
		t.Fatal("unpublished completion was exposed as permanent loss")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("owned publication did not finish")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Answer == nil || !snapshot.Volatile {
		t.Fatal("fresh completed publication disappeared")
	}
}
