package main

import (
	"./api"
	"log"
	"sync"
)

// Immutable structure that holds incoming grains from neighbor / env grains.
type NeighborImport struct {
	// Incoming grains.
	IncomingGrains []*api.Grain

	// Frozen neighbor chunks. (in their coordinates)
	EnvGrains map[string][]*api.Grain
}

// Immutable structure that holds outgoing grains / env grains visible from other
// chunks.
type NeighborExport struct {
	// Can be a subset of a chunk, but it's safe to pass everything.
	ChunkGrains []*api.Grain

	// Frozen escaped grains after canonicalization.
	EscapedGrains map[string][]*api.Grain
}

type ImportRequest struct {
	timestamp uint64
	topo      *api.ChunkTopology
	receiver  chan *NeighborImport
}

type SnapshotRequest struct {
	timestamp         uint64
	targetChunkIds    map[string]bool
	collectedSnapshot map[string]*api.ChunkSnapshot
	receiver          chan *api.SnapshotS
}

// Apparently, we only need to remember two timestamps for stepping to always work.
type ExportCache struct {
	chunkId string
	// Timestamp of head.
	timestamp  uint64
	head, prev *NeighborExport
}

// All exported methods are thread-safe, and are supposed to be called from goroutines
// simulating each chunk or grpc request handler.
type ChunkRouter struct {
	requestQueue chan *ImportRequest
	exportQueue  chan *ExportCache

	stateMutex    sync.Mutex
	snapshotReqs  []*SnapshotRequest
	exportCache   map[string]*ExportCache
	runningChunks map[string]*api.ChunkTopology
}

// Create a new chunk.
func StartNewRouter() *ChunkRouter {
	log.Printf("Starting chunk router")
	router := &ChunkRouter{
		requestQueue:  make(chan *ImportRequest, 10),
		exportQueue:   make(chan *ExportCache, 10),
		exportCache:   make(map[string]*ExportCache),
		runningChunks: make(map[string]*api.ChunkTopology),
	}
	go func() {
		var reqs []ImportRequest
		for {
			select {
			case req := <-router.requestQueue:
				router.stateMutex.Lock()
				// It's possible that new request is already satisfied by existing exports.
				reqs = append(reqs, maybeResolveRequests([]ImportRequest{*req}, router.exportCache)...)
				router.stateMutex.Unlock()
			case export := <-router.exportQueue:
				router.stateMutex.Lock()
				if router.exportCache[export.chunkId] == nil {
					router.exportCache[export.chunkId] = export
				} else {
					if export.timestamp != router.exportCache[export.chunkId].timestamp+1 {
						log.Panicf("Invalid export (chunk id=%s), current cache HEAD=%d, exported=%d",
							export.chunkId, router.exportCache[export.chunkId].timestamp, export.timestamp)
					}
					router.exportCache[export.chunkId].timestamp++
					router.exportCache[export.chunkId].prev = router.exportCache[export.chunkId].head
					router.exportCache[export.chunkId].head = export.head
				}
				reqs = maybeResolveRequests(reqs, router.exportCache)
				router.snapshotReqs = updateSnapshotReqs(router.snapshotReqs, export)
				router.stateMutex.Unlock()
			}
		}
	}()
	return router
}

// Resolve some requests and returns requests still pending.
func maybeResolveRequests(reqs []ImportRequest, exportCache map[string]*ExportCache) []ImportRequest {
	var pendingReqs []ImportRequest
	for _, req := range reqs {
		goodExports := make(map[string]*NeighborExport)
		for _, neighbor := range req.topo.Neighbors {
			export, ok := exportCache[neighbor.ChunkId]
			if !ok {
				break
			}
			if export.head != nil && req.timestamp == export.timestamp {
				goodExports[neighbor.ChunkId] = export.head
			} else if export.prev != nil && req.timestamp == export.timestamp-1 {
				goodExports[neighbor.ChunkId] = export.prev
			} else {
				break
			}
		}
		// Not enough exports found.
		if len(goodExports) < len(req.topo.Neighbors) {
			pendingReqs = append(pendingReqs, req)
			continue
		}
		// Ok to send out.
		nImport := &NeighborImport{
			IncomingGrains: nil,
			EnvGrains:      make(map[string][]*api.Grain),
		}
		for chunkId, export := range goodExports {
			nImport.IncomingGrains = append(nImport.IncomingGrains, export.EscapedGrains[req.topo.ChunkId]...)
			nImport.EnvGrains[chunkId] = export.ChunkGrains
		}
		req.receiver <- nImport
	}
	return pendingReqs
}

