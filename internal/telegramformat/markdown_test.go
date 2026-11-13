package telegramformat

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEscapeAndLimit(t *testing.T) {
	t.Parallel()
	if got := Escape(`a_[b]!`); got != `a\_\[b\]\!` {
		t.Fatalf("Escape() = %q", got)
	}
	got := Limit(strings.Repeat("a", MessageLimit+10) + `\`)
	if utf8.RuneCountInString(got) > MessageLimit || strings.HasSuffix(strings.TrimSuffix(got, ellipsis), `\`) {
		t.Fatalf("Limit() produced unsafe output of %d runes", utf8.RuneCountInString(got))
	}
}

func TestPlainTextPreservesEscapedContentAndMentionLabel(t *testing.T) {
	t.Parallel()
	input := `🎁 *Entry:* Prize\_A \*bonus\* [Ada\_B](tg://user?id=42)\!`
	if got, want := PlainText(input), "🎁 Entry: Prize_A *bonus* Ada_B!"; got != want {
		t.Fatalf("PlainText() = %q, want %q", got, want)
	}
}

func TestLimitPreservesBoldAndMentionBoundaries(t *testing.T) {
	t.Parallel()
	bold := Limit("✨ *" + strings.Repeat("a", MessageLimit) + "*")
	if utf8.RuneCountInString(bold) > MessageLimit || !strings.HasSuffix(bold, "*"+ellipsis) {
		t.Fatalf("bold message was truncated inside markup: %q", bold[len(bold)-24:])
	}
	mention := Limit("Welcome [" + strings.Repeat("a", MessageLimit) + "](tg://user?id=42)")
	if utf8.RuneCountInString(mention) > MessageLimit || strings.Contains(mention, "[") || !strings.HasSuffix(mention, ellipsis) {
		t.Fatalf("mention message was truncated inside a link: %q", mention)
	}
}

func TestLimitDoesNotCloseWhitespaceOnlyBoldIntoEmptyMarkup(t *testing.T) {
	t.Parallel()
	for _, whitespace := range []string{" ", "\t", "\n", "\u3000"} {
		t.Run(whitespace, func(t *testing.T) {
			input := "You reached *" + strings.Repeat(whitespace, MessageLimit) + "member*"
			got := Limit(input)
			if got != "You reached"+ellipsis || strings.Contains(got, "**") || utf8.RuneCountInString(got) > MessageLimit {
				t.Fatalf("truncation created an empty bold entity: %q", got)
			}
		})
	}
}
