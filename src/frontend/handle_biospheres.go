package main

import (
	"./api"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
)

func (fe *FeServiceImpl) Biospheres(ctx context.Context, q *api.BiospheresQ) (*api.BiospheresS, error) {
	var nTicks uint64
	nTicks = 38

	stateReceiver := make(chan map[uint64]api.BiosphereState, 1)
	fe.cmdQueue <- &ControllerCommand{getBiosphereStates: stateReceiver}

	client, err := fe.authDatastore(ctx)
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
		bios = append(bios, &api.BiosphereDesc{
			BiosphereId: uint64(keys[ix].ID()),
			Name:        meta.Name,
			NumCores:    uint32(meta.Nx*meta.Ny/5) + 1,
			NumTicks:    nTicks,
			State:       state,
			Nx:          meta.Nx,
			Ny:          meta.Ny,
		})
	}
	return &api.BiospheresS{
		Biospheres: bios,
	}, nil
}
