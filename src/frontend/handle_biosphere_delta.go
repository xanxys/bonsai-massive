package main

import (
	"./api"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"net/http"
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

	name := "FugaFuga"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	client, err := fe.authDatastore(ctx)
	if err != nil {
		return nil, err
	}
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	// TODO: check collision with existing name / empty names etc.
	_, err = client.Put(ctx, key, &BiosphereMeta{
		Name: q.GetDesc().Name,
	})
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
				Name:     name,
				NumCores: nCores,
				NumTicks: nTicks,
			},
		},
	}, nil
}

type googleOAuth2V3Resp struct {
	email string
}

// Decided if the user is allowed to do write operations.
func (fe *FeServiceImpl) isWriteAuthorized(auth *api.UserAuth) (bool, error) {
	resp, err := http.Get(fmt.Sprintf("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=%s", auth.IdToken))
	if err != nil {
		return false, errors.New("Auth server failed")
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var m googleOAuth2V3Resp
	err = decoder.Decode(&m)
	if err != nil {
		return false, err
	}

	if m.email != "xanxys@gmail.com" {
		return false, nil
	}

	return true, nil
}
