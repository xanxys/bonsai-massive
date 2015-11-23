package main

import (
	"./api"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
)

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) (*api.BiospheresS, error) {
	ctx := context.Background()

	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

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
	var bios []*api.BiosphereDesc
	for ix, meta := range metas {
		bios = append(bios, &api.BiosphereDesc{
			BiosphereId: uint64(keys[ix].ID()),
			Name:        meta.Name,
			NumCores:    nCores,
			NumTicks:    nTicks,
		})
	}
	return &api.BiospheresS{
		Biospheres: bios,
	}, nil
}
