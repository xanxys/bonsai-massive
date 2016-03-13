package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) DeleteChunk(ctx context.Context, q *api.DeleteChunkQ) (*api.DeleteChunkS, error) {
	for _, chunkId := range q.ChunkId {
		ck.ChunkRouter.DeleteChunk(chunkId)
	}
	return &api.DeleteChunkS{}, nil
}
