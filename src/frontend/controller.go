package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"google.golang.org/grpc"
	"log"
	"strconv"
	"strings"
	"time"
)

func NewController(fe *FeServiceImpl) *Controller {
	ctrl := &Controller{
		fe:          fe,
		targetState: make(map[uint64]TargetState),
	}
	ctrl.pool = NewPoolController(fe, ctrl)
	return ctrl
}

func (ctrl *Controller) PostChange() {
	ctrl.Reallocate()
}

// All methods are thread-safe, and are guranteed to return within 150ms.
// (one RPC w/ chunk servers or nothing).
type Controller struct {
	fe   *FeServiceImpl
	pool *PoolController
	// BiosphereId -> TargetState
	targetState map[uint64]TargetState
}

/*
BiosphereState
   = Stopped
   // Waiting for resource allocation
   | Waiting
   // ChunkId -> IP address
   | Running map[string]string
*/
type BiosphereStateFlag int

const (
	Stopped BiosphereStateFlag = iota
	Waiting
	Running
)

type TargetState struct {
	BsTopo BiosphereTopology
	Env    *api.BiosphereEnvConfig
	Slow   bool
}

type BiosphereState struct {
	flag BiosphereStateFlag
	// ChunkId -> IP address (only available when flag == Running)
	chunks map[string]string
}

func (ctrl *Controller) GetDebug() *api.ControllerDebug {
	m := make(map[uint64]*api.ControllerDebug_BiosphereState)
	for bsId, state := range ctrl.GetCurrentState() {
		m[bsId] = &api.ControllerDebug_BiosphereState{
			Flag:   api.ControllerDebug_BiosphereFlag(state.flag),
			Chunks: state.chunks,
		}
	}
	return &api.ControllerDebug{
		Biospheres: m,
	}
}

func (ctrl *Controller) GetBiosphereState(biosphereId uint64) BiosphereState {
	state, ok := ctrl.GetCurrentState()[biosphereId]
	if ok {
		return state
	} else {
		return BiosphereState{flag: Stopped}
	}
}

// TargetState != nil: Make it Running with specified parameters.
// TargetState == nil: Stop it.
//
// retval: target state is already achieved.
func (ctrl *Controller) SetBiosphereState(biosphereId uint64, targetState *TargetState) bool {
	if targetState != nil {
		ctrl.targetState[biosphereId] = *targetState
	} else {
		delete(ctrl.targetState, biosphereId)
	}
	ctrl.resetCoreTarget()
	if targetState != nil {
		ips := ctrl.pool.GetUsableIp()
		if len(ips) > 0 {
			ctrl.Reallocate()
			return true
		} else {
			return false
		}
	} else {
		ctrl.Reallocate()
		return true
	}
}

func (ctrl *Controller) resetCoreTarget() {
	const coresPerChunk = 0.5
	numCores := float64(0)
	for _, ts := range ctrl.targetState {
		numCores += float64(len(ts.BsTopo.GetChunkTopos())) * coresPerChunk
	}
	ctrl.pool.SetTargetCores(numCores)
}

func (ctrl *Controller) GetCurrentState() map[uint64]BiosphereState {
	ctx := context.Background()

	biospheres := make(map[uint64]BiosphereState)
	for _, ip := range ctrl.pool.GetUsableIp() {
		conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
			grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
		if err != nil {
			log.Printf("Couldn't connect to supposedly grpc-ok chunk server %s", ip)
			continue
		}
		defer conn.Close()
		service := api.NewChunkServiceClient(conn)
		s, err := service.ChunkSummary(ctx, &api.ChunkSummaryQ{})
		if err != nil {
			log.Printf("ChunkSummary@%s failed with %v", ip, err)
			continue
		}
		for _, chunk := range s.Chunks {
			biosphereId, err := strconv.ParseUint(strings.Split(chunk.ChunkId, "-")[0], 10, 64)
			if err != nil {
				log.Panicf("Unexpected chunkId %s found! (expecting <uint64 biosphere id>-<chunk descriptor>)", chunk.ChunkId)
				continue
			}
			_, ok := biospheres[biosphereId]
			if !ok {
				biospheres[biosphereId] = BiosphereState{
					flag:   Running,
					chunks: make(map[string]string),
				}
			}
			biospheres[biosphereId].chunks[chunk.ChunkId] = ip
		}
	}
	// Upgrade some of Stopped to Waiting.
	for bsId, _ := range ctrl.targetState {
		state, ok := biospheres[bsId]
		if !(ok && state.flag == Running) {
			biospheres[bsId] = BiosphereState{flag: Waiting}
		}
	}
	return biospheres
}

