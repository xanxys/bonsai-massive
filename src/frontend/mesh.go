package main

import (
	"./api"
	"log"
	"math"
	"math/rand"
)

type Mesh []Vertex

func (mesh Mesh) Serialize() *api.PolySoup {
	if len(mesh)%3 != 0 {
		log.Panicf("Trying to serialize broken mesh with #vertices=%d", len(mesh))
	}

	ps := api.PolySoup{
		Vertices: make([]*api.PolySoup_Vertex, len(mesh)),
	}
	for ix, vert := range mesh {
		ps.Vertices[ix] = &api.PolySoup_Vertex{
			Px: vert.Pos.X,
			Py: vert.Pos.Y,
			Pz: vert.Pos.Z,
			R:  rand.Float32(),
			G:  rand.Float32(),
			B:  rand.Float32(),
		}
	}
	return &ps
}

// Vertex is a wrapper for PolySoup_Vertex using Vector.
type Vertex struct {
	Pos Vec3f
	Nr  Vec3f
	Col Vec3f
}

// Create an icosahedron mesh. returned mesh will be approximation of
// a sphere of given center and radius, but radius is actually ill-defined.
func Icosahedron(center Vec3f, radius float32) Mesh {
	// Icosahedron definition.
	// Adopted from https://github.com/mrdoob/three.js/blob/master/src/extras/geometries/IcosahedronGeometry.js
	t := float32((1 + math.Sqrt(5)) / 2)
	vertices := []float32{
		-1, t, 0, 1, t, 0, -1, -t, 0, 1, -t, 0,
		0, -1, t, 0, 1, t, 0, -1, -t, 0, 1, -t,
		t, 0, -1, t, 0, 1, -t, 0, -1, -t, 0, 1,
	}
	indices := []int{
		0, 11, 5, 0, 5, 1, 0, 1, 7, 0, 7, 10, 0, 10, 11,
		1, 5, 9, 5, 11, 4, 11, 10, 2, 10, 7, 6, 7, 1, 8,
		3, 9, 4, 3, 4, 2, 3, 2, 6, 3, 6, 8, 3, 8, 9,
		4, 9, 5, 2, 4, 11, 6, 2, 10, 8, 6, 7, 9, 8, 1,
	}

	var vs Mesh
	for i := 0; i < len(indices); i++ {
		posBase := Vec3f{vertices[indices[i]*3+0], vertices[indices[i]*3+1], vertices[indices[i]*3+2]}
		vs = append(vs, Vertex{
			Pos: posBase.MultS(radius * 0.5).Add(center),
		})
	}
	return vs
}
