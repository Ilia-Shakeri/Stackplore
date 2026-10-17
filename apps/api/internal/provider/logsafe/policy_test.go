package logsafe

import (
	"strings"
	"testing"
)

func TestPolicyAddsNestedEscapedAndPlainFieldsWithoutRemovingBuiltins(t *testing.T) {
	t.Parallel()
	fields := []string{"Customer_ID", "payment-ref"}
	policy, err := NewPolicy(fields)
	if err != nil {
		t.Fatal(err)
	}
	fields[0] = "ordinary"
	for _, line := range []string{
		`customer_id=synth-personal-7 password=synth-personal-8 done`,
		`payment-ref='synth-personal-7 with spaces' token=synth-personal-8 done`,
		`{"details":{"customer\u005fid":"synth-personal-7"},"ordinary":"done","password":"synth-personal-8"}`,
		`{"items":[{"payment_ref":{"value":"synth-personal-7"}}],"ordinary":"done"}`,
		`{"message":"customer-id=synth-personal-7 done"}`,
		`INFO \"customer_id\":\"synth-personal-7\"`,
	} {
		message, redacted, _ := policy.Message(line, false)
		if !redacted || strings.Contains(message, "synth-personal") || !strings.Contains(message, mask) {
			t.Fatalf("policy left sensitive field visible: %q", message)
		}
	}
	ordinary := `customer_id=synth-personal-7 ordinary=done`
	if message, redacted, _ := Message(ordinary, false); message != ordinary || redacted {
		t.Fatal("additional fields unexpectedly changed the default policy")
	}
	if message, _, _ := policy.Message(`ordinary=done customer_id=synth-personal-7`, false); !strings.Contains(message, "ordinary=done") {
		t.Fatal("constructor retained caller-owned field list")
	}
}

func TestPolicyRejectsUnsafeUnboundedAndAmbiguousNames(t *testing.T) {
	t.Parallel()
	for _, fields := range [][]string{
		{""}, {"1member"}, {" member"}, {"member.id"}, {"member.*"}, {"member name"}, {"شناسه"},
		{"customer_id", "CUSTOMER-ID"}, {strings.Repeat("a", MaximumAdditionalFieldBytes+1)},
		make([]string, MaximumAdditionalFields+1),
	} {
		if _, err := NewPolicy(fields); err != ErrInvalidPolicy {
			t.Fatal("invalid redaction policy accepted")
		}
	}
	for _, fields := range [][]string{nil, {}, {"password"}, {"a" + strings.Repeat("_", MaximumAdditionalFieldBytes-1)}} {
		if _, err := NewPolicy(fields); err != nil {
			t.Fatal("valid additive policy rejected")
		}
	}
}
