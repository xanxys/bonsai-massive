package main

import (
	"./api"
	"log"
	"math"
)

// Wrapper for PolySoup
type Mesh struct {
	vertices []Vertex
	indices  []uint32
}

func (mesh *Mesh) Serialize() *api.PolySoup {
	if len(mesh.indices)%3 != 0 {
		log.Panicf("Trying to serialize broken mesh with #vertices=%d #indices=%d", len(mesh.vertices), len(mesh.indices))
	}

	ps := api.PolySoup{
		Vertices: make([]*api.PolySoup_Vertex, len(mesh.vertices)),
		Indices:  make([]uint32, len(mesh.indices)),
	}
	for ix, vert := range mesh.vertices {
		ps.Vertices[ix] = &api.PolySoup_Vertex{
			Px: round3(vert.Pos.X),
			Py: round3(vert.Pos.Y),
			Pz: round3(vert.Pos.Z),
			R:  round3(vert.Col.X),
			G:  round3(vert.Col.Y),
			B:  round3(vert.Col.Z),
		}
	}
	copy(ps.Indices, mesh.indices)
	return &ps
}

func NewMesh() *Mesh {
	return &Mesh{}
}

// Merge md to mesh. For efficiency, mesh should be bigger than md.
func (mesh *Mesh) Merge(md *Mesh) {
	delta_indices := make([]uint32, len(md.indices))
	v_offset := uint32(len(mesh.vertices))
	for ix, v_index := range md.indices {
		delta_indices[ix] = v_offset + v_index
	}
	mesh.vertices = append(mesh.vertices, md.vertices...)
	mesh.indices = append(mesh.indices, delta_indices...)
}

func round3(x float32) float32 {
	return float32(math.Floor(float64(x)*1e3+0.5) * 1e-3)
}

// Quantize by 1/256.
func round256(x float32) float32 {
	return float32(math.Floor(float64(x)*256+0.5) * 0.00390625)
}

// Quantize by 1/1024.
func round1024(x float32) float32 {
	return float32(math.Floor(float64(x)*1024+0.5) * 0.0009765625)
}

// Vertex is a wrapper for PolySoup_Vertex using Vector.
type Vertex struct {
	Pos Vec3f
	Nr  Vec3f
	Col Vec3f
}

// rgb must be in [0, 1]
func (mesh *Mesh) SetColor(rgb Vec3f) {
	for ix, _ := range mesh.vertices {
		mesh.vertices[ix].Col = rgb
	}
}

// Create an icosahedron mesh. returned mesh will be approximation of
// a sphere of given center and radius, but radius is actually ill-defined.
func Icosahedron(center Vec3f, radius float32) *Mesh {
	// Icosahedron definition.
	// Adopted from https://github.com/mrdoob/three.js/blob/master/src/extras/geometries/IcosahedronGeometry.js
	t := float32((1 + math.Sqrt(5)) / 2)
	vertices := []float32{
		-1, t, 0, 1, t, 0, -1, -t, 0, 1, -t, 0,
		0, -1, t, 0, 1, t, 0, -1, -t, 0, 1, -t,
		t, 0, -1, t, 0, 1, -t, 0, -1, -t, 0, 1,
	}
	indices := []uint32{
		0, 11, 5, 0, 5, 1, 0, 1, 7, 0, 7, 10, 0, 10, 11,
		1, 5, 9, 5, 11, 4, 11, 10, 2, 10, 7, 6, 7, 1, 8,
		3, 9, 4, 3, 4, 2, 3, 2, 6, 3, 6, 8, 3, 8, 9,
		4, 9, 5, 2, 4, 11, 6, 2, 10, 8, 6, 7, 9, 8, 1,
	}

	num_vertices := len(vertices) / 3
	mesh := &Mesh{
		vertices: make([]Vertex, num_vertices),
		indices:  make([]uint32, len(indices)),
	}
	copy(mesh.indices, indices)
	for ix := 0; ix < num_vertices; ix++ {
		posBase := Vec3f{vertices[ix*3+0], vertices[ix*3+1], vertices[ix*3+2]}
		mesh.vertices[ix] = Vertex{
			Pos: posBase.MultS(radius * 0.5).Add(center),
		}
	}
	return mesh
}
