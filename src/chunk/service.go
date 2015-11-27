package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

type CkServiceImpl struct {
	// Send null to terminate chunk executor permanently.
	chunkCommand chan *api.ModifyChunkQ
}

func NewCkService() *CkServiceImpl {
	ch := make(chan *api.ModifyChunkQ)
	go func() {
		for {
			select {
			case command := <-ch:
				log.Printf("%v\n", command)
			}
		}
	}()
	return &CkServiceImpl{
		chunkCommand: ch,
	}
}

func (ck *CkServiceImpl) Benchmark(ctx context.Context, q *api.BenchmarkQ) (*api.BenchmarkS, error) {
	Benchmark()
	return &api.BenchmarkS{
		Report: "No report",
	}, nil
}
