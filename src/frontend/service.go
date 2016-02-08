package main

import (
	"./api"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
)

type FeServiceImpl struct {
	envType string

	*ServerCred
	chunkContainerName string
	cmdQueue           chan *ControllerCommand
}

func NewFeService(envType string) *FeServiceImpl {
	cont, err := ioutil.ReadFile("/root/bonsai/config.chunk-container")
	if err != nil {
		log.Fatal(err)
	}
	fe := &FeServiceImpl{
		envType:            envType,
		ServerCred:         NewServerCred(),
		chunkContainerName: string(cont),
	}
	// TODO: Ensure one loop is always running.
	fe.cmdQueue = make(chan *ControllerCommand, 50)
	go fe.StatefulLoop()
	return fe
}

func (fe *FeServiceImpl) getBiosphereTopo(ctx context.Context, biosphereId uint64) (BiosphereTopology, *api.BiosphereEnvConfig, error) {
	client, err := fe.AuthDatastore(ctx)
	if err != nil {
		return nil, nil, err
	}
	key := datastore.NewKey(ctx, "BiosphereMeta", "", int64(biosphereId), nil)
	var meta BiosphereMeta
	err = client.Get(ctx, key, &meta)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("Found config of %d: %d x %d", key.ID(), meta.Nx, meta.Ny)
	envConfig := api.BiosphereEnvConfig{}
	err = proto.Unmarshal(meta.Env, &envConfig)
	if err != nil {
		return nil, nil, err
	}
	return NewCylinderTopology(biosphereId, int(meta.Nx), int(meta.Ny)), &envConfig, nil
}
