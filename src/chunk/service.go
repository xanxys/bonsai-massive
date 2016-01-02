package main

import (
	"./api"
	"golang.org/x/net/context"
)

type CkServiceImpl struct {
	*ChunkRouter
}

func NewCkService() *CkServiceImpl {
	ck := &CkServiceImpl{
		ChunkRouter: StartNewRouter(),
	}
	return ck
}

func (ck *CkServiceImpl) Benchmark(ctx context.Context, q *api.BenchmarkQ) (*api.BenchmarkS, error) {
	Benchmark()
	return &api.BenchmarkS{
		Report: "No report",
	}, nil
}
