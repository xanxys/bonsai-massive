package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) ChunkSummary(ctx context.Context, q *api.ChunkSummaryQ) (*api.ChunkSummaryS, error) {
	return &api.ChunkSummaryS{
		Chunks: ck.ChunkRouter.GetChunks(),
	}, nil
}
