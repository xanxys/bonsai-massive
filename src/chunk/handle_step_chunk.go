package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) StepChunk(ctx context.Context, q *api.StepChunkQ) (*api.StepChunkS, error) {
	return &api.StepChunkS{
		NewTimestamp: 12345,
	}, nil
}
