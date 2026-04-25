package clamp_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
)

func TestLimit(t *testing.T) {
	gt.Equal(t, clamp.Limit(0, 20, 50), 20)
	gt.Equal(t, clamp.Limit(-5, 20, 50), 20)
	gt.Equal(t, clamp.Limit(10, 20, 50), 10)
	gt.Equal(t, clamp.Limit(100, 20, 50), 50)
	gt.Equal(t, clamp.Limit(50, 20, 50), 50)
}
