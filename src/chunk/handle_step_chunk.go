package main

import (
	"./api"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"log"
	"sync"
	"time"
)

func (ck *CkServiceImpl) StepChunk(ctx context.Context, q *api.StepChunkQ) (*api.StepChunkS, error) {
	// Validate.
	if len(q.ChunkInput) == 0 || len(q.ChunkInput) > 9 {
		log.Printf("Invalid number of ChunkInput. inputs=%# v", pretty.Formatter(q.ChunkInput))
		return nil, grpc.Errorf(codes.InvalidArgument, "")
	}

	inputSnapshots := ck.fetchAllInput(q.ChunkInput)
	if inputSnapshots == nil {
		log.Printf("ERROR Some input fetch failed for input: %#v", q.ChunkInput)
		return &api.StepChunkS{
			Success: false,
		}, nil
	}

	chunk := NewGrainChunk(false)
	selfGrains, env := mergeAndPartition(inputSnapshots)
	chunk.Grains = selfGrains
	outgoing := chunk.Step(env, convertToWall(inputSnapshots))

	selfShard := &api.ChunkShard{Dp: &api.ChunkRel{Dx: 0, Dy: 0}, Grains: make([]*api.Grain, len(chunk.Grains))}
	for ix, grain := range chunk.Grains {
		selfShard.Grains[ix] = ser(grain)
	}
	newSnapshot := &api.ChunkState{
		Shards: append(distribute(outgoing), selfShard),
	}

	cacheKey := ck.Add(newSnapshot)
	return &api.StepChunkS{
		Success:  true,
		CacheKey: cacheKey,
	}, nil
}

// Return (self, env).
// Note that env is incomplete because:
// 1. it doesn't contain incoming grains from far away
//   e.g. (0, 1) won't contain outgoing grains from (0, 2).
// 2. it will drop outgoing grains (e.g. (0, 1) -> (0, 2))
//   this implies env grains will fit in [-1,1] * [-1,1]
func mergeAndPartition(states map[api.ChunkRel]*api.ChunkState) ([]*Grain, []*Grain) {
	var grainsSelf []*Grain
	var grainsEnv []*Grain

	for stDp, st := range states {
		for _, shard := range st.Shards {
			dx := int(stDp.Dx + shard.Dp.Dx)
			dy := int(stDp.Dy + shard.Dp.Dy)
			grainsDelta := make([]*Grain, len(shard.Grains))
			for ix, grain := range shard.Grains {
				grain := deser(grain)
				grain.Position.X += float32(dx)
				grain.Position.Y += float32(dy)
				grainsDelta[ix] = grain
			}
			if dx == 0 && dy == 0 {
				grainsSelf = append(grainsSelf, grainsDelta...)
			} else if iabs(dx) <= 1 && iabs(dy) <= 1 {
				grainsEnv = append(grainsSelf, grainsDelta...)
			}
		}
	}
	return grainsSelf, grainsEnv
}

// Bin outgoing grains into shards.
func distribute(outGrains []*Grain) []*api.ChunkShard {
	bins := make(map[api.ChunkRel][]*api.Grain)
	for _, grain := range outGrains {
		dp := api.ChunkRel{int32(ifloor(grain.Position.X)), int32(ifloor(grain.Position.Y))}
		grainProto := ser(grain)
		grainProto.Pos.X -= float32(dp.Dx)
		grainProto.Pos.Y -= float32(dp.Dy)
		bins[dp] = append(bins[dp], grainProto)
	}

	var shards []*api.ChunkShard
	for dp, grains := range bins {
		if iabs(int(dp.Dx))+iabs(int(dp.Dy)) == 0 || iabs(int(dp.Dx)) > 1 || iabs(int(dp.Dy)) > 1 {
			log.Printf("WARNING %d outgoing grains are contained in %v. Dropping", len(grains), dp)
			continue
		}
		shards = append(shards, &api.ChunkShard{Dp: &dp, Grains: grains})
	}
	return shards
}

