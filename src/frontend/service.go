package main

import (
	"./api"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"log"
	"sync"
)

type FeServiceImpl struct {
	envType string

	*ServerCred

	controller *Controller

	bsMetaCacheMutex sync.Mutex
	bsMetaCache      map[uint64]*BiosphereMeta
}

func NewFeService(envType string) *FeServiceImpl {
	fe := &FeServiceImpl{
		envType:     envType,
		ServerCred:  NewServerCred(),
		bsMetaCache: make(map[uint64]*BiosphereMeta),
	}
	// TODO: Ensure one loop is always running when fe crashes.
	fe.controller = NewController(fe)
	return fe
}

func (fe *FeServiceImpl) getBiosphereTopo(ctx context.Context, biosphereId uint64) (BiosphereTopology, *api.BiosphereEnvConfig, error, *api.TimingTrace) {
	trace := InitTrace("getBiosphereTopo")
	fe.bsMetaCacheMutex.Lock()
	defer fe.bsMetaCacheMutex.Unlock()

	bsMeta, ok := fe.bsMetaCache[biosphereId]
	if !ok {
		authTrace := InitTrace("AuthDatastore")
		client, err := fe.AuthDatastore(ctx)
		if err != nil {
			return nil, nil, err, trace
		}
		FinishTrace(authTrace, trace)

		key := datastore.NewKey(ctx, "BiosphereMeta", "", int64(biosphereId), nil)
		var meta BiosphereMeta
		err = client.Get(ctx, key, &meta)
		if err != nil {
			return nil, nil, err, trace
		}
		fe.bsMetaCache[biosphereId] = &meta
		bsMeta = &meta
		log.Printf("BiosphereMeta cache entry (bsId: %d) created. Now #(cache entry)=%d", biosphereId, len(fe.bsMetaCache))
	}
	log.Printf("Found config of %d x %d", bsMeta.Nx, bsMeta.Ny)
	envConfig := api.BiosphereEnvConfig{}
	err := proto.Unmarshal(bsMeta.Env, &envConfig)
	if err != nil {
		return nil, nil, err, trace
	}
	return NewCylinderTopology(biosphereId, int(bsMeta.Nx), int(bsMeta.Ny)), &envConfig, nil, trace
}
