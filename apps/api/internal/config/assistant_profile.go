package config

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

const (
	assistantEnvironmentPrefix            = prefix + "ASSISTANT_"
	assistantEnabledKey                   = assistantEnvironmentPrefix + "ENABLED"
	assistantOriginKey                    = assistantEnvironmentPrefix + "ORIGIN"
	assistantModelSHA256Key               = assistantEnvironmentPrefix + "MODEL_SHA256"
	assistantBuildKey                     = assistantEnvironmentPrefix + "BUILD"
	assistantTemplateSHA256Key            = assistantEnvironmentPrefix + "TEMPLATE_SHA256"
	assistantContextTokensKey             = assistantEnvironmentPrefix + "CONTEXT_TOKENS"
	assistantGenerationKey                = assistantEnvironmentPrefix + "GENERATION"
	assistantOutputTokensKey              = assistantEnvironmentPrefix + "OUTPUT_TOKENS"
	assistantCallTimeoutKey               = assistantEnvironmentPrefix + "CALL_TIMEOUT"
	maximumAssistantEnvironmentEntries    = 4096
	maximumAssistantEnvironmentEntryBytes = 16 * 1024
	maximumAssistantConfigurationBytes    = 16 * 1024
	maximumAssistantContextTokens         = 65_536
	maximumAssistantGeneration            = 9_007_199_254_740_991
)

var assistantProfileKeys = [...]string{
	assistantEnabledKey, assistantOriginKey, assistantModelSHA256Key, assistantBuildKey,
	assistantTemplateSHA256Key, assistantContextTokensKey, assistantGenerationKey,
	assistantOutputTokensKey, assistantCallTimeoutKey,
}

// AssistantProfile is a parsed operator capability, separate from Config until
// runtime composition and its acceptance gates explicitly register these keys.
// Its zero value is disabled. No parser or accessor performs network or file I/O.
type AssistantProfile struct {
	enabled      bool
	local        AssistantLocalProfile
	outputTokens int
	callTimeout  time.Duration
}

// AssistantLocalProfile is a trusted composition copy, never an HTTP input.
// The selected address is a literal loopback address; DNS and private LAN
// destinations are outside this initial configuration profile.
type AssistantLocalProfile struct {
	ID                     string
	Origin                 string
	Address                netip.Addr
	ModelAlias             string
	ExpectedBuild          string
	ExpectedTemplateSHA256 string
	ExpectedContextTokens  int
	Generation             int64
}

// AssistantLimits reports enforceable existing gateway and service bounds.
// Only output tokens and call timeout are configurable, and only downwards.
type AssistantLimits struct {
	InputBytes, InputTokens, OutputBytes, OutputTokens int
	InferenceCalls, ToolCalls                          int
	CallTimeout, RunTimeout                            time.Duration
}

func (AssistantProfile) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected assistant profile]")
}
func (AssistantProfile) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (AssistantLocalProfile) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected assistant local profile]")
}
func (AssistantLocalProfile) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (profile AssistantProfile) Enabled() bool { return profile.enabled }

// Local returns no target when disabled. The caller must keep runtime creation
// inside the true branch; validation is not a request or a readiness probe.
func (profile AssistantProfile) Local() (AssistantLocalProfile, bool) {
	if !profile.enabled {
		return AssistantLocalProfile{}, false
	}
	return profile.local, true
}

func (profile AssistantProfile) Limits() AssistantLimits {
	if !profile.enabled {
		return AssistantLimits{}
	}
	return AssistantLimits{
		InputBytes: inference.MaximumInputBytes, InputTokens: inference.MaximumInputTokens,
		OutputBytes: inference.MaximumOutputBytes, OutputTokens: profile.outputTokens,
		InferenceCalls: assistant.MaximumInferenceCalls, ToolCalls: assistant.MaximumToolCalls,
		CallTimeout: profile.callTimeout, RunTimeout: assistant.MaximumRunDuration,
	}
}

