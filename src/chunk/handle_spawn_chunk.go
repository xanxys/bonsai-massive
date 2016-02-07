package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

func (ck *CkServiceImpl) SpawnChunk(ctx context.Context, q *api.SpawnChunkQ) (*api.SpawnChunkS, error) {
	go RunChunk(ck.ChunkRouter, q)
	return &api.SpawnChunkS{}, nil
}

func RunChunk(router *ChunkRouter, q *api.SpawnChunkQ) {
	topo := q.Topology

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

	chunk := NewGrainChunk(false)
	if q.NumSoil > 0 {
		chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_SOIL, int(q.NumSoil), Vec3f{0.5, 0.5, 2.0}))
	}
	if q.NumWater > 0 {
		chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_WATER, int(q.NumWater), Vec3f{0.5, 0.55, 2.1}))
	}
	chunk.Sources = append(chunk.Sources, NewParticleSource(api.Grain_CELL, int(10), Vec3f{0.55, 0.5, 2.2}))

	if !router.RegisterNewChunk(topo) {
		log.Printf("RunChunk(%s) exiting because it's already running", topo.ChunkId)
		return
	}
	// Post initial empty state to unblock other chunks.
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

		// Unpack imported things and import.
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
		chunk.IncorporateAddition(incomingGrains)

		// Persist when requested.
		if q.SnapshotModulo > 0 && chunk.Timestamp%uint64(q.SnapshotModulo) == 0 {
			log.Printf("Snapshotting at t=%d", chunk.Timestamp)
		}

		// Actual simulation.
		escapedGrains := chunk.Step(envGrains, wall)

		// Pack exported things.
		grains := make([]*api.Grain, len(chunk.Grains))
		for ix, grain := range chunk.Grains {
			grains[ix] = ser(grain)
		}
		bins := make(map[string][]*api.Grain)
		for _, escapedGrain := range escapedGrains {
			coord := binExternal(relToId, escapedGrain.Position)
			if coord == nil {
				continue
			}
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
		log.Printf("Pos declared ougoing, but found in-chunk: %#v", pos)
		return nil
	}

	key, ok := relToId[ChunkRel{ix, iy}]
	if ok {
		return &WorldCoord2{key, pos.Sub(Vec3f{float32(ix), float32(iy), 0})}
	} else {
		log.Printf("Grain (pos %v) escaped to walled region, returning (0.5, 0.5, 10)", pos)
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
		Kind:     grain.Kind,
		CellProp: grain.CellProp,
	}
}

func ser(grain *Grain) *api.Grain {
	p := grain.Position
	v := grain.Velocity
	return &api.Grain{
		Id:       grain.Id,
		Pos:      &api.CkPosition{p.X, p.Y, p.Z},
		Vel:      &api.CkVelocity{v.X, v.Y, v.Z},
		Kind:     grain.Kind,
		CellProp: grain.CellProp,
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
