package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) SpawnChunk(ctx context.Context, q *api.SpawnChunkQ) (*api.SpawnChunkS, error) {
	//go RunChunk()
	return &api.SpawnChunkS{}, nil
}

/*
func RunChunk(topo *api.ChunkTopology) {
	chunk := NewGrainChunk()

	// Recv. run.
	escapedGrainsDelta := chunk.Chunk.Step(inGrainsPerChunk[chunk.Key], envGrains, &chunk.Wall)
	escapedGrainsList = append(escapedGrainsList, EscapedGrains{
		key:    chunk.Key,
		grains: escapedGrainsDelta,
	})
}
*/
