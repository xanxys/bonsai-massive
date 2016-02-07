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

type ServerCred struct {
	cred *jwt.Config
}

func NewServerCred() *ServerCred {
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
	return &ServerCred{
		cred: conf,
	}
}

func (cred *ServerCred) AuthDatastore(ctx context.Context) (*datastore.Client, error) {
	client, err := datastore.NewClient(
		ctx, ProjectId, cloud.WithTokenSource(cred.cred.TokenSource(ctx)))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (cred *ServerCred) AuthCompute(ctx context.Context) (*compute.Service, error) {
	client := cred.cred.Client(oauth2.NoContext)
	service, err := compute.New(client)
	return service, err
}
