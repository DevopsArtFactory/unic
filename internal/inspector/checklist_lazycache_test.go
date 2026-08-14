package inspector

import (
	"errors"
	"testing"
)

func TestLazyCacheFetchesOnceAndMemoizesError(t *testing.T) {
	var cache lazyCache[int]
	calls := 0

	value, err := cache.Get(func() (int, error) {
		calls++
		return 42, nil
	})
	if err != nil || value != 42 {
		t.Fatalf("expected first fetch to succeed, got %d err=%v", value, err)
	}

	value, err = cache.Get(func() (int, error) {
		calls++
		return 0, errors.New("should not run")
	})
	if err != nil || value != 42 || calls != 1 {
		t.Fatalf("expected memoized value without refetch, got %d err=%v calls=%d", value, err, calls)
	}

	var failing lazyCache[int]
	wantErr := errors.New("fetch failed")
	if _, err := failing.Get(func() (int, error) { return 0, wantErr }); err != wantErr {
		t.Fatalf("expected fetch error, got %v", err)
	}
	if _, err := failing.Get(func() (int, error) {
		t.Fatal("failed fetch must not be retried")
		return 0, nil
	}); err != wantErr {
		t.Fatalf("expected memoized error, got %v", err)
	}
}
