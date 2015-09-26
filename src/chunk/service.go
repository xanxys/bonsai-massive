package main

import (
	"./api"
	"golang.org/x/net/context"
)

type CkServiceImpl struct {
}

func (ck *CkServiceImpl) Test(ctx context.Context, q *api.TestQ) (*api.TestS, error) {
	return &api.TestS{}, nil
}
