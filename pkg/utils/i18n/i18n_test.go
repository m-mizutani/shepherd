package i18n_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
)

func TestNewTranslator(t *testing.T) {
	t.Run("english", func(t *testing.T) {
		tr := gt.R1(i18n.NewTranslator(i18n.LangEN)).NoError(t)
		gt.V(t, tr.Lang()).Equal(i18n.LangEN)
	})
	t.Run("japanese", func(t *testing.T) {
		tr := gt.R1(i18n.NewTranslator(i18n.LangJA)).NoError(t)
		gt.V(t, tr.Lang()).Equal(i18n.LangJA)
	})
	t.Run("unknown", func(t *testing.T) {
		_, err := i18n.NewTranslator(i18n.Lang("fr"))
		gt.Error(t, err)
	})
}

func TestTranslator_T_interpolation(t *testing.T) {
	tr := gt.R1(i18n.NewTranslator(i18n.LangEN)).NoError(t)
	got := tr.T(i18n.MsgTicketCreated, "url", "https://example.com/t/1", "id", 1)
	gt.S(t, got).Equal("<https://example.com/t/1|Ticket #1> created")

	jaTr := gt.R1(i18n.NewTranslator(i18n.LangJA)).NoError(t)
	gotJA := jaTr.T(i18n.MsgTicketCreated, "url", "https://example.com/t/1", "id", 1)
	gt.S(t, gotJA).Equal("<https://example.com/t/1|チケット #1> を作成しました")
}

func TestTranslator_T_missingParam(t *testing.T) {
	tr := gt.R1(i18n.NewTranslator(i18n.LangEN)).NoError(t)
	got := tr.T(i18n.MsgTicketCreated, "url", "https://example.com/t/1")
	// id is missing → placeholder kept verbatim so the omission is visible.
	gt.S(t, got).Equal("<https://example.com/t/1|Ticket #{id}> created")
}

func TestFrom_fallbackEnglish(t *testing.T) {
	ctx := context.Background()
	tr := i18n.From(ctx)
	gt.V(t, tr.Lang()).Equal(i18n.LangEN)
	got := tr.T(i18n.MsgStatusChange, "old", "open", "new", "closed")
	gt.S(t, got).Equal("Status: *open* → *closed*")
}

func TestWith_From_roundtrip(t *testing.T) {
	jaTr := gt.R1(i18n.NewTranslator(i18n.LangJA)).NoError(t)
	ctx := i18n.With(context.Background(), jaTr)
	gt.V(t, i18n.From(ctx).Lang()).Equal(i18n.LangJA)
}

func TestKeyParity(t *testing.T) {
	// Every English key must have a matching Japanese translation, and vice versa.
	enTr := gt.R1(i18n.NewTranslator(i18n.LangEN)).NoError(t)
	jaTr := gt.R1(i18n.NewTranslator(i18n.LangJA)).NoError(t)

	keys := []i18n.MsgKey{
		i18n.MsgTicketCreated,
		i18n.MsgStatusChange,
		i18n.MsgStatusChangeLabel,
	}
	for _, k := range keys {
		gt.S(t, enTr.T(k)).NotEqual(string(k))
		gt.S(t, jaTr.T(k)).NotEqual(string(k))
	}
}
