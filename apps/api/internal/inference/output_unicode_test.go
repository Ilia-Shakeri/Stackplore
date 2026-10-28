package inference

import "testing"

func TestAnswerRejectsUnpairedUnicodeEscapes(t *testing.T) {
	for _, text := range []string{`\ud800`, `\udfff`, `\ud800\u0041`, `\ud800\ud800`, `\ud83d text \ude00`} {
		body := []byte(`{"sections":[{"kind":"gaps","text":"` + text + `","citations":[]}]}`)
		if _, err := parseAnswer(body, map[string]struct{}{}); err != ErrInvalidOutput {
			t.Fatal("malformed Unicode was repaired into accepted output")
		}
	}
}

func TestAnswerPreservesValidPairedAndLiteralUnicodeEscapes(t *testing.T) {
	for encoded, want := range map[string]string{`\ud83d\ude00`: "😀", `\\ud800`: `\ud800`, `\u0041`: "A", `\uD800\uDC00`: "𐀀"} {
		body := []byte(`{"sections":[{"kind":"gaps","text":"` + encoded + `","citations":[]}]}`)
		answer, err := parseAnswer(body, map[string]struct{}{})
		if err != nil || len(answer.Sections) != 1 || answer.Sections[0].Text != want {
			t.Fatal("valid Unicode encoding changed")
		}
	}
}
