package service_test

import (
	"errors"
	"sync"
	"testing"

	"task240-fedlineage/internal/model"
)

func TestTask240Bug07ConcurrentSnapshotPublishHasSingleWinner(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Close("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Aggregate.Confirm("r1"); err != nil {
		t.Fatal(err)
	}
	const participants = 20
	start := make(chan struct{})
	results := make(chan *model.RoundSnapshot, participants)
	errs := make(chan error, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		id := "s-" + string(rune('a'+i))
		wg.Add(1)
		go func(snapshotID string) {
			defer wg.Done()
			<-start
			snap, err := sv.Snapshot.Publish(snapshotID, "r1")
			if err != nil {
				errs <- err
				return
			}
			results <- snap
		}(id)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	winners := 0
	for range results {
		winners++
	}
	conflicts := 0
	for err := range errs {
		if !errors.Is(err, model.ErrSnapshotConflict) {
			t.Fatalf("unexpected concurrent publish error: %v", err)
		}
		conflicts++
	}
	if winners != 1 || conflicts != participants-1 {
		t.Fatalf("expected one winner and %d conflicts, got winners=%d conflicts=%d", participants-1, winners, conflicts)
	}
	published, err := sv.Snapshot.ListByRound("r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 1 || published[0].State != model.SnapshotStatePublish {
		t.Fatalf("expected one published snapshot, got %+v", published)
	}
}
