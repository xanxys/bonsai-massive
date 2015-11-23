package main

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/cloud"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
)

const (
	ProjectId = "bonsai-genesis"
	zone      = "us-central1-b"
)

type FeServiceImpl struct {
	cred               *jwt.Config
	chunkContainerName string
}

func NewFeService() *FeServiceImpl {
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
		cred:               conf,
		chunkContainerName: string(cont),
	}
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

type BiosphereMeta struct {
	Name string
}

// Arbitrary code that needs to run continuously forever on this server.
func (fe *FeServiceImpl) StatefulLoop() {

}
