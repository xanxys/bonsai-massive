package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

type BiosphereTopology interface {
	GetChunkTopos() []*api.ChunkTopology
}

// Issue-and-forget type of commands.
type ControllerCommand struct {
	// Start new biosphere.
	bsTopo BiosphereTopology
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
func (fe *FeServiceImpl) applyChunkDelta(ctx context.Context, ipAddress string, chunkService api.ChunkServiceClient, targetState *ControllerCommand) {
	summary, err := chunkService.ChunkSummary(ctx, &api.ChunkSummaryQ{})
	if err != nil {
		log.Printf("Supposed-to-be-alive failed to return ChunkSummaryQ with error %v", err)
		return
	}

	if len(summary.Chunks) != len(targetState.bsTopo.GetChunkTopos()) {
		if len(summary.Chunks) == 0 {
			topos := targetState.bsTopo.GetChunkTopos()
			log.Printf("Spawning %d new chunks", len(topos), topos)
			for _, topo := range topos {
				chunkService.SpawnChunk(ctx, &api.SpawnChunkQ{
					Topology: topo,
				})
			}
			return
		} else {
			log.Printf("Some strange number (%d) of chunks found; probably some bug", len(summary.Chunks))
			return
		}
	}
}

// Edge X=0, nx is connected with each other at same Y,
// Y edges (0, ny) is walled.
type CylinderTopology struct {
	Nx, Ny int
	bsId   uint64
}

func NewCylinderTopology(bsId uint64, nx, ny int) *CylinderTopology {
	return &CylinderTopology{
		Nx:   nx,
		Ny:   ny,
		bsId: bsId,
	}
}

func (cylinder *CylinderTopology) GetChunkTopos() []*api.ChunkTopology {
	const idFormat = "%d-%d:%d"

	var result []*api.ChunkTopology
	for ix := 0; ix < cylinder.Nx; ix++ {
		for iy := 0; iy < cylinder.Ny; iy++ {
			topo := &api.ChunkTopology{
				ChunkId: fmt.Sprintf(idFormat, cylinder.bsId, ix, iy),
			}
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					if dx == 0 && dy == 0 {
						continue
					}
					neighborIx := (ix + dx) % cylinder.Nx
					if neighborIx < 0 {
						neighborIx += cylinder.Nx
					}
					neighborIy := iy + dy
					if neighborIy < 0 || neighborIy >= cylinder.Ny {
						continue
					}
					topo.Neighbors = append(topo.Neighbors, &api.ChunkTopology_ChunkNeighbor{
						ChunkId:  fmt.Sprintf(idFormat, cylinder.bsId, neighborIx, neighborIy),
						Internal: true,
						Dx:       int32(dx),
						Dy:       int32(dy),
					})
				}
			}
			result = append(result, topo)
		}
	}
	return result
}
