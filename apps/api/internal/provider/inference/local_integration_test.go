package inference

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"testing"
	"time"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

// This gate uses an already running, operator-pinned disposable local runtime.
// It never launches a process, downloads a model, or reads infrastructure data.
func TestPinnedLocalRuntimeIntegration(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_LOCAL_INFERENCE") != "true" {
		t.Skip("operator-pinned local inference integration is not configured")
	}
	origin := os.Getenv("STACKPLORE_TEST_INFERENCE_ORIGIN")
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil {
		t.Fatal("integration origin is invalid")
	}
	address, err := netip.ParseAddr(parsed.Hostname())
	if err != nil || !address.Unmap().IsLoopback() {
		t.Fatal("integration requires a literal loopback destination")
	}
	local, err := NewLocal(Profile{
		ID: "operator_integration", Origin: origin, ApprovedAddresses: []netip.Addr{address.Unmap()},
		ExpectedAlias: os.Getenv("STACKPLORE_TEST_INFERENCE_ALIAS"), ExpectedBuild: os.Getenv("STACKPLORE_TEST_INFERENCE_BUILD"),
		ExpectedTemplateSHA256: os.Getenv("STACKPLORE_TEST_INFERENCE_TEMPLATE_SHA256"), Generation: 1,
	}, Options{})
	if err != nil {
		t.Fatal("integration profile is invalid")
	}
	gateway, err := port.New(local, time.Now)
	if err != nil {
		t.Fatal("integration gateway is invalid")
	}
	for _, locale := range []string{"en", "fa"} {
		t.Run(locale, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*port.MaximumCallDuration)
			defer cancel()
			question := "Give one gaps section. No infrastructure evidence is supplied. State insufficient evidence. Use no citations."
			if locale == "fa" {
				question = "تنها یک بخش gaps بده. شاهد زیرساخت داده نشده است. کمبود داده را بگو. ارجاع نساز."
			}
			prepared, err := gateway.Prepare(ctx, port.Request{Locale: locale, Question: question, OutputLimit: 512})
			if err != nil || prepared == nil || prepared.InputTokens() < 1 || prepared.InputTokens() > port.MaximumInputTokens {
				t.Fatal("exact pre-dispatch count failed")
			}
			result, err := gateway.Dispatch(ctx, prepared)
			if err != nil || result.InputTokens != prepared.InputTokens() || result.OutputTokens < 1 || result.OutputTokens > 512 || len(result.Answer.Sections) == 0 {
				t.Fatal("closed local completion or usage validation failed")
			}
			for _, section := range result.Answer.Sections {
				if len(section.Citations) != 0 {
					t.Fatal("synthetic empty-evidence case invented a citation")
				}
			}
			if _, err := gateway.Dispatch(ctx, prepared); err != port.ErrPrepared {
				t.Fatal("real preparation was reused")
			}
		})
	}
}
