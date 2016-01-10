package main

import (
	"./api"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/cloud"
	"google.golang.org/cloud/datastore"
	"google.golang.org/grpc"
	"io/ioutil"
	"log"
)

const (
	ProjectId = "bonsai-genesis"
	zone      = "us-central1-b"
)

type FeServiceImpl struct {
	envType string

	cred               *jwt.Config
	chunkContainerName string
	cmdQueue           chan *ControllerCommand

	// TODO: This is not thread safe! Protect it with a lock!
	// TODO: this should be updated by controller.
	chunkServices map[string]*grpc.ClientConn
}

func NewFeService(envType string) *FeServiceImpl {
	jsonKey, err := ioutil.ReadFile("/root/bonsai/key.json")
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.JWTConfigFromJSON(
		jsonKey,
		datastore.ScopeDatastore,
		datastore.ScopeUserEmail,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/compute")
	if err != nil {
		log.Fatal(err)
	}
	cont, err := ioutil.ReadFile("/root/bonsai/config.chunk-container")
	if err != nil {
		log.Fatal(err)
	}
	fe := &FeServiceImpl{
		envType:            envType,
		cred:               conf,
		chunkContainerName: string(cont),
		chunkServices:      make(map[string]*grpc.ClientConn),
	}
	// TODO: Ensure one loop is always running.
	fe.cmdQueue = make(chan *ControllerCommand, 50)
	go fe.StatefulLoop()
	return fe
}

func (fe *FeServiceImpl) authDatastore(ctx context.Context) (*datastore.Client, error) {
	client, err := datastore.NewClient(
		ctx, ProjectId, cloud.WithTokenSource(fe.cred.TokenSource(ctx)))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (fe *FeServiceImpl) authCompute(ctx context.Context) (*compute.Service, error) {
	client := fe.cred.Client(oauth2.NoContext)
	service, err := compute.New(client)
	return service, err
}

func (fe *FeServiceImpl) getBiosphereTopo(ctx context.Context, biosphereId uint64) (BiosphereTopology, *api.BiosphereEnvConfig, error) {
	client, err := fe.authDatastore(ctx)
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

type BiosphereMeta struct {
	Name string
	Nx   int32
	Ny   int32
	// Serialized api.BiosphereEnvConfig.
	Env []byte
}
