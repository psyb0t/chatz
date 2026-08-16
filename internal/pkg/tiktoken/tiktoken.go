// Package tiktoken wraps the embedded tiktoken-go BPE codec so text can be
// split into model tokens with no runtime network fetch — the o200k_base
// vocabulary is compiled into the binary. The demo stream uses it to emit text
// at real token boundaries: a canned response then streams exactly like a live
// model turn (one SSE delta per token) instead of arbitrary fixed-width chunks.
package tiktoken

import (
	"sync"

	"github.com/psyb0t/ctxerrors"
	"github.com/tiktoken-go/tokenizer/codec" //nolint:depguard // embedded o200k_base BPE — no network fetch
)

// Lazy codec state. Constructing the codec walks the embedded BPE maps, so it
// is built once and reused across turns rather than per call.
var (
	once  sync.Once    //nolint:gochecknoglobals // shared lazy codec init
	value *codec.Codec //nolint:gochecknoglobals // shared lazy codec init
)

// codecInstance returns the shared codec, building it on first use.
func codecInstance() *codec.Codec {
	once.Do(func() {
		value = codec.NewO200kBase()
	})

	return value
}

// Tokenize splits text into its o200k_base model tokens as decoded strings,
// ready to stream one delta per token. Concatenating the returned tokens
// reproduces the input byte-for-byte.
func Tokenize(text string) ([]string, error) {
	_, tokens, err := codecInstance().Encode(text)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "tokenize text")
	}

	return tokens, nil
}

// Count returns the number of o200k_base tokens in text without allocating the
// decoded token strings used by Tokenize.
func Count(text string) (int, error) {
	ids, _, err := codecInstance().Encode(text)
	if err != nil {
		return 0, ctxerrors.Wrap(err, "count tokens")
	}

	return len(ids), nil
}
