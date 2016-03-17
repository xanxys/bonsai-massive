package main

import (
	"./api"
)

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
