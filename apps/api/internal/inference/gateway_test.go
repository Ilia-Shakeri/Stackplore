package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureBackend struct {
	count    func(context.Context, Request) (CountedInput, error)
	generate func(context.Context, CountedInput, int) (Generated, error)
}

func (backend fixtureBackend) Count(ctx context.Context, request Request) (CountedInput, error) {
	return backend.count(ctx, request)
}

func (backend fixtureBackend) Generate(ctx context.Context, input CountedInput, limit int) (Generated, error) {
	return backend.generate(ctx, input, limit)
}

func fixtureRequest() Request {
	return Request{Locale: "fa", Question: "وضعیت منبع چیست؟", Evidence: []Evidence{{ID: "e_1", Text: "Health is unknown."}}, OutputLimit: 512}
}

func fixtureGenerated() Generated {
	return Generated{Content: []byte(`{"sections":[{"kind":"facts","text":"Health is unknown.","citations":["e_1"]}]}`), InputTokens: 3, OutputTokens: 20, Complete: true, Revision: "revision_1"}
}

func fixtureGateway(t *testing.T, change func(*fixtureBackend)) *Gateway {
	t.Helper()
	backend := fixtureBackend{
		count: func(context.Context, Request) (CountedInput, error) {
			return CountedInput{Tokens: []int32{1, 2, 3}, Bytes: 30, Revision: "revision_1"}, nil
		},
		generate: func(context.Context, CountedInput, int) (Generated, error) { return fixtureGenerated(), nil },
	}
	if change != nil {
		change(&backend)
	}
	gateway, err := New(backend, time.Now)
	if err != nil {
		t.Fatal("create gateway fixture")
	}
	return gateway
}

func TestPrepareCountsBeforeOneExplicitDispatch(t *testing.T) {
	t.Parallel()
	var count, calls atomic.Int32
	var counted []int32
	gateway := fixtureGateway(t, func(backend *fixtureBackend) {
		backend.count = func(context.Context, Request) (CountedInput, error) {
			count.Add(1)
			counted = []int32{1, 2, 3}
			return CountedInput{Tokens: counted, Bytes: 30, Revision: "revision_1"}, nil
		}
		backend.generate = func(_ context.Context, input CountedInput, limit int) (Generated, error) {
			calls.Add(1)
			if input.Tokens[0] != 1 || len(input.Tokens) != 3 || limit != 512 {
				t.Error("counted input or output cap changed")
			}
			return fixtureGenerated(), nil
		}
	})
	prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
	if err != nil || prepared.InputTokens() != 3 || count.Load() != 1 || calls.Load() != 0 {
		t.Fatal("count caused inference dispatch")
	}
	counted[0] = 99
	result, err := gateway.Dispatch(context.Background(), prepared)
	if err != nil || len(result.Answer.Sections) != 1 || calls.Load() != 1 {
		t.Fatal("explicit dispatch failed")
	}
	if _, err := gateway.Dispatch(context.Background(), prepared); !errors.Is(err, ErrPrepared) || calls.Load() != 1 {
		t.Fatal("prepared input was reused")
	}
}

func TestConcurrentDispatchHasOneWinner(t *testing.T) {
	t.Parallel()
	var calls, succeeded atomic.Int32
	gateway := fixtureGateway(t, func(backend *fixtureBackend) {
		backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
			calls.Add(1)
			return fixtureGenerated(), nil
		}
	})
	prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			if _, err := gateway.Dispatch(context.Background(), prepared); err == nil {
				succeeded.Add(1)
			}
		})
	}
	workers.Wait()
	if calls.Load() != 1 || succeeded.Load() != 1 {
		t.Fatal("concurrent input dispatched more than once")
	}
}

func TestPreparedValueCopySharesOneUseState(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	gateway := fixtureGateway(t, func(backend *fixtureBackend) {
		backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
			calls.Add(1)
			return fixtureGenerated(), nil
		}
	})
	prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	copyOfHandle := *prepared
	if _, err := gateway.Dispatch(context.Background(), &copyOfHandle); err != nil {
		t.Fatal("copied handle failed its first dispatch")
	}
	if _, err := gateway.Dispatch(context.Background(), prepared); err != ErrPrepared || calls.Load() != 1 {
		t.Fatal("copied handle bypassed one-use consumption")
	}
	if !strings.HasPrefix(fmt.Sprintf("%#v", copyOfHandle), "[protected ") {
		t.Fatal("copied handle exposed volatile input")
	}
}