// LoadAssistantProfile reads one bounded environment snapshot. Getter.Environ
// must enumerate all assistant variables; LookupEnv is deliberately not used
// to combine inconsistent snapshots or fetch values absent from that inventory.
// This helper does not register settings in the existing global Config loader.
func LoadAssistantProfile(getter Getter) (AssistantProfile, error) {
	if getter == nil {
		return AssistantProfile{}, invalid("assistant_profile", "environment is unavailable")
	}
	entries := getter.Environ()
	if len(entries) > maximumAssistantEnvironmentEntries {
		return AssistantProfile{}, invalid("assistant_profile", "environment exceeds its bound")
	}
	allowed := make(map[string]bool, len(assistantProfileKeys))
	for _, name := range assistantProfileKeys {
		allowed[name] = true
	}
	values := make(map[string]string, len(assistantProfileKeys))
	total := 0
	for _, entry := range entries {
		if len(entry) > maximumAssistantEnvironmentEntryBytes {
			return AssistantProfile{}, invalid("assistant_profile", "environment entry exceeds its bound")
		}
		name, value, separated := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), assistantEnvironmentPrefix) {
			continue
		}
		total += len(entry)
		if total > maximumAssistantConfigurationBytes || len(values) >= len(assistantProfileKeys) {
			return AssistantProfile{}, invalid("assistant_profile", "configuration exceeds its bound")
		}
		if name != strings.ToUpper(name) || !allowed[name] {
			return AssistantProfile{}, invalid("assistant_profile", "contains an unknown variable")
		}
		if _, duplicate := values[name]; duplicate {
			return AssistantProfile{}, invalid(name, "is duplicated")
		}
		if !separated || value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return AssistantProfile{}, invalid(name, "is blank or not canonical")
		}
		values[name] = value
	}
	enabled, present := values[assistantEnabledKey]
	if present && enabled != "true" && enabled != "false" {
		return AssistantProfile{}, invalid(assistantEnabledKey, "must be true or false")
	}
	if enabled != "true" {
		if len(values) > 0 && !(len(values) == 1 && present) {
			return AssistantProfile{}, invalid(assistantEnabledKey, "requires profile inputs to be absent while disabled")
		}
		return AssistantProfile{}, nil
	}
	for _, name := range []string{assistantOriginKey, assistantModelSHA256Key, assistantBuildKey, assistantTemplateSHA256Key, assistantContextTokensKey, assistantGenerationKey} {
		if _, found := values[name]; !found {
			return AssistantProfile{}, invalid(name, "is required for the local profile")
		}
	}
	address, err := assistantLoopbackOrigin(values[assistantOriginKey])
	if err != nil {
		return AssistantProfile{}, err
	}
	if !assistantDigest(values[assistantModelSHA256Key]) {
		return AssistantProfile{}, invalid(assistantModelSHA256Key, "must be an approved lowercase digest")
	}
	if !assistantDigest(values[assistantTemplateSHA256Key]) {
		return AssistantProfile{}, invalid(assistantTemplateSHA256Key, "must be an approved lowercase digest")
	}
	if !assistantBuild(values[assistantBuildKey]) {
		return AssistantProfile{}, invalid(assistantBuildKey, "must be a bounded exact build pin")
	}
	contextTokens, err := assistantInteger(values[assistantContextTokensKey], assistantContextTokensKey, int64(inference.MaximumInputTokens+inference.MaximumOutputTokens), maximumAssistantContextTokens)
	if err != nil {
		return AssistantProfile{}, err
	}
	generation, err := assistantInteger(values[assistantGenerationKey], assistantGenerationKey, 1, maximumAssistantGeneration)
	if err != nil {
		return AssistantProfile{}, err
	}
	outputTokens := int64(inference.MaximumOutputTokens)
	if value, found := values[assistantOutputTokensKey]; found {
		outputTokens, err = assistantInteger(value, assistantOutputTokensKey, 1, inference.MaximumOutputTokens)
		if err != nil {
			return AssistantProfile{}, err
		}
	}
	callTimeout := inference.MaximumCallDuration
	if value, found := values[assistantCallTimeoutKey]; found {
		callTimeout, err = time.ParseDuration(value)
		if len(value) > 32 || err != nil || callTimeout.String() != value || callTimeout < time.Millisecond || callTimeout > inference.MaximumCallDuration {
			return AssistantProfile{}, invalid(assistantCallTimeoutKey, "is outside its canonical safe range")
		}
	}
	profile := AssistantProfile{
		enabled: true, outputTokens: int(outputTokens), callTimeout: callTimeout,
		local: AssistantLocalProfile{ID: "local", Origin: values[assistantOriginKey], Address: address,
			ModelAlias: "sha256-" + values[assistantModelSHA256Key], ExpectedBuild: values[assistantBuildKey],
			ExpectedTemplateSHA256: values[assistantTemplateSHA256Key], ExpectedContextTokens: int(contextTokens), Generation: generation},
	}
	if err := profile.Validate(); err != nil {
		return AssistantProfile{}, err
	}
	return profile, nil
}

