package ai

// Ports: packages/ai/test/unicode-surrogate.test.ts
//
// unicode-surrogate.test.ts is a live-provider integration suite (it drives
// real completions across every configured provider to prove a tool result
// containing emoji/surrogates round-trips without a "no low surrogate in
// string" API error). None of that is portable as a hermetic unit test. What
// *is* portable, and what this issue calls "verify, don't rewrite": the exact
// fixture strings those live tests send as tool-result content, asserted
// directly against SanitizeSurrogates (ported in Phase 1, ai/sanitize.go).

import "testing"

// From unicode-surrogate.test.ts's testEmojiInToolResults: a tool result
// containing emoji and other non-BMP/astral characters must pass through
// untouched (properly paired astral characters are not surrogates).
func TestSanitizeSurrogatesPreservesEmojiAndAstralText(t *testing.T) {
	text := `Test with emoji 🙈 and other characters:
- Monkey emoji: 🙈
- Thumbs up: 👍
- Heart: ❤️
- Thinking face: 🤔
- Rocket: 🚀
- Mixed text: Mario Zechner wann? Wo? Bin grad äußersr eventuninformiert 🙈
- Japanese: こんにちは
- Chinese: 你好
- Mathematical symbols: ∑∫∂√
- Special quotes: "curly" 'quotes'`

	if got := SanitizeSurrogates(text); got != text {
		t.Errorf("SanitizeSurrogates altered valid emoji/astral text:\n got:  %q\n want: %q", got, text)
	}
}

// From unicode-surrogate.test.ts's testRealWorldLinkedInData: the exact
// real-world payload (JSON-shaped text with embedded emoji) that originally
// triggered the upstream surrogate-pair bug report.
func TestSanitizeSurrogatesPreservesRealWorldLinkedInData(t *testing.T) {
	text := `Post: Hab einen "Generative KI für Nicht-Techniker" Workshop gebaut.
Unanswered Comments: 2

=> {
  "comments": [
    {
      "author": "Matthias Neumayer's  graphic link",
      "text": "Leider nehmen das viel zu wenige Leute ernst"
    },
    {
      "author": "Matthias Neumayer's  graphic link",
      "text": "Mario Zechner wann? Wo? Bin grad äußersr eventuninformiert 🙈"
    }
  ]
}`

	if got := SanitizeSurrogates(text); got != text {
		t.Errorf("SanitizeSurrogates altered real-world LinkedIn payload:\n got:  %q\n want: %q", got, text)
	}
}

// From unicode-surrogate.test.ts's testUnpairedHighSurrogate: simulates text
// processing that corrupted an emoji into a lone high surrogate (U+D83D,
// JS String.fromCharCode(0xd83d)) with no matching low surrogate. Go strings
// are always UTF-8 and cannot hold a raw surrogate code point directly, so
// the unpaired half is represented the way it actually arrives in practice:
// WTF-8-encoded bytes (0xED 0xA0 0xBD), the same encoding a naive UTF-16-only
// serializer or a byte-level truncation can produce. The API-breaking half
// must be dropped; surrounding text must survive.
func TestSanitizeSurrogatesRemovesUnpairedHighSurrogate(t *testing.T) {
	unpairedHighSurrogate := string([]byte{0xED, 0xA0, 0xBD}) // WTF-8 for U+D83D
	text := "Text with unpaired surrogate: " + unpairedHighSurrogate + " <- should be sanitized"
	want := "Text with unpaired surrogate:  <- should be sanitized"

	if got := SanitizeSurrogates(text); got != want {
		t.Errorf("SanitizeSurrogates(%q) = %q, want %q", text, got, want)
	}
}

// Invalid UTF-8 that is not a WTF-8-encoded surrogate (an ordinary truncated
// multi-byte sequence) must still be dropped without panicking or producing
// invalid output, since SanitizeSurrogates is the last line of defense before
// JSON-serializing tool results to a provider.
func TestSanitizeSurrogatesDropsOtherInvalidUTF8(t *testing.T) {
	text := "before \xff\xfe after"
	want := "before  after"

	if got := SanitizeSurrogates(text); got != want {
		t.Errorf("SanitizeSurrogates(%q) = %q, want %q", text, got, want)
	}
}
