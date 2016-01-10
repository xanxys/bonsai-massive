package main

import (
	"./api"
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
)

func hashFnv1(data []byte) uint64 {
	hash := uint64(0xcbf29ce484222325)
	for _, b := range data {
		hash *= uint64(0x100000001b3)
		hash ^= uint64(b)
	}
	return hash
}

func random1(seed uint64, x int32) float64 {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, seed)
	binary.Write(buf, binary.BigEndian, x)
	return float64(hashFnv1(buf.Bytes())) / math.Pow(2, 64)
}

// Return deterministic random value in [0, 1].
func random2(seed uint64, x, y int32) float64 {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, seed)
	binary.Write(buf, binary.BigEndian, x)
	binary.Write(buf, binary.BigEndian, y)
	return float64(hashFnv1(buf.Bytes())) / math.Pow(2, 64)
}

func interpolateCos(v0, v1, t float64) float64 {
	f := (1 - math.Cos(math.Pi*t)) * 0.5
	return v0*(1-f) + v1*f
}

// Return band-limited noise in [0, 1].
// One pseudo-random value is taken in [0,1]^2 area.
func noise2(seed uint64, x float64, y float64) float64 {
	ix := math.Floor(x)
	iy := math.Floor(y)
	fracX := x - ix
	fracY := y - iy
	iix := int32(ix)
	iiy := int32(iy)
	vY0 := interpolateCos(random2(seed, iix, iiy), random2(seed, iix+1, iiy), fracX)
	vY1 := interpolateCos(random2(seed, iix, iiy+1), random2(seed, iix+1, iiy+1), fracX)
	return interpolateCos(vY0, vY1, fracY)
}

// See http://freespace.virgin.net/hugo.elias/models/m_perlin.htm
func perlin2(seed uint64, x, y float64) float64 {
	src := rand.NewSource(int64(seed))

	persistence := 0.5
	numOctave := 3

	amplitude := 1.0
	freq := 1.0
	v := 0.0
	for ixOct := 0; ixOct < numOctave; ixOct++ {
		octSeed := uint64(src.Int63())
		v += noise2(octSeed, x*freq, y*freq) * amplitude
		amplitude *= persistence
		freq *= 2
	}
	return v
}

// Generate actual particle nunmbers from given environment config and topology.
// len(result) == <#chunks in topo>
// This function is deterministic.
func GenerateEnv(topo BiosphereTopology, env *api.BiosphereEnvConfig) []*api.SpawnChunkQ {
	topos := topo.GetChunkTopos()
	offsets := topo.GetGlobalOffsets()
	chunkQs := make([]*api.SpawnChunkQ, len(topos))
	for chunkIx, topo := range topos {
		chunkOffset := offsets[topo.ChunkId]
		height := perlin2(uint64(env.Seed), float64(chunkOffset.X), float64(chunkOffset.Y))

		hWater := 0.3
		if height > 0.5 {
			hWater = 0.01
		}
		chunkQs[chunkIx] = &api.SpawnChunkQ{
			Topology: topo,
			NumSoil:  int32(height * 300),
			NumWater: int32(hWater * 300),
		}
	}
	return chunkQs
}
