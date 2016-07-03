package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"sync"
	"time"
)

// Immutable structure that holds outgoing grains / env grains visible from other
// chunks.
// TODO: Migrate to proto.
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

// A packet destined to single node.
// Node must be something other than this node.
type PendingExport struct {
	Node   *api.ChunkTopology_ChunkNeighbor
	Packet *NeighborExport
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
	stateMutex     sync.Mutex
	snapshotReqs   []*SnapshotRequest
	runningChunks  map[string]*ChunkMetadata
	pendingExports []*PendingExport
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
			router.MaybeResolvePendingMulticast()
		}
	}()
	return router
}

func (router *ChunkRouter) GetRouterStatus() *api.StatusS {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()
	s := &api.StatusS{
		NumPendingExport: int64(len(router.pendingExports)),
	}
	return s
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

func (router *ChunkRouter) MaybeResolvePendingMulticast() {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	var newPending []*PendingExport
	for _, exp := range router.pendingExports {
		err := router.SendPendingExport(exp)
		if err != nil {
			log.Printf("WARNING: Resolving pending export failed. Trying again later.")
			newPending = append(newPending, exp)
		}
	}
	router.pendingExports = newPending
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
		quitCh: make(chan bool, 1), // Needs 1+ for DeleteChunk to not block
		recvCh: make(chan *NeighborExport, 10),
	}
	router.runningChunks[topo.ChunkId] = meta
	return meta
}

func (router *ChunkRouter) AcceptMulticastForExternalNodes(packet *api.NeighborExport) {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	meta, ok := router.runningChunks[packet.DestChunkId]
	if ok {
		escapedGrains := make(map[string][]*api.Grain, len(packet.EscapedGrains))
		for k, v := range packet.EscapedGrains {
			escapedGrains[k] = v.Grains
		}
		meta.recvCh <- &NeighborExport{
			OriginChunkId: packet.OriginChunkId,
			Timestamp:     packet.Timestamp,
			ChunkGrains:   packet.ChunkGrains,
			EscapedGrains: escapedGrains,
		}
	} else {
		log.Printf("WARNING: Received mutlicast packet, but destChunkId %s is not running. packet %#v dropped", packet.DestChunkId, packet)
	}
}

// Multicast given packet without blocking. Uncessful multicasts are
// moved to pending list and processed every second.
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
				log.Printf("ERROR: Internal mutlicast target %s is not running. packet %#v dropped", node.ChunkId, packet)
			}
		} else {
			exp := &PendingExport{
				Node:   node,
				Packet: packet,
			}
			err := router.SendPendingExport(exp)
			if err != nil {
				log.Printf("WARNING: multicast to IP %s failed with %#v. Moved to pending list.", node.Address, err)
				router.pendingExports = append(router.pendingExports, exp)
			}
		}
	}
	router.maybeResolveSnapshotRequests(packet)
}

// Return within 200 ms at most. Returns null when successful.
// Lock must be acquired outside this.
func (router *ChunkRouter) SendPendingExport(exp *PendingExport) error {
	ctx := context.Background()

	node := exp.Node
	packet := exp.Packet
	conn, err := grpc.Dial(fmt.Sprintf("%s:9000", node.Address),
		grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
	if err != nil {
		return err
	}
	defer conn.Close()

	chunkService := api.NewChunkServiceClient(conn)
	escapedGrainsProto := make(map[string]*api.GrainSet, len(packet.EscapedGrains))
	for k, v := range packet.EscapedGrains {
		escapedGrainsProto[k] = &api.GrainSet{Grains: v}
	}
	strictCtx, _ := context.WithTimeout(ctx, 250*time.Millisecond)
	_, err = chunkService.NotifyNeighbor(strictCtx, &api.NotifyNeighborQ{
		Packet: &api.NeighborExport{
			OriginChunkId: packet.OriginChunkId,
			DestChunkId:   node.ChunkId,
			Timestamp:     packet.Timestamp,
			ChunkGrains:   packet.ChunkGrains,
			EscapedGrains: escapedGrainsProto,
		},
	})
	if err != nil {
		return err
	}
	return nil
}