func TestCanonicalEvidenceReferenceFitsClosedAnswer(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"ev_" + strings.Repeat("a", 64), strings.Repeat("a", MaximumReferenceLength)} {
		request := fixtureRequest()
		request.Evidence[0].ID = id
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
				result := fixtureGenerated()
				result.Content = []byte(`{"sections":[{"kind":"facts","text":"Health is unknown.","citations":["` + id + `"]}]}`)
				return result, nil
			}
		})
		prepared, err := gateway.Prepare(context.Background(), request)
		if err != nil {
			t.Fatal("canonical reference was rejected")
		}
		if _, err := gateway.Dispatch(context.Background(), prepared); err != nil {
			t.Fatal("valid cited answer was rejected")
		}
	}
	request := fixtureRequest()
	request.Evidence[0].ID = strings.Repeat("a", MaximumReferenceLength+1)
	if request.Validate() == nil {
		t.Fatal("overlong reference accepted")
	}
}

func TestFailedDispatchCannotBeRetried(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	gateway := fixtureGateway(t, func(backend *fixtureBackend) {
		backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
			calls.Add(1)
			return Generated{}, errors.New("private upstream detail")
		}
	})
	prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Dispatch(context.Background(), prepared); err != ErrUncertain {
		t.Fatal("failed dispatch did not remain uncertain")
	}
	if _, err := gateway.Dispatch(context.Background(), prepared); err != ErrPrepared || calls.Load() != 1 {
		t.Fatal("uncertain call repeated")
	}
}

func TestPreparationExpiryOwnerAndCancellation(t *testing.T) {
	t.Parallel()
	gateway := fixtureGateway(t, nil)
	prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixtureGateway(t, nil).Dispatch(context.Background(), prepared); err != ErrPrepared {
		t.Fatal("another gateway used preparation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gateway.Dispatch(ctx, prepared); err != ErrCancelled {
		t.Fatal("cancelled request dispatched")
	}
	prepared.expires = time.Now().Add(-time.Second)
	if _, err := gateway.Dispatch(context.Background(), prepared); err != ErrPrepared {
		t.Fatal("expired input dispatched")
	}
	if _, err := gateway.Prepare(ctx, fixtureRequest()); err != ErrCancelled {
		t.Fatal("cancelled input counted")
	}
}

func TestGatewayRejectsUnsafeCountAndGeneration(t *testing.T) {
	t.Parallel()
	for _, input := range []CountedInput{
		{Tokens: nil, Bytes: 1, Revision: "revision_1"},
		{Tokens: []int32{-1}, Bytes: 1, Revision: "revision_1"},
		{Tokens: make([]int32, MaximumInputTokens+1), Bytes: 1, Revision: "revision_1"},
		{Tokens: []int32{1}, Bytes: MaximumInputBytes + 1, Revision: "revision_1"},
		{Tokens: []int32{1}, Bytes: 1, Revision: "bad revision"},
	} {
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			backend.count = func(context.Context, Request) (CountedInput, error) { return input, nil }
		})
		if _, err := gateway.Prepare(context.Background(), fixtureRequest()); err == nil {
			t.Fatal("invalid counted input accepted")
		}
	}
	for _, change := range []func(*Generated){
		func(result *Generated) { result.Revision = "changed" },
		func(result *Generated) { result.InputTokens = 0 },
		func(result *Generated) { result.OutputTokens = 513 },
		func(result *Generated) { result.OutputTokens = -1 },
		func(result *Generated) { result.Complete = false },
		func(result *Generated) { result.Content = make([]byte, MaximumOutputBytes+1) },
	} {
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
				result := fixtureGenerated()
				change(&result)
				return result, nil
			}
		})
		prepared, err := gateway.Prepare(context.Background(), fixtureRequest())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := gateway.Dispatch(context.Background(), prepared); err == nil {
			t.Fatal("invalid generation accepted")
		}
	}
}

