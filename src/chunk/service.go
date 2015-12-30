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

type World interface {
	GetEmbeddedChunks() []EmbeddedChunk

	// Ensure that given point lies strictly within some chunk.
	// point must not be too far outside of the source chunk.
	Canonicalize(point *WorldCoord) *WorldCoord

	//
	Transfer(point *WorldCoord, dstChunk ChunkKey) *WorldCoord
}

type ChunkKey struct {
	Dx int
	Dy int
}

type EmbeddedChunk struct {
	Key   ChunkKey
	Chunk *GrainChunk
	Wall  ChunkWall
}

// Edge X=0, nx is connected with each other at same Y,
// Y edges (0, ny) is walled.
type CylinderWorld struct {
	nx, ny int
	chunks [][]*GrainChunk
}

func NewCylinderWorld(nx, ny int) *CylinderWorld {
	chunks := make([][]*GrainChunk, nx)
	for ix := 0; ix < nx; ix++ {
		chunks[ix] = make([]*GrainChunk, ny)
		for iy := 0; iy < ny; iy++ {
			chunks[ix][iy] = NewGrainChunk()
		}
	}
	return &CylinderWorld{
		chunks: chunks,
		nx:     nx,
		ny:     ny,
	}
}

func (world *CylinderWorld) GetEmbeddedChunks() []EmbeddedChunk {
	var result []EmbeddedChunk
	for ix, chunks := range world.chunks {
		for iy, chunk := range chunks {
			result = append(result, EmbeddedChunk{
				Key:   ChunkKey{Dx: ix, Dy: iy},
				Chunk: chunk,
				Wall: ChunkWall{
					Xm: false,
					Xp: false,
					Ym: iy == 0,
					Yp: iy == world.ny-1,
				},
			})
		}
	}
	return result
}

func (world *CylinderWorld) Canonicalize(point *WorldCoord) *WorldCoord {
	// Clip Y.
	if point.Dy == 0 && point.Position.Y < 0 {
		log.Printf("%v is trying to escape from CylinderWorld, clipped to 0 (Dy=0)", point)
		point = &WorldCoord{point.ChunkKey, point.Position}
		point.Position.Y = 0
	} else if point.Dy == world.ny-1 && point.Position.Y >= 1.0 {
		slightlyInside := float32(1.0 - 1e-4)
		log.Printf("%v is trying to escape from CylinderWorld, clipped to %f (Dy=%d)", point, slightlyInside, world.ny-1)
		point = &WorldCoord{point.ChunkKey, point.Position}
		point.Position.Y = slightlyInside
	}

	dx := ifloor(point.Position.X)
	dy := ifloor(point.Position.Y)
	if dx == 0 && dy == 0 {
		log.Printf("Trying to canonicalize already canonical coordinate %v", point)
	} else if iabs(dx)+iabs(dy) > 2 {
		log.Printf("Trying to canonicalize far-away point, %v", point)
	}

	// Apply modulo to X
	newDx := (point.Dx + dx) % world.nx
	if newDx < 0 {
		newDx += world.nx
	}
	// Cap at Y (and emit warning)
	newDy := point.Dy + dy
	if newDy < 0 || newDy >= world.ny {
		log.Panicf("Dy enforcement failed for %v", point)
	}
	return &WorldCoord{
		ChunkKey{Dx: newDx, Dy: newDy},
		point.Position.Sub(Vec3f{X: float32(dx), Y: float32(dy)}),
	}
}

func (world *CylinderWorld) Transfer(point *WorldCoord, dstChunk ChunkKey) *WorldCoord {
	dx := dstChunk.Dx - point.Dx
	dy := dstChunk.Dy - point.Dy
	if iabs(dx) > world.nx/2 {
		if dx > 0 {
			dx -= world.nx
		} else {
			dx += world.nx
		}
	}
	if iabs(dx)+iabs(dy) > 2 {
		log.Printf("Transferring %v to %v, (distance seems too long, mahnattan dist=%d)", point, dstChunk, iabs(dx)+iabs(dy))
	}
	return &WorldCoord{
		Position: point.Position.Sub(Vec3f{X: float32(dx), Y: float32(dy)}),
		ChunkKey: dstChunk,
	}
}

