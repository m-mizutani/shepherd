package ptr_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/utils/ptr"
)

type stringID string

func TestNonZero_StringEmpty(t *testing.T) {
	gt.Nil(t, ptr.NonZero(""))
}

func TestNonZero_StringPopulated(t *testing.T) {
	got := ptr.NonZero("hello")
	gt.NotNil(t, got)
	gt.S(t, *got).Equal("hello")
}

func TestNonZero_NamedStringTypeRespectsZero(t *testing.T) {
	gt.Nil(t, ptr.NonZero(stringID("")))
	got := ptr.NonZero(stringID("U001"))
	gt.NotNil(t, got)
	gt.S(t, string(*got)).Equal("U001")
}

func TestNonZero_IntZero(t *testing.T) {
	gt.Nil(t, ptr.NonZero(0))
	got := ptr.NonZero(7)
	gt.NotNil(t, got)
	gt.V(t, *got).Equal(7)
}

func TestOf_AlwaysReturnsPointer(t *testing.T) {
	got := ptr.Of("")
	gt.NotNil(t, got)
	gt.S(t, *got).Equal("")
}
