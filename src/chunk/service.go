package main

import (
	"./api"
	"golang.org/x/net/context"
)

type CkServiceImpl struct {
}

func NewCkService() *CkServiceImpl {
	StartChunk()
	return &CkServiceImpl{}
}

func (ck *CkServiceImpl) Test(ctx context.Context, q *api.TestQ) (*api.TestS, error) {
	return &api.TestS{}, nil
}

// A continuous running part of world executed by at most a single thread.
type Chunk struct {
}

// TODO: split internal / external representation.
func StartChunk() *Chunk {
	Benchmark()
	go func() {

	}()
	return nil
}
