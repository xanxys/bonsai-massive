package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

type ChunkResult struct {
	Snapshot  *api.ChunkSnapshot
	Timestamp uint64
}

type CkServiceImpl struct {
	// Send null to terminate chunk executor permanently.
	chunkCommand chan *api.ModifyChunkQ

	chunkQuery chan bool

	chunkResult chan *ChunkResult
}

func NewCkService() *CkServiceImpl {
	ch := make(chan *api.ModifyChunkQ, 5)
	chQ := make(chan bool, 5)
	chR := make(chan *ChunkResult, 5)
	go worldController(ch, chQ, chR)
	return &CkServiceImpl{
		chunkCommand: ch,
		chunkQuery:   chQ,
		chunkResult:  chR,
	}
}

// Synchronize multiple chunks in a world, and responds to external command.
func worldController(ch chan *api.ModifyChunkQ, chQ chan bool, chR chan *ChunkResult) {
	log.Printf("Grain worlds created")
	gchunks := []*GrainChunk{
		NewGrainChunk(),
		NewGrainChunk(),
	}
	for {
		select {
		case command := <-ch:
			log.Printf("%v\n", command)
		case <-chQ:
			numGrains := 0
			timestamp := gchunks[0].Timestamp
			for _, gchunk := range gchunks {
				numGrains += len(gchunk.Grains)
				if gchunk.Timestamp != timestamp {
					log.Panicf("Chunk desynchronized just after synchronization (%d != %d)", timestamp, gchunk.Timestamp)
				}
			}
			log.Printf("Snapshotting at timestamp %d (%d grains, %d chunks)", timestamp, numGrains, len(gchunks))
			snapshot := &api.ChunkSnapshot{
				Grains: make([]*api.CkPosition, numGrains),
			}
			ix_offset := 0
			for ixChunk, gchunk := range gchunks {
				for ix, grain := range gchunk.Grains {
					// round to unit (0.1mm)
					p := grain.Position.Add(Vec3f{float32(ixChunk), 0, 0}).MultS(10000)
					snapshot.Grains[ix_offset+ix] = &api.CkPosition{
						int32(p.X + 0.5),
						int32(p.Y + 0.5),
						int32(p.Z + 0.5),
					}
				}
				ix_offset += len(gchunk.Grains)
			}
			chR <- &ChunkResult{
				Snapshot:  snapshot,
				Timestamp: timestamp,
			}
		default:
		}
		for _, gchunk := range gchunks {
			gchunk.Step(&ChunkWall{
				Xm: true,
				Xp: true,
				Ym: true,
				Yp: true,
			})
		}
	}
}

func (ck *CkServiceImpl) Benchmark(ctx context.Context, q *api.BenchmarkQ) (*api.BenchmarkS, error) {
	Benchmark()
	return &api.BenchmarkS{
		Report: "No report",
	}, nil
}
