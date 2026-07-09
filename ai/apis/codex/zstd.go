package codex

// Ports: the request-compression half of
// packages/ai/src/api/openai-codex-responses.ts (compressRequestBodyZstd) --
// epic 7 issue 04's scope. The Codex backend accepts zstd-compressed request
// bodies on the SSE responses endpoint (the same endpoint the official Codex
// client compresses against); this applies to every plain-SSE send, whether
// reached directly (`transport: "sse"`) or via a WebSocket fallback.
//
// Unlike upstream -- which loads node:zlib lazily and falls back to sending
// the uncompressed JSON body when it's unavailable (a browser/Vite-bundler
// concern) -- this Go port always has klauspost/compress/zstd available, so
// compression never has an "unavailable" case to fall back from.

import "github.com/klauspost/compress/zstd"

// codexZstdCompressionLevel mirrors upstream's REQUEST_COMPRESSION_ZSTD_LEVEL
// (a raw zstd level, not klauspost's abstract EncoderLevel enum).
const codexZstdCompressionLevel = 3

// compressRequestBodyZstd zstd-compresses bodyJSON at the level the Codex
// backend compresses its own client traffic against. Ports
// compressRequestBodyZstd's happy path (Go has no "compression unavailable"
// case to mirror, see package doc).
func compressRequestBodyZstd(bodyJSON []byte) ([]byte, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(codexZstdCompressionLevel)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = enc.Close() }()
	return enc.EncodeAll(bodyJSON, make([]byte, 0, len(bodyJSON))), nil
}
