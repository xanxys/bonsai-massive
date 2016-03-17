package main

import (
	"./api"
	"log"
	"sync"
	"time"
)

// Immutable structure that holds outgoing grains / env grains visible from other
// chunks.
type NeighborExport struct {
	OriginChunkId string

	Timestamp uint64

	// Can be a subset of a chunk, but it's safe to pass everything.
	ChunkGrains []*api.Grain

	// We don't care much about performance, but rather this will prevent
	// potential grain duplication.
	// key: destination chunk id
	// value: grain in destionatio coordinate
	EscapedGrains map[string][]*api.Grain
}

type SnapshotRequest struct {
	receiver       chan *api.SnapshotS
	targetChunkIds map[string]bool
	expireAt       time.Time

	// Phase 1. Determine target timestamp
	collectedTimestamp map[string]uint64

	// Phase 2. Collect snapshot.
	isTimestampValid  bool
	timestamp         uint64
	collectedSnapshot map[string]*NeighborExport
}

type ChunkMetadata struct {
	topo   *api.ChunkTopology
	quitCh chan bool
	recvCh chan *NeighborExport
}

// All exported methods are thread-safe, and are supposed to be called from goroutines
// simulating each chunk or grpc request handler.
type ChunkRouter struct {
	stateMutex    sync.Mutex
	snapshotReqs  []*SnapshotRequest
	runningChunks map[string]*ChunkMetadata
}

// Create a new chunk.
func StartNewRouter() *ChunkRouter {
	log.Printf("Starting chunk router")
	router := &ChunkRouter{
		runningChunks: make(map[string]*ChunkMetadata),
	}
	go func() {
		for {
			time.Sleep(time.Second)
			router.MaybeExpireRequests()
		}
	}()
	return router
}

// Take synchronized snapshot of given chunks at earliest convenient timestamp.
func (router *ChunkRouter) RequestSnapshot(chunkIds []string, waitFor time.Duration) chan *api.SnapshotS {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	ch := make(chan *api.SnapshotS, 1)
	targetChunkIds := make(map[string]bool)
	for _, chunkId := range chunkIds {
		targetChunkIds[chunkId] = true
	}
	if len(targetChunkIds) == 0 {
		log.Printf("WARNING: No valid id was specified; returning empty snapshot")
		ch <- &api.SnapshotS{}
		return ch
	}
	log.Printf("Registering snapshot request with target chunkIds=%v", targetChunkIds)
	router.snapshotReqs = append(router.snapshotReqs, &SnapshotRequest{
		receiver:           ch,
		targetChunkIds:     targetChunkIds,
		expireAt:           time.Now().Add(waitFor),
		collectedTimestamp: make(map[string]uint64),
	})
	return ch
}

func (router *ChunkRouter) MaybeExpireRequests() {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	router.maybeResolveSnapshotRequests(nil)
}

func (router *ChunkRouter) maybeResolveSnapshotRequests(maybePacket *NeighborExport) {
	var qs []*SnapshotRequest
	for _, q := range router.snapshotReqs {
		if !router.maybeResolveSnapshotRequest(q, maybePacket) {
			qs = append(qs, q)
		}
	}
	router.snapshotReqs = qs
}

// Returns: resolved?
// Need to be called with locked mutex.
func (router *ChunkRouter) maybeResolveSnapshotRequest(q *SnapshotRequest, packet *NeighborExport) bool {
	if time.Now().After(q.expireAt) {
		log.Printf("WARNING: Snapshot request expired; returning empty snapshot")
		q.receiver <- &api.SnapshotS{}
		return true
	}
	if packet == nil || !q.targetChunkIds[packet.OriginChunkId] {
		return false
	}
	if !q.isTimestampValid {
		q.collectedTimestamp[packet.OriginChunkId] = packet.Timestamp
		if len(q.collectedTimestamp) < len(q.targetChunkIds) {
			return false
		}
		maxTimestamp := uint64(0)
		for _, timestamp := range q.collectedTimestamp {
			if timestamp > maxTimestamp {
				maxTimestamp = timestamp
			}
		}
		q.isTimestampValid = true
		q.timestamp = maxTimestamp + 1
		q.collectedSnapshot = make(map[string]*NeighborExport)
		return false
	} else {
		if q.timestamp != packet.Timestamp {
			return false
		}
		q.collectedSnapshot[packet.OriginChunkId] = packet
		if len(q.collectedSnapshot) < len(q.targetChunkIds) {
			return false
		}
		q.receiver <- assembleSnapshot(q)
		return true
	}
}

