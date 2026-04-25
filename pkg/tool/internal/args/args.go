// Package args contains shared helpers for parsing the loosely-typed
// argument map gollem hands to Tool.Run.
package args

import "github.com/m-mizutani/goerr/v2"

// String extracts a string argument. When required is true and the value is
// empty, an error suitable for the LLM is returned.
func String(m map[string]any, key string, required bool) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		if required {
			return "", goerr.New("missing required argument", goerr.V("argument", key))
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", goerr.New("argument is not a string", goerr.V("argument", key))
	}
	if required && s == "" {
		return "", goerr.New("argument is empty", goerr.V("argument", key))
	}
	return s, nil
}

// Int returns 0 when the argument is absent or not a number-like value.
func Int(args map[string]any, key string) int {
	v, ok := args[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// Int64 returns the numeric argument together with a presence flag.
func Int64(args map[string]any, key string) (int64, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// StringSlice extracts a homogeneous []string from a JSON-decoded []any.
// Empty entries are skipped.
func StringSlice(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// PtrInt returns a pointer to a freshly allocated int holding n. Useful for
// MinLength/MaxLength fields on gollem.Parameter.
func PtrInt(n int) *int { return &n }
