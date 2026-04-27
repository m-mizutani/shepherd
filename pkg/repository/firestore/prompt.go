package firestore

import (
	"context"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Layout:
//
//	workspaces/{wsID}/settings/prompts/{promptID}/versions/{N}
//	    Version        int           # mirrors the doc id, used for OrderBy
//	    Content        string
//	    UpdatedAt      time.Time
//	    UpdatedBy      string
//	    UpdatedByEmail string
//	    UpdatedBySub   string
//
// Doc id is strconv.Itoa(version). Sorting is done on the Version field, so
// the doc id does not need to be lexicographically ordered (no zero-padding).
//
// Append runs inside RunTransaction so the "read current → check that the
// caller's draft.Version equals current+1 → create the doc at draft.Version"
// sequence is fully atomic. Both failure modes surface as
// ErrPromptVersionConflict:
//   - another writer beat us (current advanced, so draft.Version is no
//     longer current+1), and
//   - the caller passed a stale or fabricated Version that does not equal
//     current+1 (e.g. Version=11 when current=3).
// The single-document Create-on-not-exists trick alone would catch only the
// first case and silently create version-number gaps for the second one.
type promptRepository struct {
	client *firestore.Client
}

type promptDoc struct {
	Version        int
	Content        string
	UpdatedAt      time.Time
	UpdatedBy      string
	UpdatedByEmail string
	UpdatedBySub   string
}

func (r *promptRepository) versionsCollection(ws types.WorkspaceID, id model.PromptID) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(string(ws)).
		Collection("settings").Doc("prompts").
		Collection(string(id)).Doc("slot").
		Collection("versions")
}

func (r *promptRepository) Append(ctx context.Context, ws types.WorkspaceID, id model.PromptID, draft *model.PromptVersion) (*model.PromptVersion, error) {
	if draft == nil {
		return nil, goerr.New("draft is nil")
	}
	if draft.Version < 1 {
		return nil, goerr.New("draft.Version must be >= 1",
			goerr.V("version", draft.Version))
	}

	col := r.versionsCollection(ws, id)
	doc := promptDoc{
		Version:        draft.Version,
		Content:        draft.Content,
		UpdatedAt:      draft.UpdatedAt,
		UpdatedBy:      draft.UpdatedBy,
		UpdatedByEmail: draft.UpdatedByEmail,
		UpdatedBySub:   draft.UpdatedBySub,
	}

	err := r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Read the highest existing version inside the transaction. Firestore
		// guarantees this read is serialized against any concurrent writer:
		// if another transaction commits a higher version before us, this one
		// is retried (or aborted) automatically.
		iter := tx.Documents(col.OrderBy("Version", firestore.Desc).Limit(1))
		defer iter.Stop()

		current := 0
		snap, err := iter.Next()
		if err != nil && err != iterator.Done {
			return goerr.Wrap(err, "load current prompt version in tx",
				goerr.V("workspace_id", string(ws)),
				goerr.V("prompt_id", string(id)))
		}
		if err == nil {
			var pd promptDoc
			if decodeErr := snap.DataTo(&pd); decodeErr != nil {
				return goerr.Wrap(decodeErr, "decode current prompt version in tx")
			}
			current = pd.Version
		}

		if draft.Version != current+1 {
			return goerr.Wrap(interfaces.ErrPromptVersionConflict,
				"version is not exactly current+1",
				goerr.V("workspace_id", string(ws)),
				goerr.V("prompt_id", string(id)),
				goerr.V("version", draft.Version),
				goerr.V("current_version", current))
		}

		// Defense in depth: even though draft.Version == current+1 implies
		// the doc must not exist, use Create (not Set) so any surprising
		// state still fails closed instead of silently overwriting.
		return tx.Create(col.Doc(strconv.Itoa(draft.Version)), doc)
	})
	if err != nil {
		// Race losers can also surface here as ALREADY_EXISTS when two
		// transactions pick the same Version; fold that into a conflict.
		if status.Code(err) == codes.AlreadyExists {
			return nil, goerr.Wrap(interfaces.ErrPromptVersionConflict,
				"version already exists",
				goerr.V("workspace_id", string(ws)),
				goerr.V("prompt_id", string(id)),
				goerr.V("version", draft.Version))
		}
		return nil, err
	}

	out := *draft
	out.WorkspaceID = ws
	out.PromptID = id
	return &out, nil
}

func (r *promptRepository) GetCurrent(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (*model.PromptVersion, error) {
	iter := r.versionsCollection(ws, id).
		OrderBy("Version", firestore.Desc).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, goerr.Wrap(err, "failed to load current prompt",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)))
	}
	return decodePromptDoc(ws, id, doc)
}

func (r *promptRepository) GetVersion(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int) (*model.PromptVersion, error) {
	if version < 1 {
		return nil, goerr.Wrap(interfaces.ErrPromptVersionNotFound,
			"version must be >= 1",
			goerr.V("version", version))
	}
	doc, err := r.versionsCollection(ws, id).Doc(strconv.Itoa(version)).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, goerr.Wrap(interfaces.ErrPromptVersionNotFound,
				"version document missing",
				goerr.V("workspace_id", string(ws)),
				goerr.V("prompt_id", string(id)),
				goerr.V("version", version))
		}
		return nil, goerr.Wrap(err, "failed to get prompt version",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)),
			goerr.V("version", version))
	}
	return decodePromptDoc(ws, id, doc)
}

func (r *promptRepository) List(ctx context.Context, ws types.WorkspaceID, id model.PromptID) ([]*model.PromptVersion, error) {
	iter := r.versionsCollection(ws, id).
		OrderBy("Version", firestore.Asc).
		Documents(ctx)
	defer iter.Stop()

	var out []*model.PromptVersion
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate prompt versions",
				goerr.V("workspace_id", string(ws)),
				goerr.V("prompt_id", string(id)))
		}
		v, err := decodePromptDoc(ws, id, doc)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodePromptDoc(ws types.WorkspaceID, id model.PromptID, doc *firestore.DocumentSnapshot) (*model.PromptVersion, error) {
	var pd promptDoc
	if err := doc.DataTo(&pd); err != nil {
		return nil, goerr.Wrap(err, "failed to decode prompt version",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)),
			goerr.V("doc_id", doc.Ref.ID))
	}
	return &model.PromptVersion{
		WorkspaceID:    ws,
		PromptID:       id,
		Version:        pd.Version,
		Content:        pd.Content,
		UpdatedAt:      pd.UpdatedAt,
		UpdatedBy:      pd.UpdatedBy,
		UpdatedByEmail: pd.UpdatedByEmail,
		UpdatedBySub:   pd.UpdatedBySub,
	}, nil
}