func assembleSnapshot(q *SnapshotRequest) *api.SnapshotS {
	bins := make(map[string][]*api.Grain)
	for srcChunkId, packet := range q.collectedSnapshot {
		bins[srcChunkId] = append(bins[srcChunkId], packet.ChunkGrains...)
		for dstChunkId, grains := range packet.EscapedGrains {
			bins[dstChunkId] = append(bins[dstChunkId], grains...)
		}
	}
	// 	__        ___    ____  _   _ ___ _   _  ____
	// \ \      / / \  |  _ \| \ | |_ _| \ | |/ ___|
	//  \ \ /\ / / _ \ | |_) |  \| || ||  \| | |  _
	//   \ V  V / ___ \|  _ <| |\  || || |\  | |_| |
	//    \_/\_/_/   \_\_| \_\_| \_|___|_| \_|\____|
	//
	// When particles are emitted (ParticleSource exists), these snapshot does not
	// reflect accurate snapshot. (emitted particles are not included)

	s := &api.SnapshotS{
		Timestamp: q.timestamp,
		Snapshot:  make(map[string]*api.ChunkSnapshot),
	}
	for chunkId, grains := range bins {
		s.Snapshot[chunkId] = &api.ChunkSnapshot{
			Grains: grains,
		}
	}
	return s
}

func (router *ChunkRouter) GetChunks() []*api.ChunkTopology {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	var topos []*api.ChunkTopology
	for _, chunkMeta := range router.runningChunks {
		topos = append(topos, chunkMeta.topo)
	}
	return topos
}

func (router *ChunkRouter) DeleteChunk(chunkId string) {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	chunkMeta, ok := router.runningChunks[chunkId]
	if !ok {
		log.Printf("WARNING: Trying to delete non-running chunk %s", chunkId)
		return
	}
	chunkMeta.quitCh <- true
	delete(router.runningChunks, chunkId)
}

// Returns quit channel if caller should continue RequestNeighbor & NotifyResult.
// When nil is returned, caller must not touch router again because it's already
// executed by other goroutine.
func (router *ChunkRouter) RegisterNewChunk(topo *api.ChunkTopology) *ChunkMetadata {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	if router.runningChunks[topo.ChunkId] != nil {
		log.Printf("Trying to run chunk %s even though it's running, ignoring", topo.ChunkId)
		return nil
	}
	meta := &ChunkMetadata{
		topo:   topo,
		quitCh: make(chan bool),
		recvCh: make(chan *NeighborExport, 10),
	}
	router.runningChunks[topo.ChunkId] = meta
	return meta
}

func (router *ChunkRouter) MulticastToNeighbors(nodes []*api.ChunkTopology_ChunkNeighbor, packet *NeighborExport) {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	for _, node := range nodes {
		if node.Internal {
			meta, ok := router.runningChunks[node.ChunkId]
			if ok {
				// TODO: we can optimzie by dropping escaped grains of different destination.
				meta.recvCh <- packet
			} else {
				log.Printf("WARNING: Mutlicast target %s is not running. packet %#v dropped", node.ChunkId, packet)
			}
		} else {
			log.Printf("ERROR: Multicast to external node not supported yet")
		}
	}
	router.maybeResolveSnapshotRequests(packet)
}
