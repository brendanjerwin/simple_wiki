package server

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonBytesToMap deserializes a JSON object into a generic map[string]any.
// Used to convert protojson output into the frontmatter representation.
//
// json.Number is normalized to float64 so the resulting map uses the same
// value types as the rest of the frontmatter pipeline (YAML/TOML decoders all
// emit float64 for numeric scalars).
func jsonBytesToMap(b []byte) (map[string]any, error) {
	out := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if normalized, ok := normalizeJSONNumbers(out).(map[string]any); ok {
		out = normalized
	}
	return out, nil
}

// mapToJSONBytes serializes a generic map back to JSON bytes for protojson
// consumption.
func mapToJSONBytes(m map[string]any) ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	return b, nil
}

// normalizeJSONNumbers walks the decoded JSON tree and converts json.Number
// scalars into float64.
func normalizeJSONNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			x[k] = normalizeJSONNumbers(vv)
		}
		return x
	case []any:
		for i, vv := range x {
			x[i] = normalizeJSONNumbers(vv)
		}
		return x
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return float64(i)
		}
		f, _ := x.Float64()
		return f
	default:
		return v
	}
}
