package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const InventoryProgressEvent = "inventory:progress"

type InventoryProgress struct {
	Scanned   int  `json:"scanned"`
	Total     int  `json:"total"`
	Done      bool `json:"done"`
	Cancelled bool `json:"cancelled"`
}

type InventoryService struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	emit     func(string, ...any) bool
	total    int
	interval time.Duration
}

func init() {
	application.RegisterEvent[InventoryProgress]("inventory:progress")
}

func NewInventoryService() *InventoryService {
	return &InventoryService{
		emit: func(name string, data ...any) bool {
			return application.Get().Event.Emit(name, data...)
		},
		total:    100,
		interval: 15 * time.Millisecond,
	}
}

func (s *InventoryService) StartMockScan() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return errors.New("inventory scan already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.run(ctx)
	return nil
}

func (s *InventoryService) CancelMockScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *InventoryService) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

func (s *InventoryService) run(ctx context.Context) {
	updates := make(chan InventoryProgress, 1)
	delivered := make(chan struct{})
	go func() {
		defer close(delivered)
		for progress := range updates {
			s.emit(InventoryProgressEvent, progress)
		}
	}()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	scanned := 0
	for scanned < s.total {
		select {
		case <-ctx.Done():
			offerLatest(updates, InventoryProgress{Scanned: scanned, Total: s.total, Done: true, Cancelled: true})
			scanned = s.total
		case <-ticker.C:
			scanned++
			offerLatest(updates, InventoryProgress{Scanned: scanned, Total: s.total, Done: scanned == s.total})
		}
	}

	close(updates)
	<-delivered
	s.mu.Lock()
	s.cancel = nil
	s.mu.Unlock()
}

func offerLatest(updates chan InventoryProgress, progress InventoryProgress) {
	select {
	case updates <- progress:
	default:
		<-updates
		updates <- progress
	}
}
