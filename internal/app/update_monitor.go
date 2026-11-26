package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sysentra/internal/settings"
)

type updatePreferenceStore interface {
	Read() (settings.UpdateChecks, error)
	Write(settings.UpdateChecks) error
}

type UpdatePreferencesView struct {
	Scheduled       bool `json:"scheduled"`
	Notifications   bool `json:"notifications"`
	IntervalMinutes int  `json:"intervalMinutes"`
}

type UpdateCheckResultView struct {
	Total     int    `json:"total"`
	XBPS      int    `json:"xbps"`
	Flatpak   int    `json:"flatpak"`
	CheckedAt string `json:"checkedAt"`
}

type UpdateMonitorService struct {
	packages *PackageService
	store    updatePreferenceStore
	mu       sync.RWMutex
	prefs    settings.UpdateChecks
	checkMu  sync.Mutex
	reset    chan struct{}
	onResult func(UpdateCheckResultView, bool)
}

func NewUpdateMonitorService(ctx context.Context, packages *PackageService, store updatePreferenceStore, handler func(UpdateCheckResultView, bool)) (*UpdateMonitorService, error) {
	prefs, err := store.Read()
	if err != nil {
		return nil, err
	}
	service := &UpdateMonitorService{packages: packages, store: store, prefs: prefs, reset: make(chan struct{}, 1), onResult: handler}
	service.start(ctx)
	return service, nil
}

func (s *UpdateMonitorService) GetUpdatePreferences() UpdatePreferencesView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return updatePreferencesView(s.prefs)
}

func (s *UpdateMonitorService) SetUpdatePreferences(scheduled, notifications bool, intervalMinutes int) (UpdatePreferencesView, error) {
	prefs := settings.UpdateChecks{Scheduled: scheduled, Notifications: notifications, IntervalMinutes: intervalMinutes}
	if err := s.store.Write(prefs); err != nil {
		return UpdatePreferencesView{}, err
	}
	s.mu.Lock()
	s.prefs = prefs
	s.mu.Unlock()
	select {
	case s.reset <- struct{}{}:
	default:
	}
	return updatePreferencesView(prefs), nil
}

func (s *UpdateMonitorService) CheckForUpdates() (UpdateCheckResultView, error) {
	return s.check(false)
}

func (s *UpdateMonitorService) PublishUpdateCount(count int) error {
	if count < 0 {
		return fmt.Errorf("update count cannot be negative")
	}
	s.publish(UpdateCheckResultView{Total: count, CheckedAt: time.Now().Format(time.RFC3339)}, false)
	return nil
}

func (s *UpdateMonitorService) start(ctx context.Context) {
	go func() {
		for {
			s.mu.RLock()
			prefs := s.prefs
			s.mu.RUnlock()
			if !prefs.Scheduled {
				select {
				case <-ctx.Done():
					return
				case <-s.reset:
					continue
				}
			}
			timer := time.NewTimer(time.Duration(prefs.IntervalMinutes) * time.Minute)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-s.reset:
				timer.Stop()
			case <-timer.C:
				_, _ = s.check(true)
			}
		}
	}()
}

func (s *UpdateMonitorService) check(scheduled bool) (UpdateCheckResultView, error) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	catalog, err := s.packages.ListUpdates()
	if err != nil {
		return UpdateCheckResultView{}, err
	}
	result := UpdateCheckResultView{Total: len(catalog.Packages), CheckedAt: time.Now().Format(time.RFC3339)}
	for _, pkg := range catalog.Packages {
		if pkg.BackendID == "flatpak" {
			result.Flatpak++
		} else {
			result.XBPS++
		}
	}
	s.mu.RLock()
	notifications := s.prefs.Notifications
	s.mu.RUnlock()
	notify := scheduled && notifications && result.Total > 0
	s.publish(result, notify)
	return result, nil
}

func (s *UpdateMonitorService) publish(result UpdateCheckResultView, notify bool) {
	s.mu.RLock()
	handler := s.onResult
	s.mu.RUnlock()
	if handler != nil {
		handler(result, notify)
	}
}

func updatePreferencesView(value settings.UpdateChecks) UpdatePreferencesView {
	return UpdatePreferencesView{Scheduled: value.Scheduled, Notifications: value.Notifications, IntervalMinutes: value.IntervalMinutes}
}
