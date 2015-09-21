package main

import (
	"./api"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
)

type FeServiceImpl struct {
}

func Auth() *datastore.Client {
	jsonKey, err := ioutil.ReadFile("/root/bonsai/key.json")
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.JWTConfigFromJSON(
		jsonKey,
		datastore.ScopeDatastore,
		datastore.ScopeUserEmail,
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "bonsai-genesis", cloud.WithTokenSource(conf.TokenSource(ctx)))
	if err != nil {
		log.Fatal(err)
	}
	// Use the client (see other examples).
	return client
}

type BiosphereMeta struct {
	Name string
}

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) *api.BiospheresS {
	name := "Hogehoge"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}

func (fe *FeServiceImpl) HandleBiosphereDelta(q *api.BiosphereDeltaQ) *api.BiospheresS {
	name := "FugaFuga"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	ctx := context.Background()
	client := Auth()
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	_, err := client.Put(ctx, key, &BiosphereMeta{
		Name: q.GetDesc().GetName(),
	})
	if err != nil {
		log.Fatal(err)
	}

	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}
