package firestore

import "github.com/m-mizutani/fireconf"

// EmbeddingDim is the embedding vector dimensionality persisted into the
// `tickets` collection's vector index. The CLI flag --embedding-dim defaults
// to the same value; if the two ever diverge, FindNearest will reject the
// query because the index dimension does not match the query vector.
const EmbeddingDim = 768

// DesiredConfig returns the fireconf configuration shepherd's `migrate`
// subcommand applies to Firestore. It is exposed (and unit-tested) so that
// schema changes are reviewable in code rather than hidden behind cloud
// console clicks.
//
// The single declared index is a vector index on `tickets.Embedding`.
// Tickets live as a sub-collection (`workspaces/{ws}/tickets/{id}`), so the
// index is registered under the unqualified collection id `tickets` with
// `QueryScope: COLLECTION_GROUP` — Firestore vector indexes on
// sub-collections must be collection-group scoped to be queryable across
// workspaces.
func DesiredConfig() *fireconf.Config {
	return &fireconf.Config{
		Collections: []fireconf.Collection{
			{
				Name: "tickets",
				Indexes: []fireconf.Index{
					{
						QueryScope: fireconf.QueryScopeCollectionGroup,
						Fields: []fireconf.IndexField{
							{
								Path:   "Embedding",
								Vector: &fireconf.VectorConfig{Dimension: EmbeddingDim},
							},
						},
					},
				},
			},
		},
	}
}
