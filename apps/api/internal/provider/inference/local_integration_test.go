package inference

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	contextTokens, err := strconv.Atoi(os.Getenv("STACKPLORE_TEST_INFERENCE_CONTEXT_TOKENS"))
	if err != nil || strconv.Itoa(contextTokens) != os.Getenv("STACKPLORE_TEST_INFERENCE_CONTEXT_TOKENS") {
		t.Fatal("integration context pin is invalid")
	}
	local, err := NewLocal(Profile{
		ID: "operator_integration", Origin: origin, ApprovedAddresses: []netip.Addr{address.Unmap()},
		ExpectedAlias: os.Getenv("STACKPLORE_TEST_INFERENCE_ALIAS"), ExpectedBuild: os.Getenv("STACKPLORE_TEST_INFERENCE_BUILD"),
		ExpectedTemplateSHA256: os.Getenv("STACKPLORE_TEST_INFERENCE_TEMPLATE_SHA256"), ExpectedContextTokens: contextTokens, Generation: 1,
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
			started := time.Now()
			inputTokens, outputTokens := 0, 0
			outcome := "fail"
			t.Cleanup(func() {
				t.Logf("case=empty_evidence_%s input_tokens=%d output_tokens=%d elapsed_ms=%d outcome=%s", locale, inputTokens, outputTokens, time.Since(started).Milliseconds(), outcome)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*port.MaximumCallDuration)
			defer cancel()
			question := "No infrastructure evidence is supplied. Return exactly one gaps section, with no citations. Its text must be exactly: " + noEvidenceProbeText(locale)
			if locale == "fa" {
				question = "شاهد زیرساخت داده نشده است. تنها یک بخش gaps بدون ارجاع بده. متن آن دقیقاً این باشد: " + noEvidenceProbeText(locale)
			}
			prepared, err := gateway.Prepare(ctx, port.Request{Locale: locale, Question: question, OutputLimit: 512})
			if err != nil || prepared == nil || prepared.InputTokens() < 1 || prepared.InputTokens() > port.MaximumInputTokens {
				t.Fatal("exact pre-dispatch count failed")
			}
			inputTokens = prepared.InputTokens()
			result, err := gateway.Dispatch(ctx, prepared)
			outputTokens = result.OutputTokens
			if err != nil || result.InputTokens != inputTokens || result.OutputTokens < 1 || result.OutputTokens > 512 {
				t.Fatal("closed local completion or usage validation failed")
			}
			if !validNoEvidenceProbe(locale, result.Answer) {
				t.Fatal("synthetic empty-evidence semantics or locale failed")
			}
			if _, err := gateway.Dispatch(ctx, prepared); err != port.ErrPrepared {
				t.Fatal("real preparation was reused")
			}
			outcome = "pass"
		})
	}
}

// This controlled probe checks protocol and locale obedience, not answer quality.
func noEvidenceProbeText(locale string) string {
	switch locale {
	case "en":
		return "Insufficient infrastructure evidence."
	case "fa":
		return "شواهد زیرساخت کافی نیست."
	default:
		return ""
	}
}

func validNoEvidenceProbe(locale string, answer port.Answer) bool {
	text := noEvidenceProbeText(locale)
	if text == "" || len(answer.Sections) != 1 {
		return false
	}
	section := answer.Sections[0]
	return section.Kind == port.SectionGaps && strings.TrimSpace(section.Text) == text && section.Citations != nil && len(section.Citations) == 0
}

func TestNoEvidenceProbeRejectsWrongLocaleAndMeaning(t *testing.T) {
	t.Parallel()
	for _, locale := range []string{"en", "fa"} {
		valid := port.Answer{Sections: []port.Section{{Kind: port.SectionGaps, Text: noEvidenceProbeText(locale), Citations: []string{}}}}
		if !validNoEvidenceProbe(locale, valid) {
			t.Fatal("valid localized empty-evidence probe rejected")
		}
		other := "en"
		if locale == "en" {
			other = "fa"
		}
		for _, invalid := range []port.Answer{
			{},
			{Sections: []port.Section{{Kind: port.SectionGaps, Text: noEvidenceProbeText(other), Citations: []string{}}}},
			{Sections: []port.Section{{Kind: port.SectionGaps, Text: "Healthy.", Citations: []string{}}}},
			{Sections: []port.Section{{Kind: port.SectionHypotheses, Text: noEvidenceProbeText(locale), Citations: []string{}}}},
			{Sections: []port.Section{{Kind: port.SectionGaps, Text: noEvidenceProbeText(locale), Citations: []string{"invented"}}}},
			{Sections: []port.Section{{Kind: port.SectionGaps, Text: noEvidenceProbeText(locale)}}},
			{Sections: append(append([]port.Section{}, valid.Sections...), valid.Sections...)},
		} {
			if validNoEvidenceProbe(locale, invalid) {
				t.Fatal("invalid localized empty-evidence probe accepted")
			}
		}
		if validNoEvidenceProbe("de", valid) {
			t.Fatal("unsupported probe locale accepted")
		}
	}
}
