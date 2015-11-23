package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (fe *FeServiceImpl) Chunk(ctx context.Context, q *api.ChunkQ) (*api.ChunkS, error) {
	return &api.ChunkS{}, nil
}
