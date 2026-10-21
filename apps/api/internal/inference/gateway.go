package inference

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync/atomic"
	"time"
)

type Gateway struct {
	backend Backend
	now     func() time.Time
}

// Prepared is a volatile one-use capability, never a transport or stored value.
type Prepared struct {
	owner    *Gateway
	input    CountedInput
	evidence map[string]struct{}
	limit    int
	expires  time.Time
	used     *atomic.Bool
}

func (Prepared) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected prepared input]")
}
func (Prepared) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (prepared *Prepared) InputTokens() int {
	if prepared == nil {
		return 0
	}
	return len(prepared.input.Tokens)
}

func New(backend Backend, now func() time.Time) (*Gateway, error) {
	if backend == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Gateway{backend: backend, now: now}, nil
}

// Prepare counts exact formatted input without requesting generation.
func (gateway *Gateway) Prepare(ctx context.Context, request Request) (*Prepared, error) {
	if gateway == nil || ctx == nil {
		return nil, ErrInvalid
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled
	}
	work, cancel := context.WithTimeout(ctx, MaximumCallDuration)
	defer cancel()
	input, err := gateway.backend.Count(work, request)
	if work.Err() != nil {
		return nil, ErrCancelled
	}
	if err != nil {
		if err == ErrInputLimit || err == ErrIdentity || err == ErrInvalid || err == ErrCancelled {
			return nil, err
		}
		return nil, ErrUnavailable
	}
	if input.Bytes < 1 || input.Bytes > MaximumInputBytes || len(input.Tokens) < 1 || len(input.Tokens) > MaximumInputTokens || !validID(input.Revision) {
		return nil, ErrInputLimit
	}
	for _, token := range input.Tokens {
		if token < 0 {
			return nil, ErrInvalid
		}
	}
	input.Tokens = slices.Clone(input.Tokens)
	refs := make(map[string]struct{}, len(request.Evidence))
	for _, item := range request.Evidence {
		refs[item.ID] = struct{}{}
	}
	return &Prepared{owner: gateway, input: input, evidence: refs, limit: request.OutputLimit, expires: gateway.now().Add(PreparationLifetime), used: new(atomic.Bool)}, nil
}

// Dispatch consumes its preparation before any external call. No failed call is retried.
func (gateway *Gateway) Dispatch(ctx context.Context, prepared *Prepared) (Result, error) {
	if gateway == nil || ctx == nil || prepared == nil || prepared.used == nil || prepared.owner != gateway || !gateway.now().Before(prepared.expires) {
		return Result{}, ErrPrepared
	}
	if ctx.Err() != nil {
		return Result{}, ErrCancelled
	}
	if !prepared.used.CompareAndSwap(false, true) {
		return Result{}, ErrPrepared
	}
	defer clear(prepared.input.Tokens)
	work, cancel := context.WithTimeout(ctx, MaximumCallDuration)
	defer cancel()
	input := prepared.input
	input.Tokens = slices.Clone(input.Tokens)
	generated, err := gateway.backend.Generate(work, input, prepared.limit)
	clear(input.Tokens)
	defer clear(generated.Content)
	if err != nil || work.Err() != nil {
		return Result{}, ErrUncertain
	}
	if generated.Revision != prepared.input.Revision {
		return Result{}, ErrIdentity
	}
	if generated.InputTokens != len(prepared.input.Tokens) || generated.OutputTokens < 1 || generated.OutputTokens > prepared.limit || !generated.Complete {
		return Result{}, ErrOutputLimit
	}
	answer, err := parseAnswer(generated.Content, prepared.evidence)
	if err != nil {
		return Result{}, err
	}
	return Result{Answer: answer, InputTokens: generated.InputTokens, OutputTokens: generated.OutputTokens, Revision: generated.Revision}, nil
}
