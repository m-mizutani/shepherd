package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	firestoreRepo "github.com/m-mizutani/shepherd/pkg/repository/firestore"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
)

func runTest(t *testing.T, name string, testFn func(t *testing.T, repo interfaces.Repository)) {
	t.Helper()

	t.Run("Memory/"+name, func(t *testing.T) {
		repo := memory.New()
		defer func() { _ = repo.Close() }()
		testFn(t, repo)
	})

	t.Run("Firestore/"+name, func(t *testing.T) {
		repo := newFirestoreRepo(t)
		if repo == nil {
			return
		}
		defer func() { _ = repo.Close() }()
		testFn(t, repo)
	})
}

func newFirestoreRepo(t *testing.T) interfaces.Repository {
	t.Helper()

	projectID := os.Getenv("TEST_FIRESTORE_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_FIRESTORE_PROJECT_ID not set")
		return nil
	}

	databaseID := os.Getenv("TEST_FIRESTORE_DATABASE_ID")

	ctx := context.Background()
	repo, err := firestoreRepo.New(ctx, projectID, databaseID)
	if err != nil {
		t.Fatalf("failed to create firestore repo: %v", err)
	}

	return repo
}
