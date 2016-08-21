package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) GetChunk(ctx context.Context, q *api.GetChunkQ) (*api.GetChunkS, error) {
	cState := ck.Get(q.CacheKey)
	return &api.GetChunkS{
		Success: cState != nil,
		Content: cState,
	}, nil
}