func convertToWall(states map[api.ChunkRel]*api.ChunkState) *ChunkWall {
	return &ChunkWall{
		Xp: states[api.ChunkRel{1, 0}] != nil,
		Xm: states[api.ChunkRel{-1, 0}] != nil,
		Yp: states[api.ChunkRel{0, 1}] != nil,
		Ym: states[api.ChunkRel{0, -1}] != nil,
	}
}

// Currently returns all input snapshots.
// Returns nil if any pf the fetch failed.
func (ck *CkServiceImpl) fetchAllInput(chunkInputs []*api.StepChunkQ_Input) map[api.ChunkRel]*api.ChunkState {
	var wg sync.WaitGroup
	states := make(map[api.ChunkRel]*api.ChunkState)
	for _, chunkInput := range chunkInputs {
		switch inputType := chunkInput.Data.Location.(type) {
		case *api.ChunkDataLocator_SelfCacheKey:
			states[*chunkInput.Dp] = ck.Get(inputType.SelfCacheKey)
		case *api.ChunkDataLocator_RemoteCacheKey:
			wg.Add(1)
			go func() {
				defer wg.Done()
				states[*chunkInput.Dp] = ck.fetchRemoteCache(inputType.RemoteCacheKey)
			}()
		case *api.ChunkDataLocator_DatastoreKey:
			wg.Add(1)
			go func() {
				defer wg.Done()
				states[*chunkInput.Dp] = ck.fetchDatastoreSnapshot(inputType.DatastoreKey)
			}()
		default:
			log.Printf("ERROR: Unknown ChunkInput type %v at rel %v", inputType, chunkInput.Dp)
		}
	}

	wg.Wait()
	for _, state := range states {
		if state == nil {
			return nil
		}
	}
	return states
}

func (ck *CkServiceImpl) fetchRemoteCache(remoteKey *api.RemoteChunkCache) *api.ChunkState {
	svcIpPort := fmt.Sprintf("%s:9000", remoteKey.Ip)
	conn, err := grpc.Dial(svcIpPort, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
	if err != nil {
		log.Printf("ERROR Failed to connect to %s", svcIpPort)
		return nil
	}
	defer conn.Close()
	service := api.NewChunkServiceClient(conn)
	strictCtx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	s, err := service.GetChunk(strictCtx, &api.GetChunkQ{CacheKey: remoteKey.CacheKey})
	if err != nil {
		log.Printf("ERROR: GetChunk@%s(%d) failed with %v", remoteKey.Ip, remoteKey.CacheKey, err)
		return nil
	}
	if !s.Success {
		log.Printf("ERROR: %d@%s was unavailable", remoteKey.Ip, remoteKey.CacheKey)
		return nil
	}
	return s.Content
}

func (ck *CkServiceImpl) fetchDatastoreSnapshot(dsKey int64) *api.ChunkState {
	strictCtx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	client, err := ck.AuthDatastore(strictCtx)
	if err != nil {
		log.Printf("ERROR datastore auth failed %#v", err)
		return nil
	}

	key := datastore.NewKey(strictCtx, "PersistentChunkSnapshot", "", dsKey, nil)
	snapshot := new(PersistentChunkSnapshot)
	err = client.Get(strictCtx, key, snapshot)
	if err != nil {
		log.Printf("ERROR datastore.Get(%#v) failed %#v", key, err)
	}

	snapshotProto := &api.ChunkSnapshot{}
	err = proto.Unmarshal(snapshot.Snapshot, snapshotProto)
	if err != nil {
		log.Printf("ERROR failed to unmarshal; corrupt datastore entry at key %d %#v", dsKey, err)
		return nil
	}

	// Convert to a shard w/o any outgoing grains.
	return &api.ChunkState{
		Shards: []*api.ChunkShard{
			&api.ChunkShard{
				Dp:     &api.ChunkRel{Dx: 0, Dy: 0},
				Grains: snapshotProto.Grains,
			},
		},
	}
}
