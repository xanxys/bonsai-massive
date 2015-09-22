package main

import (
	"./api"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/cloud"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
)

type FeServiceImpl struct {
	datastoreCred *jwt.Config
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
	)
	if err != nil {
		log.Fatal(err)
	}
	return &FeServiceImpl{
		datastoreCred: conf,
	}
}

func (fe *FeServiceImpl) auth(ctx context.Context) (*datastore.Client, error) {
	client, err := datastore.NewClient(
		ctx, "bonsai-genesis", cloud.WithTokenSource(fe.datastoreCred.TokenSource(ctx)))
	if err != nil {
		return nil, err
	}
	return client, nil
}

type BiosphereMeta struct {
	Name string
}

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) (*api.BiospheresS, error) {
	ctx := context.Background()

	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	client, err := fe.auth(ctx)
	if err != nil {
		return nil, err
	}
	dq := datastore.NewQuery("BiosphereMeta")

	var metas []*BiosphereMeta
	_, err = client.GetAll(ctx, dq, &metas)
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
	ctx := context.Background()

	name := "FugaFuga"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	client, err := fe.auth(ctx)
	if err != nil {
		return nil, err
	}
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	_, err = client.Put(ctx, key, &BiosphereMeta{
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
