package httpapi

import (
	"context"
	"encoding/json"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"time"
)

// Snapshots are always metadata-only and never contain playback URLs.
func (a *api) readSnapshot(ctx context.Context, sourceID, typ, catID string) ([]catalog.Preview, time.Duration, bool) {
	raw, refreshed, err := a.db.CatalogSnapshot(ctx, sourceID, typ, catID)
	if err != nil {
		return nil, 0, false
	}
	var items []catalog.Preview
	if json.Unmarshal(raw, &items) != nil || items == nil || len(items) > 25 {
		return nil, 0, false
	}
	return items, time.Since(refreshed), true
}
func (a *api) persistSnapshot(ctx context.Context, sourceID, typ, catID string, items []catalog.Preview) {
	data, err := json.Marshal(items)
	if err != nil {
		return
	}
	_ = a.db.SaveCatalogSnapshot(ctx, sourceID, typ, catID, data)
}
func (a *api) refreshSnapshot(key, sourceID, manifestURL, typ, catID string) {
	a.cache.mu.Lock()
	if a.cache.refreshing[key] || len(a.cache.refreshing) >= 32 {
		a.cache.mu.Unlock()
		return
	}
	a.cache.refreshing[key] = true
	a.cache.mu.Unlock()
	go func() {
		defer func() {
			a.cache.mu.Lock()
			delete(a.cache.refreshing, key)
			a.cache.mu.Unlock()
		}()
		queueCtx, queueCancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer queueCancel()
		select {
		case a.cache.refreshSlots <- struct{}{}:
		case <-queueCtx.Done():
			return
		}
		defer func() { <-a.cache.refreshSlots }()
		ctx, cancel := context.WithTimeout(context.Background(), 11*time.Second)
		defer cancel()
		items, err := a.catalogClient.Catalog(ctx, manifestURL, typ, catID)
		if err != nil {
			return
		}
		source, readErr := a.db.GetCatalogSource(ctx, sourceID)
		if readErr != nil || source.CacheMode == "off" {
			return
		}
		a.persistSnapshot(ctx, sourceID, typ, catID, items)
		a.cache.mu.Lock()
		a.cache.data[key] = catalogCacheEntry{items: items, expiry: time.Now().Add(90 * time.Second)}
		a.cache.mu.Unlock()
	}()
}
