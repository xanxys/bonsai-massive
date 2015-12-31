package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) SpawnChunk(ctx context.Context, q *api.SpawnChunkQ) (*api.SpawnChunkS, error) {
	return &api.SpawnChunkS{}, nil
}
