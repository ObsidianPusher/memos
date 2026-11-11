package v1

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	defaultClashSubscriptionUpdateIntervalSeconds int64 = 3600
	minClashSubscriptionUpdateIntervalSeconds     int64 = 60
	maxClashSubscriptionUpdateIntervalSeconds     int64 = 30 * 24 * 3600
)

var errClashSubscriptionPermissionDenied = stderrors.New("permission denied")

type ClashVergeSubscription struct {
	ID                    string `json:"id"`
	UserID                int32  `json:"user_id"`
	Name                  string `json:"name"`
	SourceType            string `json:"source_type,omitempty"`
	Source                string `json:"source"`
	Content               string `json:"content,omitempty"`
	Include               string `json:"include"`
	AutoUpdate            bool   `json:"auto_update"`
	UpdateIntervalSeconds int64  `json:"update_interval_seconds"`
	LastRefreshTs         int64  `json:"last_refresh_ts"`
	LastRefreshError      string `json:"last_refresh_error,omitempty"`
	CreatedTs             int64  `json:"created_ts"`
	UpdatedTs             int64  `json:"updated_ts"`
}

type clashVergeSubscriptionStore struct {
	path string

	mu     sync.Mutex
	loaded bool
	items  map[string]*ClashVergeSubscription
}

func newClashVergeSubscriptionStore(path string) *clashVergeSubscriptionStore {
	return &clashVergeSubscriptionStore{
		path:  path,
		items: make(map[string]*ClashVergeSubscription),
	}
}

func (s *clashVergeSubscriptionStore) ListByUser(userID int32) ([]*ClashVergeSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	list := make([]*ClashVergeSubscription, 0)
	for _, item := range s.items {
		if item.UserID != userID {
			continue
		}
		list = append(list, cloneClashVergeSubscription(item))
	}
	slices.SortFunc(list, func(a, b *ClashVergeSubscription) int {
		if a.UpdatedTs == b.UpdatedTs {
			if a.CreatedTs == b.CreatedTs {
				if a.ID < b.ID {
					return -1
				}
				if a.ID > b.ID {
					return 1
				}
				return 0
			}
			if a.CreatedTs > b.CreatedTs {
				return -1
			}
			return 1
		}
		if a.UpdatedTs > b.UpdatedTs {
			return -1
		}
		return 1
	})
	return list, nil
}

func (s *clashVergeSubscriptionStore) Get(id string) (*ClashVergeSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	item := s.items[id]
	if item == nil {
		return nil, nil
	}
	return cloneClashVergeSubscription(item), nil
}

func (s *clashVergeSubscriptionStore) Create(userID int32, name, sourceType, source, content, include string, autoUpdate bool, intervalSeconds int64) (*ClashVergeSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	item := &ClashVergeSubscription{
		ID:                    uuid.NewString(),
		UserID:                userID,
		Name:                  name,
		SourceType:            sourceType,
		Source:                source,
		Content:               content,
		Include:               include,
		AutoUpdate:            autoUpdate,
		UpdateIntervalSeconds: normalizeUpdateInterval(autoUpdate, intervalSeconds),
		CreatedTs:             now,
		UpdatedTs:             now,
	}
	s.items[item.ID] = item
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneClashVergeSubscription(item), nil
}

func (s *clashVergeSubscriptionStore) Update(id string, userID int32, name, sourceType, source, content, include string, autoUpdate bool, intervalSeconds int64) (*ClashVergeSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	item := s.items[id]
	if item == nil {
		return nil, nil
	}
	if item.UserID != userID {
		return nil, errClashSubscriptionPermissionDenied
	}
	item.Name = name
	item.SourceType = sourceType
	item.Source = source
	item.Content = content
	item.Include = include
	item.AutoUpdate = autoUpdate
	item.UpdateIntervalSeconds = normalizeUpdateInterval(autoUpdate, intervalSeconds)
	item.UpdatedTs = time.Now().Unix()
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneClashVergeSubscription(item), nil
}

func (s *clashVergeSubscriptionStore) UpdateRefreshState(id string, refreshTs int64, lastError string) (*ClashVergeSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	item := s.items[id]
	if item == nil {
		return nil, nil
	}
	if refreshTs > 0 {
		item.LastRefreshTs = refreshTs
	}
	item.LastRefreshError = lastError
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneClashVergeSubscription(item), nil
}

func (s *clashVergeSubscriptionStore) Delete(id string, userID int32) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return false, err
	}

	item := s.items[id]
	if item == nil {
		return false, nil
	}
	if item.UserID != userID {
		return false, errClashSubscriptionPermissionDenied
	}
	delete(s.items, id)
	if err := s.saveLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *clashVergeSubscriptionStore) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	s.loaded = true

	content, err := os.ReadFile(s.path)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.Wrap(err, "failed to read clash verge subscription store")
	}
	if len(content) == 0 {
		return nil
	}

	var items []*ClashVergeSubscription
	if err := json.Unmarshal(content, &items); err != nil {
		return errors.Wrap(err, "failed to parse clash verge subscription store")
	}
	for _, item := range items {
		if item == nil || item.ID == "" {
			continue
		}
		// Backward compatibility for legacy records created before source type and update policy fields existed.
		if item.SourceType == "" {
			item.SourceType = clashSubscriptionSourceURL
		}
		if item.SourceType == clashSubscriptionSourceURL && !item.AutoUpdate && item.UpdateIntervalSeconds == 0 && item.LastRefreshTs == 0 {
			item.AutoUpdate = true
		}
		item.UpdateIntervalSeconds = normalizeUpdateInterval(item.AutoUpdate, item.UpdateIntervalSeconds)
		s.items[item.ID] = item
	}
	return nil
}

func (s *clashVergeSubscriptionStore) saveLocked() error {
	list := make([]*ClashVergeSubscription, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	slices.SortFunc(list, func(a, b *ClashVergeSubscription) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})

	payload, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal clash verge subscriptions")
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return errors.Wrap(err, "failed to create clash verge subscription store directory")
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return errors.Wrap(err, "failed to write clash verge subscription temp store")
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return errors.Wrap(err, "failed to rename clash verge subscription store")
	}
	return nil
}

func normalizeUpdateInterval(autoUpdate bool, intervalSeconds int64) int64 {
	if !autoUpdate {
		return 0
	}
	if intervalSeconds == 0 {
		return defaultClashSubscriptionUpdateIntervalSeconds
	}
	if intervalSeconds < minClashSubscriptionUpdateIntervalSeconds {
		return minClashSubscriptionUpdateIntervalSeconds
	}
	if intervalSeconds > maxClashSubscriptionUpdateIntervalSeconds {
		return maxClashSubscriptionUpdateIntervalSeconds
	}
	return intervalSeconds
}

func cloneClashVergeSubscription(item *ClashVergeSubscription) *ClashVergeSubscription {
	if item == nil {
		return nil
	}
	copy := *item
	return &copy
}
