// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookidoo/internal/cliutil"
)

func TestConcurrentProfileUpdatesDoNotLoseWrites(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)

	const count = 32
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		i := i
		go func() {
			name := fmt.Sprintf("profile-%02d", i)
			errCh <- updateProfileStore(func(s *profileStore) error {
				s.Profiles[name] = Profile{Name: name, Values: map[string]string{"limit": "1"}}
				return nil
			})
		}()
	}
	for i := 0; i < count; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent update failed: %v", err)
		}
	}

	store, err := loadProfileStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(store.Profiles); got != count {
		t.Fatalf("saved %d profiles after %d concurrent updates, want all", got, count)
	}
}
