package config

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	firestoreRepo "github.com/m-mizutani/shepherd/pkg/repository/firestore"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

type Repository struct {
	backend    string
	projectID  string
	databaseID string
}

func (r *Repository) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "repository-backend",
			Usage:       "Repository backend type (firestore or memory)",
			Value:       "memory",
			Sources:     cli.EnvVars("SHEPHERD_REPOSITORY_BACKEND"),
			Destination: &r.backend,
		},
		&cli.StringFlag{
			Name:        "firestore-project-id",
			Usage:       "Firestore Project ID",
			Sources:     cli.EnvVars("SHEPHERD_FIRESTORE_PROJECT_ID"),
			Destination: &r.projectID,
		},
		&cli.StringFlag{
			Name:        "firestore-database-id",
			Usage:       "Firestore Database ID",
			Sources:     cli.EnvVars("SHEPHERD_FIRESTORE_DATABASE_ID"),
			Destination: &r.databaseID,
		},
	}
}

func (r *Repository) Backend() string    { return r.backend }
func (r *Repository) ProjectID() string  { return r.projectID }
func (r *Repository) DatabaseID() string { return r.databaseID }

func (r *Repository) Configure(ctx context.Context) (interfaces.Repository, error) {
	switch r.backend {
	case "firestore":
		if r.projectID == "" {
			return nil, goerr.New("firestore-project-id is required when using firestore backend")
		}
		repo, err := firestoreRepo.New(ctx, r.projectID, r.databaseID)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to initialize firestore repository")
		}
		logging.Default().Info("Using Firestore repository",
			"project_id", r.projectID,
			"database_id", r.databaseID,
		)
		return repo, nil

	case "memory":
		logging.Default().Info("Using in-memory repository")
		return memory.New(), nil

	default:
		return nil, goerr.New("unknown repository backend", goerr.V("backend", r.backend))
	}
}
