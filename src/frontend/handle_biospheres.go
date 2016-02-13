package main

import (
	"./api"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"log"
	"sort"
	"time"
)

const tickPerYear = 5000

func (fe *FeServiceImpl) Biospheres(ctx context.Context, q *api.BiospheresQ) (*api.BiospheresS, error) {
	stateReceiver := make(chan map[uint64]api.BiosphereState, 1)
	fe.cmdQueue <- &ControllerCommand{getBiosphereStates: stateReceiver}

	client, err := fe.AuthDatastore(ctx)
	if err != nil {
		return nil, err
	}
	dq := datastore.NewQuery("BiosphereMeta")

	var metas []*BiosphereMeta
	keys, err := client.GetAll(ctx, dq, &metas)
	if err != nil {
		return nil, err
	}

	chunkState := <-stateReceiver
	var bios []*api.BiosphereDesc
	for ix, meta := range metas {
		state, ok := chunkState[uint64(keys[ix].ID())]
		if !ok {
			state = api.BiosphereState_STOPPED
		}
		topo := NewCylinderTopology(uint64(keys[ix].ID()), int(meta.Nx), int(meta.Ny))
		chunkId := topo.GetChunkTopos()[0].ChunkId

		t0 := time.Now()
		query := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", chunkId)
		var ss []*PersistentChunkSnapshot
		_, err := client.GetAll(ctx, query, &ss)
		if err != nil {
			return nil, err
		}
		log.Printf("Naive query took %s for %s", time.Since(t0), chunkId)

		maxTimestamp := uint64(0)
		persistedYearsMap := make(map[int32]bool)
		for _, snapshot := range ss {
			if snapshot.Timestamp%tickPerYear == 0 {
				persistedYearsMap[int32(snapshot.Timestamp/tickPerYear)] = true
			}
			if uint64(snapshot.Timestamp) > maxTimestamp {
				maxTimestamp = uint64(snapshot.Timestamp)
			}
		}
		var persistedYears []int32
		for year := range persistedYearsMap {
			persistedYears = append(persistedYears, year)
		}
		sort.Sort(I32Slice(persistedYears))

		bios = append(bios, &api.BiosphereDesc{
			BiosphereId:    uint64(keys[ix].ID()),
			Name:           meta.Name,
			NumCores:       uint32(meta.Nx*meta.Ny/5) + 1,
			NumTicks:       maxTimestamp,
			State:          state,
			Nx:             meta.Nx,
			Ny:             meta.Ny,
			PersistedYears: persistedYears,
		})
	}
	return &api.BiospheresS{
		Biospheres: bios,
	}, nil
}

type I32Slice []int32

func (slice I32Slice) Len() int {
	return len(slice)
}

func (slice I32Slice) Less(i, j int) bool {
	return slice[i] < slice[j]
}

func (slice I32Slice) Swap(i, j int) {
	t := slice[i]
	slice[i] = slice[j]
	slice[j] = t
}
