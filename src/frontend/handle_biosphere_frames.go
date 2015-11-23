package main

import (
	"./api"
	"golang.org/x/net/context"
	"math"
)

func (fe *FeServiceImpl) BiosphereFrames(ctx context.Context, q *api.BiosphereFramesQ) (*api.BiosphereFramesS, error) {
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

	ps := &api.PolySoup{}
	for i := 0; i < len(indices); i++ {
		ps.Vertices = append(ps.Vertices, &api.PolySoup_Vertex{
			Px: vertices[indices[i]*3+0],
			Py: vertices[indices[i]*3+1],
			Pz: vertices[indices[i]*3+2],
		})
	}

	return &api.BiosphereFramesS{
		Content: ps,
	}, nil
}