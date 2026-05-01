package firestore_test

import (
	"testing"

	"github.com/m-mizutani/fireconf"
	"github.com/m-mizutani/gt"
	firestoreRepo "github.com/m-mizutani/shepherd/pkg/repository/firestore"
)

func TestDesiredConfig(t *testing.T) {
	cfg := firestoreRepo.DesiredConfig()
	gt.NotNil(t, cfg)
	gt.A(t, cfg.Collections).Length(1)

	col := cfg.Collections[0]
	gt.S(t, col.Name).Equal("tickets")
	gt.A(t, col.Indexes).Length(1)

	idx := col.Indexes[0]
	gt.Equal(t, idx.QueryScope, fireconf.QueryScopeCollectionGroup)
	gt.A(t, idx.Fields).Length(1)

	field := idx.Fields[0]
	gt.S(t, field.Path).Equal("Embedding")
	gt.NotNil(t, field.Vector)
	gt.Equal(t, field.Vector.Dimension, firestoreRepo.EmbeddingDim)
	gt.Equal(t, firestoreRepo.EmbeddingDim, 768)
}
