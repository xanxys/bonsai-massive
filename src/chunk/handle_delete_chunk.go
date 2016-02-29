package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) DeleteChunk(ctx context.Context, q *api.DeleteChunkQ) (*api.DeleteChunkS, error) {
	ck.DeleteAllChunks()
	return &api.DeleteChunkS{}, nil
}
