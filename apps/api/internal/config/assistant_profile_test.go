package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type assistantSnapshotGetter struct{ entries []string }

func (g assistantSnapshotGetter) Environ() []string { return slices.Clone(g.entries) }
func (assistantSnapshotGetter) LookupEnv(string) (string, bool) {
	panic("snapshot parser must not use a second environment read")
}

func assistantProfileEnvironment() map[string]string {
	return map[string]string{
		assistantEnabledKey: "true", assistantOriginKey: "http://127.0.0.1:59641",
		assistantModelSHA256Key: strings.Repeat("a", 64), assistantBuildKey: "synthetic-build-1",
		assistantTemplateSHA256Key: strings.Repeat("b", 64), assistantContextTokensKey: "24576", assistantGenerationKey: "1",
	}
}

func assistantProfileSnapshot(values map[string]string) assistantSnapshotGetter {
	entries := make([]string, 0, len(values))
	for key, value := range values {
		entries = append(entries, key+"="+value)
	}
	return assistantSnapshotGetter{entries: entries}
}

func TestAssistantProfileDisabledAndSingleSnapshot(t *testing.T) {
	for _, entries := range [][]string{nil, {"UNRELATED=value"}, {assistantEnabledKey + "=false"}} {
		profile, err := LoadAssistantProfile(assistantSnapshotGetter{entries: entries})
		if err != nil || profile.Enabled() || profile.Limits() != (AssistantLimits{}) || profile.Validate() != nil {
			t.Fatal("disabled profile changed or failed validation")
		}
		if local, enabled := profile.Local(); enabled || local != (AssistantLocalProfile{}) {
			t.Fatal("disabled profile exposed an address")
		}
	}
	if _, err := LoadAssistantProfile(nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("missing environment accepted")
	}
	profile, err := LoadAssistantProfile(assistantProfileSnapshot(assistantProfileEnvironment()))
	if err != nil || !profile.Enabled() || profile.Validate() != nil {
		t.Fatal("valid snapshot refused")
	}
	local, enabled := profile.Local()
	if !enabled || local.Address != netip.MustParseAddr("127.0.0.1") || local.ID != "local" || local.Generation != 1 || local.ModelAlias != "sha256-"+strings.Repeat("a", 64) || local.ExpectedContextTokens != 24576 {
		t.Fatal("trusted identity pins changed")
	}
	limits := profile.Limits()
	if limits.InputBytes != inference.MaximumInputBytes || limits.InputTokens != inference.MaximumInputTokens || limits.OutputBytes != inference.MaximumOutputBytes || limits.OutputTokens != inference.MaximumOutputTokens || limits.CallTimeout != inference.MaximumCallDuration || limits.RunTimeout != assistant.MaximumRunDuration || limits.InferenceCalls != assistant.MaximumInferenceCalls || limits.ToolCalls != assistant.MaximumToolCalls {
		t.Fatal("gateway or runtime limit drift")
	}
	local.Origin = "https://example.invalid"
	again, _ := profile.Local()
	if again.Origin == local.Origin {
		t.Fatal("composition copy mutated stored profile")
	}
}

