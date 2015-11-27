package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) ModifyChunk(ctx context.Context, q *api.ModifyChunkQ) (*api.ModifyChunkS, error) {
	ck.chunkCommand <- q
	return &api.ModifyChunkS{}, nil
}