// Reallocate all chunks optimally. After this method, all Waiting will become Running.
func (ctrl *Controller) Reallocate() {
	ctx := context.Background()
	biospheres := ctrl.GetCurrentState()
	log.Printf("Reallocating chunks: current biospherse=%#v, target=%#v", biospheres, ctrl.targetState)
	// TODO: take snapshots of running biospheres instead of starting from persistent snapshot

	ips := ctrl.pool.GetUsableIp()
	if len(ips) == 0 {
		log.Print("ERROR: Chunk Reallocate requested, but doing nothing because 0 usable ips found. Probably a bug.")
		return
	}

	// Turn down all biospheres.
	for _, ip := range ips {
		conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
			grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
		if err != nil {
			log.Printf("ERROR: Unusable (%v) IP %s returned from GetUsableIp", err, ip)
			return
		}
		defer conn.Close()
		chunkService := api.NewChunkServiceClient(conn)
		csummary, err := chunkService.ChunkSummary(ctx, &api.ChunkSummaryQ{})
		if err != nil {
			log.Printf("ERROR: Unable to get chunk summary from IP %s: %v", ip, err)
			return
		}
		chunkIds := make([]string, len(csummary.Chunks))
		for ix, chunkTopo := range csummary.Chunks {
			chunkIds[ix] = chunkTopo.ChunkId
		}
		log.Printf("deleting chunks@ %s: %#v", ip, chunkIds)
		chunkService.DeleteChunk(ctx, &api.DeleteChunkQ{ChunkId: chunkIds})
	}

	// Calculate IP <-> chunk correspondence.
	client, err := ctrl.fe.AuthDatastore(ctx)
	if err != nil {
		log.Printf("ERROR: Datastore failed with %#v", err)
		return
	}

	numChunks := 0
	for _, ts := range ctrl.targetState {
		numChunks += len(GenerateEnv(ts.BsTopo, ts.Env))
	}
	chunkIndex := 0
	chunkLocation := make(map[string]string)            // ChunkId -> IP address
	serverChunks := make(map[string][]*api.SpawnChunkQ) // IP address -> [ChunkGen]
	for _, ts := range ctrl.targetState {
		// TODO: This is slow. Do something.
		firstChunkId := GenerateEnv(ts.BsTopo, ts.Env)[0].Topology.ChunkId
		query := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", firstChunkId)
		var ss []*PersistentChunkSnapshot
		_, err := client.GetAll(ctx, query, &ss)
		if err != nil {
			log.Printf("ERROR: failed to retrieve max timestamp %v", err)
			return
		}
		maxTimestamp := uint64(0)
		for _, snapshot := range ss {
			if uint64(snapshot.Timestamp) > maxTimestamp {
				maxTimestamp = uint64(snapshot.Timestamp)
			}
		}

		for _, genReq := range GenerateEnv(ts.BsTopo, ts.Env) {
			serverIndex := (chunkIndex * len(ips)) / numChunks
			serverIp := ips[serverIndex]
			genReq.SnapshotModulo = 5000
			genReq.RecordModulo = 100
			genReq.StartTimestamp = maxTimestamp
			if ts.Slow {
				genReq.FrameWaitNs = uint32(200 * time.Millisecond)
			} else {
				genReq.FrameWaitNs = 0
			}
			chunkLocation[genReq.Topology.ChunkId] = serverIp
			serverChunks[serverIp] = append(serverChunks[serverIp], genReq)
			chunkIndex++
		}
	}
	log.Printf("Assignment: %#v %#v", chunkLocation, serverChunks)

	// Respawn biospheres.
	for _, ip := range ips {
		conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
			grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
		if err != nil {
			log.Printf("ERROR: Unusable (%v) IP %s returned from GetUsableIp", err, ip)
			return
		}
		defer conn.Close()
		chunkService := api.NewChunkServiceClient(conn)

		for _, genReq := range serverChunks[ip] {
			for _, neighbor := range genReq.Topology.Neighbors {
				neighborIp := chunkLocation[neighbor.ChunkId]
				if neighborIp == ip {
					neighbor.Internal = true
				} else {
					neighbor.Internal = false
					neighbor.Address = neighborIp
				}
			}
			chunkService.SpawnChunk(ctx, genReq)
		}
	}
	log.Printf("Reallocate complete!")
}

const chunkIdFormat = "%d-%d:%d"
