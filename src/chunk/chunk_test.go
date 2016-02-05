package main

import (
	"./api"
	"math/rand"
	"testing"
)

func TestChunkSanityForCell(t *testing.T) {
	wall := &ChunkWall{false, false, false, false}
	chunk := NewGrainChunk(true)
	chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_CELL, int(10), Vec3f{0, 0, 1.0}))
	for i := 0; i < 100; i++ {
		chunk.Step(nil, nil, wall)
		assertParticlesAreInBound(chunk, t)
	}
}

func TestChunkSanityEnclosed(t *testing.T) {
	wall := &ChunkWall{false, false, false, false}
	rand.Seed(1)
	for trial := 0; trial < 10; trial++ {
		chunk := NewGrainChunk(true)
		chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_SOIL, rand.Intn(50), Vec3f{rand.Float32() * 0.1, rand.Float32() * 0.1, 1.0}))
		chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_SOIL, rand.Intn(50), Vec3f{rand.Float32() * 0.1, rand.Float32() * 0.1, 1.0}))
		chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_SOIL, rand.Intn(50), Vec3f{rand.Float32() * 0.1, rand.Float32() * 0.1, 1.0}))
		for i := 0; i < 100; i++ {
			chunk.Step(nil, nil, wall)
			assertParticlesAreInBound(chunk, t)
		}
	}
}

func assertParticlesAreInBound(chunk *GrainChunk, t *testing.T) {
	eps := float32(0.01)
	for ix, grain := range chunk.Grains {
		if grain.Position.X < -eps || grain.Position.X > 1+eps ||
			grain.Position.Y < -eps || grain.Position.Y > 1+eps ||
			grain.Position.Z < -eps || grain.Position.Z > 100 {
			t.Errorf("Grain %d (%#v) has escaped at timestamp %d", ix, grain, chunk.Timestamp)
		}
	}
}
