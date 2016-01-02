package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

func (ck *CkServiceImpl) SpawnChunk(ctx context.Context, q *api.SpawnChunkQ) (*api.SpawnChunkS, error) {
	go RunChunk(ck.ChunkRouter, q.Topology)
	return &api.SpawnChunkS{}, nil
}

func RunChunk(router *ChunkRouter, topo *api.ChunkTopology) {
	// Decode topo once.
	relToId := make(map[ChunkRel]string)
	idToRel := make(map[string]ChunkRel)
	for _, neighbor := range topo.Neighbors {
		rel := ChunkRel{int(neighbor.Dx), int(neighbor.Dy)}
		relToId[rel] = neighbor.ChunkId
		idToRel[neighbor.ChunkId] = rel
	}
	_, canPassXm := relToId[ChunkRel{-1, 0}]
	_, canPassXp := relToId[ChunkRel{1, 0}]
	_, canPassYm := relToId[ChunkRel{0, -1}]
	_, canPassYp := relToId[ChunkRel{0, 1}]
	wall := &ChunkWall{
		Xm: !canPassXm,
		Xp: !canPassXp,
		Ym: !canPassYm,
		Yp: !canPassYp,
	}

	chunk := NewGrainChunk()

	// Need to post first state to unblock other chunks.
	grains := make([]*api.Grain, len(chunk.Grains))
	for ix, grain := range chunk.Grains {
		grains[ix] = ser(grain)
	}
	router.NotifyResult(chunk.Timestamp, topo, &NeighborExport{
		ChunkGrains:   grains,
		EscapedGrains: make(map[string][]*api.Grain),
	})

	for {
		nImport := <-router.RequestNeighbor(chunk.Timestamp, topo)

		// Unpack imported things.
		incomingGrains := make([]*Grain, len(nImport.IncomingGrains))
		for ix, grainProto := range nImport.IncomingGrains {
			incomingGrains[ix] = deser(grainProto)
		}
		var envGrains []*Grain
		for chunkId, sGrains := range nImport.EnvGrains {
			rel := idToRel[chunkId]
			deltaPos := Vec3f{float32(rel.Dx), float32(rel.Dy), 0}
			for _, grainProto := range sGrains {
				grain := deser(grainProto)
				grain.Position = grain.Position.Add(deltaPos)
				envGrains = append(envGrains)
			}
		}

		// Actual simulation step.
		escapedGrains := chunk.Step(incomingGrains, envGrains, wall)

		// Pack exported things.
		grains := make([]*api.Grain, len(chunk.Grains))
		for ix, grain := range chunk.Grains {
			grains[ix] = ser(grain)
		}
		bins := make(map[string][]*api.Grain)
		for _, escapedGrain := range escapedGrains {
			coord := binExternal(relToId, escapedGrain.Position)
			sGrain := ser(escapedGrain)
			sGrain.Pos = &api.CkPosition{
				coord.Pos.X, coord.Pos.Y, coord.Pos.Z,
			}
			bins[coord.Key] = append(bins[coord.Key], sGrain)
		}
		nExport := &NeighborExport{
			ChunkGrains:   grains,
			EscapedGrains: bins,
		}
		router.NotifyResult(chunk.Timestamp, topo, nExport)
	}
}

type ChunkRel struct {
	Dx, Dy int
}

type WorldCoord2 struct {
	Key string
	Pos Vec3f
}

// Convert a known-to-be-outlying point to WorldCoord.
func binExternal(relToId map[ChunkRel]string, pos Vec3f) *WorldCoord2 {
	ix := ifloor(pos.X)
	iy := ifloor(pos.Y)
	if ix == 0 && iy == 0 {
		log.Panicf("Pos declared ougoing, but found in-chunk: %#v", pos)
		return nil
	}

	key, ok := relToId[ChunkRel{ix, iy}]
	if ok {
		return &WorldCoord2{key, pos.Sub(Vec3f{float32(ix), float32(iy), 0})}
	} else {
		log.Panicf("Grain (pos %v) escaped to walled region, returning (0.5, 0.5, 10)", pos)
		return nil
	}
}

func deser(grain *api.Grain) *Grain {
	p := grain.Pos
	v := grain.Vel
	return &Grain{
		Id:       grain.Id,
		Position: Vec3f{p.X, p.Y, p.Z},
		Velocity: Vec3f{v.X, v.Y, v.Z},
		IsWater:  grain.Kind == api.Grain_WATER,
	}
}

func ser(grain *Grain) *api.Grain {
	p := grain.Position
	v := grain.Velocity
	kind := api.Grain_WATER
	if !grain.IsWater {
		kind = api.Grain_SOIL
	}
	return &api.Grain{
		Id:   grain.Id,
		Pos:  &api.CkPosition{p.X, p.Y, p.Z},
		Vel:  &api.CkVelocity{v.X, v.Y, v.Z},
		Kind: kind,
	}
}

func ifloor(x float32) int {
	if x >= 0 {
		return int(x)
	} else {
		return int(x) - 1
	}
}

func iabs(x int) int {
	if x >= 0 {
		return x
	} else {
		return -x
	}
}
