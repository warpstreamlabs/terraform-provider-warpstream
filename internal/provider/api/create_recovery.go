package api

import (
	"errors"
	"fmt"
	"log"
	"time"
)

// orphanCreatedAtSkew allows for clock skew between this machine and the server when deciding whether a
// listed object was created by our own failed attempt.
const orphanCreatedAtSkew = 5 * time.Minute

// createWithRecovery runs create, which returns a secret only once and so must not be retried blindly.
// After an ambiguous failure, deleteOrphan removes what the failed attempt may have created, and create
// runs once more. cleanupHint tells the user what to clean up when that isn't possible.
func createWithRecovery[T any](
	what string,
	create func() (T, error),
	deleteOrphan func(startedAt time.Time) error,
	cleanupHint string,
) (T, error) {
	var zero T
	startedAt := time.Now()
	res, err := create()
	if !errors.Is(err, ErrAmbiguous) {
		return res, err
	}

	log.Printf("creating %s failed ambiguously, checking for a partially created one: %v", what, err)
	if err := deleteOrphan(startedAt); err != nil {
		return zero, fmt.Errorf(
			"error creating %s: %w; could not clean up what the failed request may have created (%w), %s before retrying",
			what, ErrAmbiguous, err, cleanupHint)
	}

	res, err = create()
	if errors.Is(err, ErrAmbiguous) {
		return zero, fmt.Errorf("error creating %s: %w; %s before retrying", what, err, cleanupHint)
	}
	return res, err
}

// requireCreatedSince errors unless the listed object was created after startedAt, allowing for clock
// skew, so recovery never deletes an object that existed before the request.
func requireCreatedSince(kind, name, id, createdAt string, startedAt time.Time) error {
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return fmt.Errorf("%s %s (%s) has unparseable created_at %q: %w", kind, name, id, createdAt, err)
	}
	if t.Before(startedAt.Add(-orphanCreatedAtSkew)) {
		return fmt.Errorf("%s %s (%s) was created at %s, before this request", kind, name, id, createdAt)
	}
	return nil
}
