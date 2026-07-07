package ai

import (
	"strconv"
	"unicode/utf16"
)

// Ports: packages/ai/src/utils/hash.ts (cyrb53 variant)

// ShortHash is a fast deterministic non-cryptographic hash used to shorten
// long strings (cache keys, tool-call ids). It reproduces the TS
// implementation exactly, including its UTF-16 code-unit iteration and JS
// 32-bit integer semantics, so hashes are stable across both ports.
func ShortHash(str string) string {
	h1 := int32(-559038737) // 0xdeadbeef as int32
	h2 := int32(0x41c6ce57)
	for _, ch := range utf16.Encode([]rune(str)) {
		h1 = imul(h1^int32(ch), 2654435761)
		h2 = imul(h2^int32(ch), 1597334677)
	}
	h1 = imul(h1^shr(h1, 16), 2246822507) ^ imul(h2^shr(h2, 13), 3266489909)
	h2 = imul(h2^shr(h2, 16), 2246822507) ^ imul(h1^shr(h1, 13), 3266489909)
	return strconv.FormatUint(uint64(uint32(h2)), 36) + strconv.FormatUint(uint64(uint32(h1)), 36)
}

// imul reproduces JS Math.imul: 32-bit integer multiply with wrapping.
// The uint32 multiplier constants wrap into int32 range like JS coerces them.
func imul(a int32, b uint32) int32 {
	return int32(uint32(a) * b)
}

// shr reproduces the JS unsigned right shift (>>>) reinterpreted as int32 for
// subsequent xor operations.
func shr(v int32, n uint) int32 {
	return int32(uint32(v) >> n)
}
