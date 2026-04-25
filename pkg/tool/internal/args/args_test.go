package args_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/args"
)

func TestString(t *testing.T) {
	t.Run("required missing", func(t *testing.T) {
		_, err := args.String(map[string]any{}, "k", true)
		gt.Error(t, err)
	})
	t.Run("required empty", func(t *testing.T) {
		_, err := args.String(map[string]any{"k": ""}, "k", true)
		gt.Error(t, err)
	})
	t.Run("optional missing", func(t *testing.T) {
		v, err := args.String(map[string]any{}, "k", false)
		gt.NoError(t, err)
		gt.Equal(t, v, "")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := args.String(map[string]any{"k": 1}, "k", false)
		gt.Error(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		v, err := args.String(map[string]any{"k": "hi"}, "k", true)
		gt.NoError(t, err)
		gt.Equal(t, v, "hi")
	})
}

func TestInt(t *testing.T) {
	gt.Equal(t, args.Int(map[string]any{}, "k"), 0)
	gt.Equal(t, args.Int(map[string]any{"k": 5}, "k"), 5)
	gt.Equal(t, args.Int(map[string]any{"k": int64(7)}, "k"), 7)
	gt.Equal(t, args.Int(map[string]any{"k": float64(9)}, "k"), 9)
	gt.Equal(t, args.Int(map[string]any{"k": "x"}, "k"), 0)
}

func TestInt64(t *testing.T) {
	_, ok := args.Int64(map[string]any{}, "k")
	gt.False(t, ok)
	v, ok := args.Int64(map[string]any{"k": float64(42)}, "k")
	gt.True(t, ok)
	gt.Equal(t, v, int64(42))
}

func TestStringSlice(t *testing.T) {
	gt.Nil(t, args.StringSlice(map[string]any{}, "k"))
	got := args.StringSlice(map[string]any{"k": []any{"a", "", "b", 1}}, "k")
	gt.Equal(t, len(got), 2)
	gt.Equal(t, got[0], "a")
	gt.Equal(t, got[1], "b")
}
