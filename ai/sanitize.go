package ai

import (
	"strings"
	"unicode/utf8"
)

// Ports: packages/ai/src/utils/sanitize-unicode.ts

// SanitizeSurrogates removes unpaired Unicode surrogate code points from a
// string. Unpaired surrogates (U+D800..U+DFFF outside a valid pair) break
// JSON serialization at many API providers.
//
// In Go, a lone surrogate manifests either as a surrogate code point encoded
// in WTF-8-style bytes or as invalid UTF-8; both are removed. Properly paired
// astral characters (emoji etc.) are single runes above U+FFFF and are
// preserved untouched.
func SanitizeSurrogates(text string) string {
	if utf8.ValidString(text) && !strings.ContainsRune(text, utf8.RuneError) {
		// Fast path: valid UTF-8 cannot contain surrogate code points.
		return text
	}

	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid byte sequence: check for a WTF-8-encoded surrogate
			// (0xED 0xA0..0xBF 0x80..0xBF) and drop it whole; otherwise drop
			// the single invalid byte.
			if i+2 < len(text) && text[i] == 0xED && text[i+1] >= 0xA0 && text[i+1] <= 0xBF &&
				text[i+2] >= 0x80 && text[i+2] <= 0xBF {
				i += 3
				continue
			}
			i++
			continue
		}
		if r >= 0xD800 && r <= 0xDFFF {
			i += size
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}