func TestAssistantProfileRejectsAmbiguousSnapshot(t *testing.T) {
	good := assistantProfileSnapshot(assistantProfileEnvironment()).entries
	cases := map[string][]string{
		"duplicate":                   append(slices.Clone(good), assistantEnabledKey+"=true"),
		"lowercase":                   append(slices.Clone(good), strings.ToLower(assistantEnabledKey)+"=true"),
		"unknown":                     append(slices.Clone(good), assistantEnvironmentPrefix+"ENDPOINT=https://example.invalid"),
		"missing_separator":           {assistantEnabledKey},
		"empty":                       {assistantEnabledKey + "="},
		"whitespace":                  {assistantEnabledKey + "= true"},
		"numeric_boolean":             {assistantEnabledKey + "=1"},
		"nul":                         {assistantEnabledKey + "=true\x00"},
		"invalid_utf8":                {assistantEnabledKey + "=" + string([]byte{0xff})},
		"disabled_latent_input":       {assistantEnabledKey + "=false", assistantOriginKey + "=http://127.0.0.1:59641"},
		"absent_enabled_latent_input": {assistantOriginKey + "=http://127.0.0.1:59641"},
		"too_many_entries":            make([]string, maximumAssistantEnvironmentEntries+1),
		"entry_too_large":             {"UNRELATED=" + strings.Repeat("x", maximumAssistantEnvironmentEntryBytes)},
		"configuration_too_large":     {assistantBuildKey + "=" + strings.Repeat("x", maximumAssistantConfigurationBytes-100), assistantOriginKey + "=" + strings.Repeat("y", 101)},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAssistantProfile(assistantSnapshotGetter{entries: entries}); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
	for _, key := range []string{assistantOriginKey, assistantModelSHA256Key, assistantBuildKey, assistantTemplateSHA256Key, assistantContextTokensKey, assistantGenerationKey} {
		values := assistantProfileEnvironment()
		delete(values, key)
		if _, err := LoadAssistantProfile(assistantProfileSnapshot(values)); !errors.Is(err, ErrInvalid) {
			t.Fatal("required identity pin absent but accepted")
		}
	}
}

func TestAssistantProfileCanonicalOriginsAndPins(t *testing.T) {
	for _, origin := range []string{"http://127.0.0.1:59641", "https://127.0.0.1", "http://[::1]:59641"} {
		values := assistantProfileEnvironment()
		values[assistantOriginKey] = origin
		if _, err := LoadAssistantProfile(assistantProfileSnapshot(values)); err != nil {
			t.Fatal("literal canonical loopback refused")
		}
	}
	cases := map[string][]string{
		assistantOriginKey:         {"http://localhost:59641", "https://example.invalid", "http://169.254.169.254", "http://10.0.0.1", "http://0.0.0.0", "http://[fd00:ec2::254]", "http://[::ffff:127.0.0.1]", "http://[::1%25zone]:59641", "http://127.0.0.1:059641", "http://127.0.0.1:80", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1/", "http://127.0.0.1?", "http://127.0.0.1#fragment", "http://user@127.0.0.1", "file:///tmp/service", strings.Repeat("x", 513)},
		assistantModelSHA256Key:    {strings.Repeat("0", 64), strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("a", 63)},
		assistantTemplateSHA256Key: {strings.Repeat("0", 64), strings.Repeat("B", 64), "not-a-digest"},
		assistantBuildKey:          {" bad", "bad ", "bad\nline", "bad\tline", "bad\x7f", strings.Repeat("x", 257)},
		assistantContextTokensKey:  {"0", "+24576", "024576", "24576.0", strconv.Itoa(inference.MaximumInputTokens + inference.MaximumOutputTokens - 1), strconv.Itoa(maximumAssistantContextTokens + 1)},
		assistantGenerationKey:     {"0", "-1", "01", "+1", strconv.FormatInt(maximumAssistantGeneration+1, 10), strings.Repeat("9", 17)},
		assistantOutputTokensKey:   {"0", "-1", "01", strconv.Itoa(inference.MaximumOutputTokens + 1)},
		assistantCallTimeoutKey:    {"0s", "1us", "1000ms", "26s", "bad", strings.Repeat("1", 33)},
	}
	for key, inputs := range cases {
		for index, input := range inputs {
			t.Run(strings.TrimPrefix(key, assistantEnvironmentPrefix)+"_"+strconv.Itoa(index), func(t *testing.T) {
				values := assistantProfileEnvironment()
				values[key] = input
				if _, err := LoadAssistantProfile(assistantProfileSnapshot(values)); !errors.Is(err, ErrInvalid) {
					t.Fatal("unsafe or noncanonical pin accepted")
				}
			})
		}
	}
	for _, limits := range [][2]string{{"1", "1ms"}, {strconv.Itoa(inference.MaximumOutputTokens), inference.MaximumCallDuration.String()}} {
		values := assistantProfileEnvironment()
		values[assistantOutputTokensKey] = limits[0]
		values[assistantCallTimeoutKey] = limits[1]
		profile, err := LoadAssistantProfile(assistantProfileSnapshot(values))
		if err != nil {
			t.Fatal("bounded configurable limits refused")
		}
		wantOutput, _ := strconv.Atoi(limits[0])
		wantTime, _ := time.ParseDuration(limits[1])
		if profile.Limits().OutputTokens != wantOutput || profile.Limits().CallTimeout != wantTime {
			t.Fatal("configured limits drift")
		}
	}
}

func TestAssistantProfileValidationAndPrivateFormatting(t *testing.T) {
	valid, err := LoadAssistantProfile(assistantProfileSnapshot(assistantProfileEnvironment()))
	if err != nil {
		t.Fatal("valid fixture refused")
	}
	mutations := []func(*AssistantProfile){
		func(p *AssistantProfile) { p.enabled = false }, func(p *AssistantProfile) { p.local.ID = "other" },
		func(p *AssistantProfile) { p.local.Address = netip.MustParseAddr("::1") }, func(p *AssistantProfile) { p.local.Origin = "https://example.invalid" },
		func(p *AssistantProfile) { p.local.ModelAlias = "bad" }, func(p *AssistantProfile) { p.local.ExpectedTemplateSHA256 = "bad" },
		func(p *AssistantProfile) { p.local.ExpectedBuild = "bad\nline" }, func(p *AssistantProfile) { p.local.ExpectedContextTokens = 0 },
		func(p *AssistantProfile) { p.local.ExpectedContextTokens = maximumAssistantContextTokens + 1 }, func(p *AssistantProfile) { p.local.Generation = 0 },
		func(p *AssistantProfile) { p.local.Generation = maximumAssistantGeneration + 1 }, func(p *AssistantProfile) { p.outputTokens = 0 },
		func(p *AssistantProfile) { p.outputTokens = inference.MaximumOutputTokens + 1 }, func(p *AssistantProfile) { p.callTimeout = 0 },
		func(p *AssistantProfile) { p.callTimeout = inference.MaximumCallDuration + time.Millisecond },
	}
	for _, mutate := range mutations {
		profile := valid
		mutate(&profile)
		if !errors.Is(profile.Validate(), ErrInvalid) {
			t.Fatal("mutated capability accepted")
		}
	}
	local, _ := valid.Local()
	for _, value := range []any{valid, local} {
		bytes, err := json.Marshal(value)
		if err != nil || string(bytes) != "{}" {
			t.Fatal("profile serialized private pins")
		}
		formatted := fmt.Sprintf("%v %+v %#v", value, value, value)
		for _, private := range []string{local.Origin, local.ExpectedBuild, local.ModelAlias, local.ExpectedTemplateSHA256} {
			if strings.Contains(formatted, private) {
				t.Fatal("profile formatting exposed pins")
			}
		}
	}
	values := assistantProfileEnvironment()
	values[assistantBuildKey] = "synthetic-untrusted-marker\ninvalid"
	_, err = LoadAssistantProfile(assistantProfileSnapshot(values))
	if err == nil || strings.Contains(err.Error(), "synthetic-untrusted-marker") {
		t.Fatal("configuration error exposed value")
	}
}