type WorldCoord struct {
	ChunkKey
	Position Vec3f
}

type EscapedGrains struct {
	key    ChunkKey
	grains []*Grain
}

// Synchronize multiple chunks in a world, and responds to external command.
func worldController(ch chan *api.ModifyChunkQ, chQ chan bool, chR chan *ChunkResult) {
	log.Printf("Grain world created")
	world := NewCylinderWorld(3, 2)
	escapedGrainsList := make([]EscapedGrains, 0)
	for {
		select {
		case command := <-ch:
			log.Printf("%v\n", command)
		case <-chQ:
			numGrains := 0
			timestamp := world.GetEmbeddedChunks()[0].Chunk.Timestamp
			for _, chunk := range world.GetEmbeddedChunks() {
				numGrains += len(chunk.Chunk.Grains)
				if chunk.Chunk.Timestamp != timestamp {
					log.Panicf("Chunk desynchronized just after synchronization (%d != %d)", timestamp, chunk.Chunk.Timestamp)
				}
			}
			log.Printf("Snapshotting at timestamp %d (%d grains, %d chunks)", timestamp, numGrains, len(world.GetEmbeddedChunks()))
			snapshot := &api.ChunkSnapshot{
				Grains: make([]*api.Grain, numGrains),
			}
			ix_offset := 0
			for _, eChunk := range world.GetEmbeddedChunks() {
				for ix, grain := range eChunk.Chunk.Grains {
					var kind api.Grain_Kind
					if grain.IsWater {
						kind = api.Grain_WATER
					} else {
						kind = api.Grain_SOIL
					}
					// round to unit (0.1mm)
					p := grain.Position.Add(Vec3f{float32(eChunk.Key.Dx), float32(eChunk.Key.Dy), 0}).MultS(10000)
					snapshot.Grains[ix_offset+ix] = &api.Grain{
						Id: grain.Id,
						Pos: &api.CkPosition{
							int32(p.X + 0.5),
							int32(p.Y + 0.5),
							int32(p.Z + 0.5),
						},
						Kind: kind,
					}
				}
				ix_offset += len(eChunk.Chunk.Grains)
			}
			chR <- &ChunkResult{
				Snapshot:  snapshot,
				Timestamp: timestamp,
			}
		default:
		}
		// Distribute escapedGrains to chunks as inGrains.
		inGrainsPerChunk := make(map[ChunkKey][]*Grain)
		for _, escapedGrains := range escapedGrainsList {
			for _, grain := range escapedGrains.grains {
				canonCoord := world.Canonicalize(&WorldCoord{
					Position: grain.Position,
					ChunkKey: escapedGrains.key,
				})
				// It's safe to overwrite Position here, since a grain only
				// exists in one place (really?, what will happen w.r.t. envGrains?)
				// TODO: make sure safety
				grain.Position = canonCoord.Position
				inGrainsPerChunk[canonCoord.ChunkKey] = append(inGrainsPerChunk[canonCoord.ChunkKey], grain)
			}
		}
		escapedGrainsList = nil

		for _, chunk := range world.GetEmbeddedChunks() {
			envGrains := make([]*Grain, 0)
			for _, chunkOther := range world.GetEmbeddedChunks() {
				if chunk.Key != chunkOther.Key {
					for _, grain := range chunkOther.Chunk.Grains {
						transferredGrain := *grain
						transferredGrain.Position = world.Transfer(&WorldCoord{
							ChunkKey: chunkOther.Key,
							Position: grain.Position,
						}, chunk.Key).Position
						envGrains = append(envGrains, &transferredGrain)
					}
				}
			}
			escapedGrainsDelta := chunk.Chunk.Step(inGrainsPerChunk[chunk.Key], envGrains, &chunk.Wall)
			escapedGrainsList = append(escapedGrainsList, EscapedGrains{
				key:    chunk.Key,
				grains: escapedGrainsDelta,
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
