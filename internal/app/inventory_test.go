package app

import (
	"sync"
	"testing"
	"time"
)

func TestOfferLatestReplacesStaleProgress(t *testing.T) {
	updates := make(chan InventoryProgress, 1)
	offerLatest(updates, InventoryProgress{Scanned: 1})
	offerLatest(updates, InventoryProgress{Scanned: 2})

	if got := (<-updates).Scanned; got != 2 {
		t.Fatalf("latest progress = %d, want 2", got)
	}
}

func TestInventoryServiceEmitsCompletion(t *testing.T) {
	var mu sync.Mutex
	var events []InventoryProgress
	service := NewInventoryService()
	service.total = 3
	service.interval = time.Millisecond
	service.emit = func(_ string, data ...any) bool {
		mu.Lock()
		events = append(events, data[0].(InventoryProgress))
		mu.Unlock()
		return false
	}

	if err := service.StartMockScan(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for service.Running() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 || !events[len(events)-1].Done || events[len(events)-1].Scanned != 3 {
		t.Fatalf("completion event missing: %#v", events)
	}
}

func TestInventoryServiceRejectsConcurrentScans(t *testing.T) {
	service := NewInventoryService()
	service.total = 100
	service.interval = time.Millisecond
	service.emit = func(string, ...any) bool { return false }

	if err := service.StartMockScan(); err != nil {
		t.Fatal(err)
	}
	if err := service.StartMockScan(); err == nil {
		t.Fatal("concurrent scan was accepted")
	}
	service.CancelMockScan()
}
