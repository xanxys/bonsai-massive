package main

import (
	"math"
)

type Vec3f struct {
	X float32
	Y float32
	Z float32
}

func NewVec3f0() Vec3f {
	return Vec3f{X: 0, Y: 0, Z: 0}
}

func (v Vec3f) LengthSq() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

func (v Vec3f) Length() float32 {
	return float32(math.Sqrt(float64(v.LengthSq())))
}

func (v Vec3f) Normalized() Vec3f {
	return v.MultS(1 / v.Length())
}

// Project v onto the plane defined by given normal.
// caller must ensure: normal.Length() == 1
func (v Vec3f) ProjectOnPlane(normal Vec3f) Vec3f {
	return v.Sub(normal.MultS(normal.Dot(v)))
}

func (v Vec3f) Dot(u Vec3f) float32 {
	return v.X*u.X + v.Y*u.Y + v.Z*u.Z
}

func (v Vec3f) Add(u Vec3f) Vec3f {
	return Vec3f{v.X + u.X, v.Y + u.Y, v.Z + u.Z}
}

func (v Vec3f) Sub(u Vec3f) Vec3f {
	return Vec3f{v.X - u.X, v.Y - u.Y, v.Z - u.Z}
}

func (v Vec3f) MultS(s float32) Vec3f {
	return Vec3f{
		X: s * v.X,
		Y: s * v.Y,
		Z: s * v.Z,
	}
}

type Grain struct {
	IsWater  bool
	Position Vec3f
	Velocity Vec3f

	PositionNew Vec3f
}

func NewGrain(isWater bool, initialPos Vec3f) *Grain {
	return &Grain{
		IsWater:  isWater,
		Position: initialPos,
		Velocity: NewVec3f0(),
	}
}
