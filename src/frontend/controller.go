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
	Nx, Ny int
}

// Magically ensured (not yet) that only one instance of this code is always
// running in FE cluster. (staging & prod will have different ones.)
//
// Arbitrary code that needs to run continuously forever on this server.
func (fe *FeServiceImpl) StatefulLoop() {
	log.Println("Starting stateful loop")
	var targetState *ControllerCommand
	for {
		select {
		case cmd := <-fe.cmdQueue:
			log.Printf("Received controller command: %v", cmd)
			targetState = cmd
		case <-time.After(10 * time.Second):
			ctx := context.Background()
			fe.applyDelta(ctx, targetState)
		}
	}
}

// Modify chunk servers so that they will become targetState eventually.
// This function must ensure it completes within a few seconds at most.
//
// This function just ensures proper number of chunk servers is running.
// It's basically same as kubernetes replication controller, but GKE price model
// is not suitable for me, so I'll manage chunk servers here... for now.
func (fe *FeServiceImpl) applyDelta(ctx context.Context, targetState *ControllerCommand) {
	chunkInstances, err := fe.GetChunkServerInstances(ctx)
	if err != nil {
		log.Printf("Error while fetching instance list %v", err)
		return
	}
	if targetState != nil && len(chunkInstances) == 0 {
		log.Printf("Allocating 1 node")
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
			fe.applyChunkDelta(ctx, ip, chunkService, targetState)
		}
	}
}

// After confirming chunk sever is properly responding at ipAddress, try to
// match its state to targetState.
func (fe *FeServiceImpl) applyChunkDelta(ctx context.Context, ipAddress string, chunkService api.ChunkServiceClient, targetState *ControllerCommand) {
	summary, err := chunkService.ChunkSummary(ctx, &api.ChunkSummaryQ{})
	if err != nil {
		log.Printf("Supposed-to-be-alive failed to return ChunkSummaryQ with error %v", err)
		return
	}

	if len(summary.Chunks) != targetState.Nx*targetState.Ny {
		if len(summary.Chunks) == 0 {
			log.Printf("Spawning %d new chunks", targetState.Nx*targetState.Ny)
			for ix := 0; ix < targetState.Nx; ix++ {
				for iy := 0; iy < targetState.Ny; iy++ {
					chunkService.SpawnChunk(ctx, &api.SpawnChunkQ{
						Topology: &api.ChunkTopology{},
					})
				}
			}
			return
		} else {
			log.Printf("Some strange number (%d) of chunks found; probably some bug", len(summary.Chunks))
			return
		}
	}
}
