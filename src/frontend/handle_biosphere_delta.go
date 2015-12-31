package main

import (
	"./api"
	"errors"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
)

func (fe *FeServiceImpl) BiosphereDelta(ctx context.Context, q *api.BiosphereDeltaQ) (*api.BiospheresS, error) {
	if q.Auth == nil {
		return nil, errors.New("BiosphereDelta requires auth")
	}
	canWrite, err := fe.isWriteAuthorized(q.Auth)
	if err != nil {
		return nil, err
	}
	if canWrite {
		return nil, errors.New("UI must disallow unauthorized actions")
	}

	var nTicks uint64
	nTicks = 38

	client, err := fe.authDatastore(ctx)
	if err != nil {
		return nil, err
	}
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	// TODO: check collision with existing name / empty names etc.
	meta := &BiosphereMeta{
		Name: q.GetCreationConfig().Name,
		Nx:   q.GetCreationConfig().Nx,
		Ny:   q.GetCreationConfig().Ny,
	}
	key, err = client.Put(ctx, key, meta)
	if err != nil {
		return nil, err
	}

	clientCompute, err := fe.authCompute(ctx)
	if err != nil {
		return nil, err
	}
	fe.prepare(clientCompute)

	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				BiosphereId: uint64(key.ID()),
				Name:        meta.Name,
				NumCores:    uint32(meta.Nx*meta.Ny/5) + 1,
				NumTicks:    nTicks,
			},
		},
	}, nil
}
