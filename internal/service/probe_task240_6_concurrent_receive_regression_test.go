package service_test

import (
	"errors"
	"sync"
	"testing"

	"task240-fedlineage/internal/model"
)

func TestTask240Bug06ConcurrentReceiveIsIdempotent(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	const participants = 20
	start := make(chan struct{})
	results := make(chan *model.ClientUpdate, participants)
	errs := make(chan error, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			u, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-shared", RoundID: "r1", ClientID: "c1", ParamDigest: "d1", Dimension: 8})
			if err != nil {
				errs <- err
				return
			}
			results <- u
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent receive returned error: %v", err)
	}
	newCount, replayCount := 0, 0
	for u := range results {
		switch u.State {
		case model.UpdateStateNew:
			newCount++
		case model.UpdateStateReplay:
			replayCount++
		default:
			t.Fatalf("unexpected state %q", u.State)
		}
	}
	if newCount != 1 || replayCount != participants-1 {
		t.Fatalf("expected one new and %d replay, got new=%d replay=%d", participants-1, newCount, replayCount)
	}
	stored, err := sv.Update.Get("u-shared")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RoundID != "r1" || stored.ClientID != "c1" || stored.ParamDigest != "d1" || stored.Dimension != 8 {
		t.Fatalf("stored identity changed: %+v", stored)
	}
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-shared", RoundID: "r1", ClientID: "c2", ParamDigest: "d1", Dimension: 8}); !errors.Is(err, model.ErrUpdateConflict) {
		t.Fatalf("mutated duplicate identity was accepted: %v", err)
	}
}
