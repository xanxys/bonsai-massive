package main

import (
	"./api"
	"fmt"
)

type BiosphereTopology interface {
	GetChunkTopos() []*api.ChunkTopology
	GetGlobalOffsets() map[string]Vec3f
}

// Edge X=0, nx is connected with each other at same Y,
// Y edges (0, ny) is walled.
type CylinderTopology struct {
	Nx, Ny int
	bsId   uint64
}

func NewCylinderTopology(bsId uint64, nx, ny int) *CylinderTopology {
	return &CylinderTopology{
		Nx:   nx,
		Ny:   ny,
		bsId: bsId,
	}
}

func (cylinder *CylinderTopology) GetChunkTopos() []*api.ChunkTopology {
	var result []*api.ChunkTopology
	for ix := 0; ix < cylinder.Nx; ix++ {
		for iy := 0; iy < cylinder.Ny; iy++ {
			topo := &api.ChunkTopology{
				ChunkId: fmt.Sprintf(chunkIdFormat, cylinder.bsId, ix, iy),
			}
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					if dx == 0 && dy == 0 {
						continue
					}
					neighborIx := (ix + dx) % cylinder.Nx
					if neighborIx < 0 {
						neighborIx += cylinder.Nx
					}
					neighborIy := iy + dy
					if neighborIy < 0 || neighborIy >= cylinder.Ny {
						continue
					}
					topo.Neighbors = append(topo.Neighbors, &api.ChunkTopology_ChunkNeighbor{
						ChunkId:  fmt.Sprintf(chunkIdFormat, cylinder.bsId, neighborIx, neighborIy),
						Internal: true,
						Dx:       int32(dx),
						Dy:       int32(dy),
					})
				}
			}
			result = append(result, topo)
		}
	}
	return result
}

func (cylinder *CylinderTopology) GetGlobalOffsets() map[string]Vec3f {
	offsets := make(map[string]Vec3f)
	for ix := 0; ix < cylinder.Nx; ix++ {
		for iy := 0; iy < cylinder.Ny; iy++ {
			offsets[fmt.Sprintf(chunkIdFormat, cylinder.bsId, ix, iy)] = Vec3f{
				X: float32(ix),
				Y: float32(iy),
				Z: 0.0,
			}
		}
	}
	return offsets
}
