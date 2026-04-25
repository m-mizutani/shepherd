package slack

import "github.com/m-mizutani/goerr/v2"

// stringArg extracts a string argument. When required is true and the value is
// empty, an error suitable for the LLM is returned.
func stringArg(args map[string]any, key string, required bool) (string, error) {
	v, ok := args[key]
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

// intArg returns 0 when the argument is absent or not a number-like value.
func intArg(args map[string]any, key string) int {
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

func ptrInt(n int) *int { return &n }
