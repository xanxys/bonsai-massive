package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

// Issue-and-forget type of commands.
type ControllerCommand struct {
	// Start new biosphere.
	bsId   uint64
	bsTopo BiosphereTopology
	env    *api.BiosphereEnvConfig

	// Query managed biospheres and their states.
	// This is a few seconds old. (depending on polling interval)
	getBiosphereStates chan map[uint64]api.BiosphereState
}

const chunkIdFormat = "%d-%d:%d"

// Magically ensured (not yet) that only one instance of this code is always
// running in FE cluster. (staging & prod will have different ones.)
//
// Arbitrary code that needs to run continuously forever on this server.
func (fe *FeServiceImpl) StatefulLoop() {
	log.Println("Starting stateful loop")
	var targetState *ControllerCommand
	latestState := make(map[uint64]api.BiosphereState)
	infTicks := time.Tick(10 * time.Second)
	for {
		select {
		case cmd := <-fe.cmdQueue:
			if cmd == nil {
				log.Printf("Received nil command")
				targetState = nil
				latestState = make(map[uint64]api.BiosphereState)
			} else if cmd.getBiosphereStates != nil {
				log.Printf("Received getBiosphereStates")
				frozenState := make(map[uint64]api.BiosphereState)
				for k, v := range latestState {
					frozenState[k] = v
				}
				cmd.getBiosphereStates <- frozenState
			} else {
				log.Printf("Received controller command: %v", cmd)
				targetState = cmd
				latestState[cmd.bsId] = api.BiosphereState_T_RUN
			}
		case <-infTicks:
			ctx := context.Background()
			fe.applyDelta(ctx, latestState, targetState)
		}
	}
}

// Modify chunk servers so that they will become targetState eventually.
// This function must ensure it completes within a few seconds at most.
//
// This function just ensures proper number of chunk servers is running.
// It's basically same as kubernetes replication controller, but GKE price model
// is not suitable for me, so I'll manage chunk servers here... for now.
func (fe *FeServiceImpl) applyDelta(ctx context.Context, latestState map[uint64]api.BiosphereState, targetState *ControllerCommand) {
	chunkInstances, err := fe.GetChunkServerInstances(ctx)
	if err != nil {
		log.Printf("Error while fetching instance list %v", err)
		return
	}
	if targetState != nil && len(chunkInstances) == 0 {
		log.Printf("Allocating 1 node")
		latestState[targetState.bsId] = api.BiosphereState_T_RUN
		clientCompute, err := fe.authCompute(ctx)
		if err != nil {
			log.Printf("Error in allocation: %v", err)
			return
		}
		fe.prepare(clientCompute)
	} else if targetState != nil && len(chunkInstances) > 0 {
		for _, instance := range chunkInstances {
			ip := instance.NetworkInterfaces[0].NetworkIP
			conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
				grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
			if err != nil {
				// Server not ready yet. This is expected, so don't do anything and just wait for next cycle.
				return
			}
			defer conn.Close()
			chunkService := api.NewChunkServiceClient(conn)
			_, err = chunkService.Status(ctx, &api.StatusQ{})
			if err != nil {
				// Server not ready yet. This is expected, so don't do anything and just wait for next cycle.
				return
			}
			fe.applyChunkDelta(ctx, ip, chunkService, latestState, targetState)
		}
	} else if targetState == nil && len(chunkInstances) > 0 {
		log.Printf("Deallocating %d nodes", len(chunkInstances))
		clientCompute, err := fe.authCompute(ctx)
		if err != nil {
			log.Printf("Error in compute auth: %v", err)
			return
		}
		names := make([]string, len(chunkInstances))
		for ix, chunkInstance := range chunkInstances {
			names[ix] = chunkInstance.Name
		}
		fe.deleteInstances(clientCompute, names)
	}
}

// After confirming chunk sever is properly responding at ipAddress, try to
// match its state to targetState.
func (fe *FeServiceImpl) applyChunkDelta(ctx context.Context, ipAddress string, chunkService api.ChunkServiceClient, latestState map[uint64]api.BiosphereState, targetState *ControllerCommand) {
	summary, err := chunkService.ChunkSummary(ctx, &api.ChunkSummaryQ{})
	if err != nil {
		log.Printf("Supposed-to-be-alive failed to return ChunkSummaryQ with error %v", err)
		return
	}

	if len(summary.Chunks) != len(targetState.bsTopo.GetChunkTopos()) {
		if len(summary.Chunks) == 0 {
			chunkGens := GenerateEnv(targetState.bsTopo, targetState.env)
			log.Printf("Spawning %d new chunks: %#v", len(chunkGens), chunkGens)
			for _, chunkGen := range chunkGens {
				chunkService.SpawnChunk(ctx, chunkGen)
			}
			return
		} else {
			log.Printf("Some strange number (%d) of chunks found; probably some bug", len(summary.Chunks))
			return
		}
	} else {
		latestState[targetState.bsId] = api.BiosphereState_RUNNING
	}
}
