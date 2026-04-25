// Package i18n provides backend message translation. The active Translator is
// propagated via context.Context (similar to pkg/utils/logging). Callers obtain
// it with i18n.From(ctx) and emit user-facing strings with .T(key, params...).
package i18n

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
)

type Lang string

const (
	LangEN Lang = "en"
	LangJA Lang = "ja"
)

type Translator interface {
	T(key MsgKey, params ...any) string
	Lang() Lang
}

type translator struct {
	lang     Lang
	messages map[MsgKey]string
}

var messagesMap = map[Lang]map[MsgKey]string{
	LangEN: en,
	LangJA: ja,
}

// NewTranslator builds a Translator for the given language. An unsupported
// value returns an error.
func NewTranslator(lang Lang) (Translator, error) {
	msgs, ok := messagesMap[lang]
	if !ok {
		return nil, goerr.New("unsupported language",
			goerr.V("lang", string(lang)),
		)
	}
	return &translator{lang: lang, messages: msgs}, nil
}

func (t *translator) Lang() Lang { return t.lang }

func (t *translator) T(key MsgKey, params ...any) string {
	tmpl, ok := t.messages[key]
	if !ok {
		// Fallback to English; if still missing, surface the key itself so
		// the omission is visible without panicking.
		if t.lang != LangEN {
			if alt, ok := en[key]; ok {
				tmpl = alt
			}
		}
		if tmpl == "" {
			tmpl = string(key)
		}
	}
	if len(params) == 0 {
		return tmpl
	}
	return interpolate(tmpl, params)
}

func interpolate(tmpl string, params []any) string {
	if len(params)%2 != 0 {
		return tmpl
	}
	pairs := make(map[string]string, len(params)/2)
	for i := 0; i < len(params); i += 2 {
		k, ok := params[i].(string)
		if !ok {
			continue
		}
		pairs[k] = fmt.Sprint(params[i+1])
	}

	var b strings.Builder
	b.Grow(len(tmpl))
	for i := 0; i < len(tmpl); {
		if tmpl[i] == '{' {
			end := strings.IndexByte(tmpl[i:], '}')
			if end > 0 {
				name := tmpl[i+1 : i+end]
				if val, ok := pairs[name]; ok {
					b.WriteString(val)
				} else {
					b.WriteString(tmpl[i : i+end+1])
				}
				i += end + 1
				continue
			}
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String()
}

type ctxKey struct{}

// With returns a new context carrying tr as the active Translator.
func With(ctx context.Context, tr Translator) context.Context {
	return context.WithValue(ctx, ctxKey{}, tr)
}

// From returns the Translator stored in ctx. If none is present, an English
// Translator is returned so callers can always chain i18n.From(ctx).T(...).
func From(ctx context.Context) Translator {
	if tr, ok := ctx.Value(ctxKey{}).(Translator); ok && tr != nil {
		return tr
	}
	return defaultEN
}

var defaultEN Translator = &translator{lang: LangEN, messages: en}
