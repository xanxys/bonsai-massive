package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

type CkServiceImpl struct {
	// Send null to terminate chunk executor permanently.
	chunkCommand chan *api.ModifyChunkQ

	chunkQuery chan bool

	chunkResult chan *api.ChunkSnapshot
}

func NewCkService() *CkServiceImpl {
	ch := make(chan *api.ModifyChunkQ, 5)
	chQ := make(chan bool, 5)
	chR := make(chan *api.ChunkSnapshot, 5)
	go func() {
		log.Printf("Grain world created")
		gworld := NewGrainWorld()
		for {
			select {
			case command := <-ch:
				log.Printf("%v\n", command)
			case <-chQ:
				log.Printf("Snapshotting at timestamp %d (%d grains)", gworld.Timestamp, len(gworld.Grains))
				snapshot := &api.ChunkSnapshot{
					Grains: make([]*api.CkPosition, len(gworld.Grains)),
				}
				for ix, grain := range gworld.Grains {
					// round to unit (0.1mm)
					p := grain.Position.MultS(10000)
					snapshot.Grains[ix] = &api.CkPosition{
						int32(p.X + 0.5),
						int32(p.Y + 0.5),
						int32(p.Z + 0.5),
					}
				}
				chR <- snapshot
			default:
			}
			gworld.Step()
		}
	}()
	return &CkServiceImpl{
		chunkCommand: ch,
		chunkQuery:   chQ,
		chunkResult:  chR,
	}
}

func (ck *CkServiceImpl) Benchmark(ctx context.Context, q *api.BenchmarkQ) (*api.BenchmarkS, error) {
	Benchmark()
	return &api.BenchmarkS{
		Report: "No report",
	}, nil
}
