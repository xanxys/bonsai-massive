package main

import (
	"./api"
	"log"
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

// Apparently, we only need to remember two timestamps for stepping to always work.
type ExportCache struct {
	chunkId string
	// Timestamp of head.
	timestamp  uint64
	head, prev *NeighborExport
}

type ChunkRouter struct {
	requestQueue chan *ImportRequest
	exportQueue  chan *ExportCache
}

// Interface to exchange / synchronize chunk information.
//
// All methods are thread-safe, and are supposed to be called from goroutines
// simulating each chunk.
func StartNewRouter() *ChunkRouter {
	log.Printf("Starting chunk router")
	router := &ChunkRouter{
		requestQueue: make(chan *ImportRequest, 10),
		exportQueue:  make(chan *ExportCache, 10),
	}
	go func() {
		var reqs []ImportRequest
		exportCache := make(map[string]*ExportCache)
		for {
			select {
			case req := <-router.requestQueue:
				// It's possible that new request is already satisfied by existing exports.
				reqs = append(reqs, maybeResolveRequests([]ImportRequest{*req}, exportCache)...)
			case export := <-router.exportQueue:
				if export.timestamp != exportCache[export.chunkId].timestamp+1 {
					log.Panicf("Invalid export (chunk id=%s), current cache HEAD=%d, exported=%d",
						export.chunkId, exportCache[export.chunkId].timestamp, export.timestamp)
				}
				exportCache[export.chunkId].timestamp++
				exportCache[export.chunkId].prev = exportCache[export.chunkId].head
				exportCache[export.chunkId].head = export.head
				reqs = maybeResolveRequests(reqs, exportCache)
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

// Take synchronized snapshot of given chunks at earliest convenient timestamp.
// Note that depending on given chunkIds and/or execution status, it might never
// return and waste memory forever.
// (e.g. when they're from different biospheres running at totally different timestamp)
func (router *ChunkRouter) RequestSnapshot(chunkIds []string) chan *api.SnapshotS {
	ch := make(chan *api.SnapshotS, 1)
	return ch
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
