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

func Auth() (context.Context, *datastore.Client) {
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
	return ctx, client
}

type BiosphereMeta struct {
	Name string
}

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) (*api.BiospheresS, error) {
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	ctx, client := Auth()
	dq := datastore.NewQuery("BiosphereMeta")

	var metas []*BiosphereMeta
	_, err := client.GetAll(ctx, dq, &metas)
	if err != nil {
		return nil, err
	}
	var bios []*api.BiosphereDesc
	for _, meta := range metas {
		bios = append(bios, &api.BiosphereDesc{
			Name:     &meta.Name,
			NumCores: &nCores,
			NumTicks: &nTicks,
		})
	}
	return &api.BiospheresS{
		Biospheres: bios,
	}, nil
}

func (fe *FeServiceImpl) HandleBiosphereDelta(q *api.BiosphereDeltaQ) (*api.BiospheresS, error) {
	name := "FugaFuga"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	ctx, client := Auth()
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	_, err := client.Put(ctx, key, &BiosphereMeta{
		Name: q.GetDesc().GetName(),
	})
	if err != nil {
		return nil, err
	}

	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}, nil
}