// Validate also guards typed composition against latent disabled pins and
// changed copies. No address lookup, readiness request or program is started.
func (profile AssistantProfile) Validate() error {
	if !profile.enabled {
		if profile.local != (AssistantLocalProfile{}) || profile.outputTokens != 0 || profile.callTimeout != 0 {
			return invalid(assistantEnabledKey, "contains inactive profile inputs")
		}
		return nil
	}
	local := profile.local
	address, err := assistantLoopbackOrigin(local.Origin)
	if err != nil {
		return err
	}
	if local.ID != "local" || local.Address != address || !strings.HasPrefix(local.ModelAlias, "sha256-") || !assistantDigest(strings.TrimPrefix(local.ModelAlias, "sha256-")) || !assistantDigest(local.ExpectedTemplateSHA256) || !assistantBuild(local.ExpectedBuild) || local.ExpectedContextTokens < inference.MaximumInputTokens+inference.MaximumOutputTokens || local.ExpectedContextTokens > maximumAssistantContextTokens || local.Generation < 1 || local.Generation > maximumAssistantGeneration {
		return invalid("assistant_profile", "has invalid local identity pins")
	}
	if profile.outputTokens < 1 || profile.outputTokens > inference.MaximumOutputTokens || profile.callTimeout < time.Millisecond || profile.callTimeout > inference.MaximumCallDuration {
		return invalid("assistant_profile", "has invalid call limits")
	}
	return nil
}

func assistantLoopbackOrigin(value string) (netip.Addr, error) {
	if len(value) > 512 {
		return netip.Addr{}, invalid(assistantOriginKey, "must be an exact literal loopback origin")
	}
	origin, err := parsePublicOrigin(value)
	if err != nil {
		return netip.Addr{}, invalid(assistantOriginKey, "must be an exact literal loopback origin")
	}
	address, err := netip.ParseAddr(origin.Hostname())
	if err != nil || address.Zone() != "" || address.Is4In6() || !address.IsLoopback() || address.String() != origin.Hostname() {
		return netip.Addr{}, invalid(assistantOriginKey, "must be an exact literal loopback origin")
	}
	return address, nil
}

func assistantInteger(value, field string, minimum, maximum int64) (int64, error) {
	if len(value) < 1 || len(value) > 16 {
		return 0, invalid(field, "is outside its canonical safe range")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || strconv.FormatInt(parsed, 10) != value || parsed < minimum || parsed > maximum {
		return 0, invalid(field, "is outside its canonical safe range")
	}
	return parsed, nil
}

func assistantDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value || value == strings.Repeat("0", 64) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func assistantBuild(value string) bool {
	if len(value) < 1 || len(value) > 256 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}
