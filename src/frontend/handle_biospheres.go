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
	ctx = TraceStart(ctx, "/frontend.Biospheres")
	defer TraceEnd(ctx, fe.ServerCred)

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

	var bios []*api.BiosphereDesc
	for ix, meta := range metas {
		bsId := uint64(keys[ix].ID())
		bsState := fe.controller.GetBiosphereState(bsId)
		stateProto := api.BiosphereState_UNKNOWN
		if bsState.flag == Stopped {
			stateProto = api.BiosphereState_STOPPED
		} else if bsState.flag == Waiting {
			stateProto = api.BiosphereState_T_RUN
		} else if bsState.flag == Running {
			stateProto = api.BiosphereState_RUNNING
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
			BiosphereId:    bsId,
			Name:           meta.Name,
			NumCores:       uint32(meta.Nx*meta.Ny/5) + 1,
			NumTicks:       maxTimestamp,
			State:          stateProto,
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
