package main

import (
	"./api"
	"math/rand"
	"sync"
	"time"
)

const EXPIRATION_DURATION = 5 * time.Minute

// Thread safe TTL-ed cache of immutable chunk snapshots.
// Note TTL gurantees minimum time held in memory, but NOT
// when it's deleted (it can stay forever if Add is not called, thus not refreshed).
type SnapshotCache struct {
	lock      sync.Mutex
	snapshots map[uint64]*cacheEntry
}

type cacheEntry struct {
	created time.Time
	data    *api.ChunkState
}

func NewSnapshotCache() *SnapshotCache {
	return &SnapshotCache{
		snapshots: make(map[uint64]*cacheEntry),
	}
}

// Insert a new entry to the cache. Expired cache are also deleted.
func (cache *SnapshotCache) Add(data *api.ChunkState) uint64 {
	key := uint64(rand.Int63())

	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.lockedRefresh()
	cache.snapshots[key] = &cacheEntry{
		created: time.Now(),
		data:    data,
	}
	return key
}

// Returns nil if not found (including expried).
func (cache *SnapshotCache) Get(key uint64) *api.ChunkState {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	entry, _ := cache.snapshots[key]
	if entry == nil {
		return nil
	}
	return entry.data
}

func (cache *SnapshotCache) Count() int64 {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return int64(len(cache.snapshots))
}

func (cache *SnapshotCache) lockedRefresh() {
	for key, entry := range cache.snapshots {
		if time.Now().Sub(entry.created) > EXPIRATION_DURATION {
			delete(cache.snapshots, key)
		}
	}
}