// Call this with every new export. Each snapshotrequest will collect necessary information,
// and resolve itself when it's ready.
func updateSnapshotReqs(reqs []*SnapshotRequest, newExport *ExportCache) []*SnapshotRequest {
	var pendingReqs []*SnapshotRequest
	for _, req := range reqs {
		if req.targetChunkIds[newExport.chunkId] && req.collectedSnapshot[newExport.chunkId] == nil {
			if newExport.timestamp == req.timestamp {
				req.collectedSnapshot[newExport.chunkId] = &api.ChunkSnapshot{
					Grains: newExport.head.ChunkGrains,
				}
			} else if newExport.timestamp > req.timestamp {
				log.Printf("Somehow missed capturing snapshot of chunk %s @ %d (already @ %d)",
					newExport.chunkId, req.timestamp, newExport.timestamp)
			}
		}

		if len(req.targetChunkIds) == len(req.collectedSnapshot) {
			log.Printf("SnapshotRequest resolved (ids=%#v@t=%d)", req.targetChunkIds, req.timestamp)
			req.receiver <- &api.SnapshotS{
				Timestamp: req.timestamp,
				Snapshot:  req.collectedSnapshot,
			}
		} else {
			pendingReqs = append(pendingReqs, req)
		}
	}
	return pendingReqs
}

// Take synchronized snapshot of given chunks at earliest convenient timestamp.
// Note that depending on given chunkIds and/or execution status, it might never
// return and waste memory forever.
// (e.g. when they're from different biospheres running at totally different timestamp)
func (router *ChunkRouter) RequestSnapshot(chunkIds []string) chan *api.SnapshotS {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	ch := make(chan *api.SnapshotS, 1)

	// Calculate nearest not-exported timestamp.
	targetChunkIds := make(map[string]bool)
	minTimestamp := uint64(0xffffffffffffffff)
	maxTimestamp := uint64(0)
	for _, chunkId := range chunkIds {
		cache, ok := router.exportCache[chunkId]
		if !ok {
			log.Printf("Specified chunk id (%s) not registered; ignoring", chunkId)
			continue
		}
		if cache.timestamp < minTimestamp {
			minTimestamp = cache.timestamp
		}
		if cache.timestamp > maxTimestamp {
			maxTimestamp = cache.timestamp
		}
		targetChunkIds[chunkId] = true
	}
	if len(targetChunkIds) == 0 {
		log.Printf("No valid id was specified; returning empty snapshot")
		ch <- &api.SnapshotS{}
		return ch
	}
	if minTimestamp+uint64(len(chunkIds)) < maxTimestamp {
		log.Printf("Too different timestamp observed (min: %d, max: %d, delta:%d) in %v, impossible to synchronize. Returning empty snapshot",
			minTimestamp, maxTimestamp, maxTimestamp-minTimestamp, chunkIds)
		ch <- &api.SnapshotS{}
		return ch
	}
	targetTimestamp := maxTimestamp + 1
	log.Printf("Registering snapshot request with target (%d) %v", targetTimestamp, targetChunkIds)
	router.snapshotReqs = append(router.snapshotReqs, &SnapshotRequest{
		timestamp:         targetTimestamp,
		targetChunkIds:    targetChunkIds,
		collectedSnapshot: make(map[string]*api.ChunkSnapshot),
		receiver:          ch,
	})
	return ch
}

func (router *ChunkRouter) GetChunks() []*api.ChunkTopology {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	var topos []*api.ChunkTopology
	for _, topo := range router.runningChunks {
		topos = append(topos, topo)
	}
	return topos
}

func (router *ChunkRouter) DeleteAllChunks() {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	// TODO: implement
}

// Returns true if caller should continue RequestNeighbor & NotifyResult.
// When false is returned, caller must not touch router again because it's already
// executed by other goroutine.
func (router *ChunkRouter) RegisterNewChunk(topo *api.ChunkTopology) bool {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	if router.runningChunks[topo.ChunkId] != nil {
		log.Printf("Trying to run chunk %s even though it's running, ignoring", topo.ChunkId)
		return false
	}
	router.runningChunks[topo.ChunkId] = topo
	return true
}

// Request neighbors necessary for stepping chunk from timestap to timetamp+1,
// with given topo. Returns a channel that returns that data once.
func (router *ChunkRouter) RequestNeighbor(timestamp uint64, topo *api.ChunkTopology) chan *NeighborImport {
	ch := make(chan *NeighborImport, 1)
	router.requestQueue <- &ImportRequest{timestamp, topo, ch}
	return ch
}

func (router *ChunkRouter) NotifyResult(timestamp uint64, topo *api.ChunkTopology, export *NeighborExport) {
	router.exportQueue <- &ExportCache{
		chunkId:   topo.ChunkId,
		timestamp: timestamp,
		head:      export,
	}
}
