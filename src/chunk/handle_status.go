package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) Status(ctx context.Context, q *api.StatusQ) (*api.StatusS, error) {
	return &api.StatusS{}, nil
}
