//go:build darwin

package appshell

import (
	"errors"
	"testing"
)

func TestLoginItemCoordinatorTick(t *testing.T) {
	var setCalls []bool
	var migrateCalls int
	c := &loginItemCoordinator{
		fetch: func() (bool, error) { return true, nil },
		set:   func(b bool) error { setCalls = append(setCalls, b); return nil },
		migrate: func() (bool, error) {
			migrateCalls++
			return true, nil
		},
		logf: func(string, ...any) {},
	}

	c.tick()
	if len(setCalls) != 1 || !setCalls[0] {
		t.Fatalf("first tick should apply desired=true, got %v", setCalls)
	}

	// No change → no further set/migrate calls.
	c.tick()
	c.tick()
	if migrateCalls != 1 {
		t.Fatalf("migration should run once, got %d", migrateCalls)
	}
	if len(setCalls) != 1 {
		t.Fatalf("unchanged desired state should not re-apply, got %v", setCalls)
	}

	// Toggle in the webui → exactly one new apply.
	c.fetch = func() (bool, error) { return false, nil }
	c.tick()
	c.tick()
	if len(setCalls) != 2 || setCalls[1] {
		t.Fatalf("toggle should apply once, got %v", setCalls)
	}
}

func TestLoginItemCoordinatorRetriesAfterFailure(t *testing.T) {
	var calls int
	c := &loginItemCoordinator{
		fetch: func() (bool, error) { return false, errors.New("daemon down") },
		set:   func(bool) error { return nil },
		migrate: func() (bool, error) {
			calls++
			return false, nil
		},
		logf: func(string, ...any) {},
	}
	c.tick()
	if calls != 0 {
		t.Fatal("fetch failure should skip migrate")
	}
	if c.applied != nil {
		t.Fatal("nothing should be applied when fetch fails")
	}

	// Daemon recovers → migration + apply go through on the next tick.
	c.fetch = func() (bool, error) { return true, nil }
	c.tick()
	if calls != 1 || c.applied == nil || !*c.applied {
		t.Fatalf("recovered tick should migrate and apply, calls=%d applied=%v", calls, c.applied)
	}
}

func TestLoginItemCoordinatorMigrateFailureRetried(t *testing.T) {
	var migrateAttempts int
	var setCalls int
	fail := true
	c := &loginItemCoordinator{
		fetch: func() (bool, error) { return true, nil },
		set: func(bool) error {
			setCalls++
			return nil
		},
		migrate: func() (bool, error) {
			migrateAttempts++
			if fail {
				return false, errors.New("boom")
			}
			return false, nil
		},
		logf: func(string, ...any) {},
	}
	c.tick()
	if setCalls != 0 {
		t.Fatal("apply must wait for successful migration")
	}
	fail = false
	c.tick()
	if migrateAttempts != 2 || setCalls != 1 {
		t.Fatalf("migration retry then apply, attempts=%d set=%d", migrateAttempts, setCalls)
	}
}
