package inspector

import (
	"errors"
	"testing"
)

func TestLoadStateRunsOnceAndMemoizesError(t *testing.T) {
	var state loadState
	calls := 0

	if err := state.do(func() error { calls++; return nil }); err != nil {
		t.Fatalf("expected first load to succeed, got %v", err)
	}
	if err := state.do(func() error { calls++; return errors.New("should not run") }); err != nil {
		t.Fatalf("expected memoized success without reload, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one fetch, got %d", calls)
	}

	var failing loadState
	wantErr := errors.New("fetch failed")
	if err := failing.do(func() error { return wantErr }); err != wantErr {
		t.Fatalf("expected fetch error, got %v", err)
	}
	if err := failing.do(func() error {
		t.Fatal("failed load must not be retried")
		return nil
	}); err != wantErr {
		t.Fatalf("expected memoized error, got %v", err)
	}
}
