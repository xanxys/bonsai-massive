package main

import (
	"./api"
	"fmt"
	"math/rand"
	"testing"
)

func TestChunkSanityForCell(t *testing.T) {
	wall := &ChunkWall{false, false, false, false}
	chunk := NewGrainChunk(true)
	chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_CELL, int(10), Vec3f{0, 0, 1.0}))
	for i := 0; i < 100; i++ {
		chunk.IncorporateAddition(nil)
		chunk.Step(nil, wall)
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
			chunk.IncorporateAddition(nil)
			chunk.Step(nil, wall)
			assertParticlesAreInBound(chunk, t)
		}
	}
}

// Measure force between two soil grains by checking acceleration.
func TestGrainForceSymmetryXAxis(t *testing.T) {
	// If it satisfies weaker version, it's ok.
	epsilonRel := float32(1e-2)
	epsilonAbs := float32(0.1) // TODO: This is too weak! fix it!
	types := []api.Grain_Kind{api.Grain_WATER, api.Grain_SOIL, api.Grain_CELL}
	for _, tyTarget := range types {
		for _, tyEnv := range types {
			if tyEnv < tyTarget {
				continue
			}
			t.Logf("== %s(targ) - %s(env)\n", tyTarget, tyEnv)
			t.Logf("x, f(x), f(-x)\n")
			for i := 1; i < 200; i++ {
				dist := float32(i) * 1e-3
				fPos := measureForceX(tyTarget, tyEnv, dist)
				fNeg := measureForceX(tyTarget, tyEnv, -dist)
				residue := abs(fPos + fNeg)
				if residue > abs(fPos)*epsilonRel && residue > epsilonAbs {
					t.Logf("%f, %f, %f\n", dist, fPos, fNeg)
					t.Errorf("fPos and fNeg are not symmetric (residue=%f)", residue)
				}
			}
		}
	}
}

func TestDumpGrainForces(t *testing.T) {
	types := []api.Grain_Kind{api.Grain_WATER, api.Grain_SOIL, api.Grain_CELL}

	t.Logf("x\t f(W-W)\t f(W-S)\t f(W-C)\t f(S-W)\t f(S-S)\t f(S-C)\t f(C-W)\t f(C-S)\t f(C-C)\n")
	for i := 1; i < 200; i++ {
		dist := float32(i) * 1e-3
		row := fmt.Sprintf("%f", dist)
		for _, tyTarget := range types {
			for _, tyEnv := range types {
				fPos := measureForceX(tyTarget, tyEnv, dist)
				row = fmt.Sprintf("%s\t %f", row, fPos)
			}
		}
		t.Logf("%s\n", row)
	}
	//	t.Errorf("Dumping")
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	} else {
		return x
	}
}

// Measure the force that a particle at origin (in empty space) will receive
// from another particle at (dist, 0, 0).
func measureForceX(tyTarget, tyEnv api.Grain_Kind, dist float32) float32 {
	wall := &ChunkWall{false, false, false, false}
	chunk := NewGrainChunk(false)
	chunk.gravityAccel = NewVec3f0()

	// Setup two grains:
	// at origin: measurement target
	// at X=dist: field generator
	grains := []*Grain{
		NewGrain(tyTarget, Vec3f{0.5, 0.5, 0.5}),
		NewGrain(tyEnv, Vec3f{0.5 + dist, 0.5, 0.5}),
	}
	grains[1].Velocity.X -= 1e-3 // going away
	chunk.IncorporateAddition(grains)
	chunk.Step(nil, wall)
	return grains[0].Velocity.MultS(1/dt).X * massGrain
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