func TestAnswerSchemaAndCitationValidation(t *testing.T) {
	t.Parallel()
	for _, content := range []string{
		`{"sections":[],"extra":true}`,
		`{"sections":[{"kind":"facts","text":"Known.","citations":[]}]}`,
		`{"sections":[{"kind":"facts","text":"Known.","citations":["missing"]}]}`,
		`{"sections":[{"kind":"facts","text":"Known.","citations":["e_1","e_1"]}]}`,
		`{"sections":[{"kind":"gaps","text":"Missing.","citations":null}]}`,
		`{"sections":[{"kind":"exec","text":"Bad.","citations":[]}]}`,
		`{"sections":[{"kind":"gaps","text":"<img src=x>","citations":[]}]}`,
		`{"sections":[{"kind":"gaps","text":"[go](https://example.test)","citations":[]}]}`,
		`{"sections":[{"kind":"gaps","text":"Missing.","citations":[],"action":"go"}]}`,
		`{"sections":[{"kind":"gaps","text":"Missing.","text":"Changed.","citations":[]}]}`,
		`{"sections":[],"sections":[{"kind":"gaps","text":"Missing.","citations":[]}]}`,
		`{"sections":[{"kind":"gaps","text":"Missing.","citations":[]}]} {}`,
	} {
		if _, err := parseAnswer([]byte(content), map[string]struct{}{"e_1": {}}); err == nil {
			t.Fatal("invalid answer schema or citation accepted")
		}
	}
	if _, err := parseAnswer([]byte(`{"sections":[{"kind":"gaps","text":"داده کافی نیست.","citations":[]}]}`), nil); err != nil {
		t.Fatal("valid localized gap rejected")
	}
	var schema map[string]any
	if json.Unmarshal(AnswerSchema(), &schema) != nil || schema["additionalProperties"] != false {
		t.Fatal("schema is not closed")
	}
}

func TestGatewayBoundsCancellationAndSafeErrors(t *testing.T) {
	t.Parallel()
	if _, err := New(nil, time.Now); err != ErrInvalid {
		t.Fatal("missing backend accepted")
	}
	if _, err := New(fixtureBackend{}, nil); err != ErrInvalid {
		t.Fatal("missing clock accepted")
	}
	gateway := fixtureGateway(t, func(backend *fixtureBackend) {
		backend.count = func(context.Context, Request) (CountedInput, error) {
			return CountedInput{}, errors.New("private upstream detail")
		}
	})
	if _, err := gateway.Prepare(context.Background(), fixtureRequest()); err != ErrUnavailable {
		t.Fatal("private count failure escaped")
	}
	if _, err := gateway.Prepare(nil, fixtureRequest()); err != ErrInvalid {
		t.Fatal("nil count context accepted")
	}
	if _, err := gateway.Dispatch(nil, nil); err != ErrPrepared {
		t.Fatal("nil dispatch accepted")
	}
	var empty *Prepared
	if empty.InputTokens() != 0 {
		t.Fatal("nil input exposes tokens")
	}
	gateway = fixtureGateway(t, func(backend *fixtureBackend) {
		backend.count = func(ctx context.Context, _ Request) (CountedInput, error) {
			<-ctx.Done()
			return CountedInput{}, ctx.Err()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := gateway.Prepare(ctx, fixtureRequest()); err != ErrCancelled {
		t.Fatal("count ignored deadline")
	}
}

func TestInputValidationAndProtectedFormatting(t *testing.T) {
	t.Parallel()
	for _, change := range []func(*Request){
		func(request *Request) { request.Locale = "xx" },
		func(request *Request) { request.Question = "" },
		func(request *Request) { request.Question = strings.Repeat("x", 8193) },
		func(request *Request) { request.OutputLimit = MaximumOutputTokens + 1 },
		func(request *Request) { request.Evidence = append(request.Evidence, request.Evidence[0]) },
		func(request *Request) { request.Evidence[0].ID = "bad/id" },
	} {
		request := fixtureRequest()
		change(&request)
		if request.Validate() == nil {
			t.Fatal("invalid request accepted")
		}
	}
	for _, protected := range []any{fixtureRequest(), CountedInput{Tokens: []int32{123}}, fixtureGenerated(), &Prepared{}, Result{Answer: Answer{Sections: []Section{{Text: "private fixture content"}}}}} {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if !strings.HasPrefix(fmt.Sprintf(format, protected), "[protected ") {
				t.Fatal("protected input escaped formatting")
			}
		}
		if body, err := json.Marshal(protected); err != nil || string(body) != "{}" {
			t.Fatal("protected input escaped serialization")
		}
	}
}
