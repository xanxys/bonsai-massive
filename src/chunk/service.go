package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
	"math"
	"math/rand"
	"time"
)

// Global world config.
const dt = 1 / 30.0

// Global simulation config.
const floor_static = 0.7
const floor_dynamic = 0.5
const cfm_epsilon = 1e-3
const sand_water_equiv = 0.3

// Global water config.
const reflection_coeff = 0.3 // Must be in (0, 1)
const density_base = 1000.0  // kg/m^3
const h = 0.1
const mass_grain = 0.1 * 113 / 20 // V_sphere(h) * density_base
const num_iter = 3

// Sand config.
const sand_radius = 0.04
const sand_stiffness = 2e-2
const friction_static = 0.5  // must be in [0, 1)
const friction_dynamic = 0.3 // must be in [0, friction_static)

type CkServiceImpl struct {
}

func NewCkService() *CkServiceImpl {
	StartChunk()
	return &CkServiceImpl{}
}

func (ck *CkServiceImpl) Test(ctx context.Context, q *api.TestQ) (*api.TestS, error) {
	return &api.TestS{}, nil
}

// A continuous running part of world executed by at most a single thread.
type Chunk struct {
}

type Vec3f struct {
	X float32
	Y float32
	Z float32
}

func NewVec3f0() *Vec3f {
	return &Vec3f{X: 0, Y: 0, Z: 0}
}

func (v *Vec3f) LengthSq() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

func (v *Vec3f) Length() float32 {
	return float32(math.Sqrt(float64(v.LengthSq())))
}

func (v *Vec3f) Normalized() *Vec3f {
	return v.MultS(1 / v.Length())
}

// Project v onto the plane defined by given normal.
// caller must ensure: normal.Length() == 1
func (v *Vec3f) ProjectOnPlane(normal *Vec3f) *Vec3f {
	return v.Sub(normal.MultS(normal.Dot(v)))
}

func (v *Vec3f) Dot(u *Vec3f) float32 {
	return v.X*u.X + v.Y*u.Y + v.Z*u.Z
}

func (v *Vec3f) Add(u *Vec3f) *Vec3f {
	return &Vec3f{v.X + u.X, v.Y + u.Y, v.Z + u.Z}
}

func (v *Vec3f) Sub(u *Vec3f) *Vec3f {
	return &Vec3f{v.X - u.X, v.Y - u.Y, v.Z - u.Z}
}

func (v *Vec3f) MultS(s float32) *Vec3f {
	return &Vec3f{
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

func NewGrain(isWater bool, initialPos *Vec3f) *Grain {
	return &Grain{
		IsWater:  isWater,
		Position: *initialPos,
		Velocity: *NewVec3f0(),
	}
}

// Poly6 kernel
func SphKernel(dp *Vec3f, h float32) float32 {
	lenSq := dp.LengthSq()
	if lenSq < h*h {
		return float32(math.Pow(float64(h*h-lenSq), 3)) * (315.0 / 64.0 / math.Pi / float32(math.Pow(float64(h), 9)))
	} else {
		return 0
	}
}

// Spiky kernel
func SphKernelGrad(dp *Vec3f, h float32) *Vec3f {
	dpLen := dp.Length()
	if 0 < dpLen && dpLen < h {
		return dp.MultS(float32(math.Pow(float64(h-dpLen), 2)) / dpLen)
	} else {
		return NewVec3f0()
	}
}

// A source that emits constant flow of given type of particle from
// nearly fixed location, until total_num particles are emitted from this source.
// ParticleSource is physically implausible, so it has high change of being
// removed in final version, but very useful for debugging / showing demo.
type ParticleSource struct {
	isWater           bool
	totalNum          int
	framesPerParticle int
	basePositions     []*Vec3f
}

func NewParticleSource(isWater bool, totalNum int, centerPos Vec3f) *ParticleSource {
	return &ParticleSource{
		isWater:           isWater,
		totalNum:          totalNum,
		framesPerParticle: 4,
		basePositions: []*Vec3f{
			centerPos.Add(&Vec3f{-0.1, -0.1, 0}),
			centerPos.Add(&Vec3f{-0.1, 0.1, 0}),
			centerPos.Add(&Vec3f{0.1, -0.1, 0}),
			centerPos.Add(&Vec3f{0.1, 0.1, 0}),
		},
	}
}

// Return particles to be inserted to the scene at given timestamp.
func (ps *ParticleSource) MaybeEmit(timestamp uint64) []*Grain {
	ts := int(timestamp)
	if ts >= ps.framesPerParticle*ps.totalNum {
		return []*Grain{}
	}
	if ts%ps.framesPerParticle != 0 {
		return []*Grain{}
	}

	phase := (ts / ps.framesPerParticle) % len(ps.basePositions)
	initialPos := ps.basePositions[phase].Add(
		&Vec3f{rand.Float32() * 0.01, rand.Float32() * 0.01, 0})
	return []*Grain{NewGrain(ps.isWater, initialPos)}
}

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
type GrainWorld struct {
	Grains    []*Grain
	Sources   []*ParticleSource
	Timestamp uint64
}

func NewGrainWorld() *GrainWorld {
	return &GrainWorld{
		Grains: []*Grain{},
		Sources: []*ParticleSource{
			NewParticleSource(true, 3000, Vec3f{0.5, 0.5, 2.0}),
			NewParticleSource(false, 3000, Vec3f{0.1, 0.1, 1.0}),
		},
		Timestamp: 0,
	}
}

type BinKey struct {
	X, Y, Z int
}

// Return neighbor indices for each grain index.
// Neighbors of g = {g'.PositionNew.DistanceTo(g.PositionNew) < h | g in grains }
// Note that neighbors contains itself.
func (world *GrainWorld) IndexNeighbors(h float32) [][]int {
	toBinKey := func(grain *Grain) BinKey {
		indexF := grain.PositionNew.MultS(1 / h)
		return BinKey{
			X: int(indexF.X),
			Y: int(indexF.Y),
			Z: int(indexF.Z),
		}
	}

	// Bin all grain indices.
	bins := make(map[BinKey][]int)
	for ix, grain := range world.Grains {
		key := toBinKey(grain)
		gs, exist := bins[key]
		if exist {
			bins[key] = append(gs, ix)
		} else {
			bins[key] = []int{ix}
		}
	}

	// For each grain, lookup nearby bins and filter actual neighbors.
	var neighborIndex [][]int
	for _, grain := range world.Grains {
		var neighbors []int
		key := toBinKey(grain)
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for dz := -1; dz <= 1; dz++ {
					nKey := BinKey{key.X + dx, key.Y + dy, key.Z + dz}
					gs, exist := bins[nKey]
					if exist {
						for _, gIndex := range gs {
							if world.Grains[gIndex].PositionNew.Sub(&grain.PositionNew).Length() < h {
								neighbors = append(neighbors, gIndex)
							}
						}
					}
				}
			}
		}
		neighborIndex = append(neighborIndex, neighbors)
	}
	return neighborIndex
}

type Constraint struct {
	Value float32
	Grads map[int]*Vec3f
}

// :: [{
//    constraint: number,
//    gradient: Map Index Vector3
// }]
// Typically, gradient contains ixTarget.
// Result can be empty when there's no active constraint for given
// particle.
// gradient(ix) == Deriv[constraint, pos[ix]]
func (world *GrainWorld) ConstraintsFor(neighbors [][]int, ixTarget int) []Constraint {
	density := func() float32 {
		if !world.Grains[ixTarget].IsWater {
			log.Fatal("density is only applicable to water grains")
		}
		var acc float32
		for _, ixOther := range neighbors[ixTarget] {
			equiv := float32(1.0)
			if !world.Grains[ixOther].IsWater {
				equiv = sand_water_equiv
			}
			weight := SphKernel(world.Grains[ixTarget].PositionNew.Sub(&world.Grains[ixOther].PositionNew), h)
			acc += weight * mass_grain * equiv
		}
		return acc
	}

	density_constraint_deriv := func() map[int]*Vec3f {
		if !world.Grains[ixTarget].IsWater {
			log.Fatal("gradient of density is only defined for water")
		}
		grads := make(map[int]*Vec3f)
		for _, ixDeriv := range neighbors[ixTarget] {
			equiv := float32(1.0)
			if !world.Grains[ixDeriv].IsWater {
				equiv = sand_water_equiv
			}

			gradAccum := NewVec3f0()
			for _, ixOther := range neighbors[ixTarget] {
				if ixOther == ixTarget {
					continue
				}
				other_equiv := float32(1.0)
				if !world.Grains[ixOther].IsWater {
					other_equiv = sand_water_equiv
				}

				if ixDeriv == ixOther {
					gradAccum = gradAccum.Add(
						SphKernelGrad(
							world.Grains[ixOther].PositionNew.Sub(&world.Grains[ixTarget].PositionNew),
							h).MultS(equiv * other_equiv))
				} else if ixDeriv == ixTarget {
					gradAccum = gradAccum.Add(
						SphKernelGrad(
							world.Grains[ixTarget].PositionNew.Sub(&world.Grains[ixOther].PositionNew),
							h).MultS(equiv * other_equiv))
				}
			}
			grads[ixDeriv] = gradAccum.MultS(-1 / density_base)
		}
		return grads
	}

	var cs []Constraint
	if world.Grains[ixTarget].IsWater {
		cs = append(cs, Constraint{
			Value: density()/density_base - 1,
			Grads: density_constraint_deriv(),
		})
	} else {
		// This will result in 2 same constraints per particle pair,
		// but there's no problem (other than performance) for repeating
		// same constraint.
		for _, ixOther := range neighbors[ixTarget] {
			if ixTarget == ixOther {
				continue // no collision with self
			}
			if world.Grains[ixOther].IsWater {
				continue // No sand-other interaction for now.
			}
			dp := world.Grains[ixTarget].PositionNew.Sub(&world.Grains[ixOther].PositionNew)
			penetration := sand_radius*2 - dp.Length()
			if penetration > 0 {
				// Collision (no penetration) constraint.
				f_normal := penetration * sand_stiffness
				dp = dp.Normalized()
				grads := make(map[int]*Vec3f)
				grads[ixOther] = dp.MultS(sand_stiffness)
				grads[ixTarget] = dp.MultS(-sand_stiffness)
				cs = append(cs, Constraint{
					Value: f_normal,
					Grads: grads,
				})

				// Tangential friction constraint.
				dv := world.Grains[ixTarget].PositionNew.Sub(&world.Grains[ixTarget].Position).Sub(
					world.Grains[ixOther].PositionNew.Sub(&world.Grains[ixOther].Position))
				dir_tangent := dv.ProjectOnPlane(dp).Normalized()

				// Both max static friction & dynamic friction are proportional to
				// force along normal (collision).
				if dv.LengthSq() > 0 {
					grads_t := make(map[int]*Vec3f)
					f_tangent := dv.Length()
					if f_tangent < f_normal*friction_static {
						// Static friction.
						grads_t[ixOther] = dir_tangent.MultS(-f_tangent)
						grads_t[ixTarget] = dir_tangent.MultS(f_tangent)
					} else {
						// Dynamic friction.
						f_tangent = f_normal * friction_dynamic
						if f_tangent >= dv.Length() {
							log.Panicln("Dynamic friction condition breached")
						}
						grads_t[ixOther] = dir_tangent.MultS(-f_tangent)
						grads_t[ixTarget] = dir_tangent.MultS(f_tangent)
					}
					cs = append(cs, Constraint{
						Value: f_tangent,
						Grads: grads_t,
					})
				}
			}
		}
	}
	return cs
}

// Position-based dynamics.
func (world *GrainWorld) Step() {
	accel := Vec3f{0, 0, -1}

	// Emit new particles.
	for _, source := range world.Sources {
		world.Grains = append(world.Grains, source.MaybeEmit(world.Timestamp)...)
	}
	// Apply gravity & velocity.
	for _, grain := range world.Grains {
		grain.PositionNew = *grain.Position.Add(grain.Velocity.MultS(dt)).Add(accel.MultS(0.5 * dt * dt))
	}
	// Index spatially.
	neighbors := world.IndexNeighbors(h)

	// Iteratively resolve collisions & constraints.
	for iter := 0; iter < num_iter; iter++ {
		for ix, _ := range world.Grains {
			constraints := world.ConstraintsFor(neighbors, ix)
			for _, constraint := range constraints {
				var gradLengthSq float32
				for ixFeedback := range constraint.Grads {
					gradLengthSq += constraint.Grads[ixFeedback].LengthSq()
				}
				scale := -constraint.Value / (gradLengthSq + cfm_epsilon)

				for ixFeedback := range constraint.Grads {
					world.Grains[ixFeedback].PositionNew = *world.Grains[ixFeedback].PositionNew.Add(constraint.Grads[ixFeedback].MultS(scale))
				}
			}
		}

		// Box collision & floor friction.
		for _, grain := range world.Grains {
			if grain.PositionNew.X < 0 {
				grain.PositionNew.X *= -reflection_coeff
			} else if grain.PositionNew.X > 1 {
				grain.PositionNew.X = 1 - (grain.PositionNew.X-1)*reflection_coeff
			}
			if grain.PositionNew.Y < 0 {
				grain.PositionNew.Y *= -reflection_coeff
			} else if grain.PositionNew.Y > 1 {
				grain.PositionNew.Y = 1 - (grain.PositionNew.Y-1)*reflection_coeff
			}
			if grain.PositionNew.Z < 0 {
				dz := -grain.PositionNew.Z * (1 + reflection_coeff)
				grain.PositionNew.Z += dz

				dxy := grain.PositionNew.Sub(&grain.Position).ProjectOnPlane(&Vec3f{0, 0, 1})
				if dxy.Length() < dz*floor_static {
					// Static friction.
					grain.PositionNew.X = grain.Position.X
					grain.PositionNew.Y = grain.Position.Y
				} else {
					// Dynamic friction.
					dxy_capped := dxy.Length()
					if dxy_capped > dz*floor_dynamic {
						dxy_capped = dz * floor_dynamic
					}
					grain.PositionNew = *grain.PositionNew.Sub(dxy.Normalized().MultS(dxy_capped))
				}
			}
		}
	}

	// Actually update velocity & position.
	// PositionNew is destroyed after this.
	for _, grain := range world.Grains {
		grain.Velocity = *grain.PositionNew.Sub(&grain.Position).MultS(1.0 / dt)
		grain.Position = grain.PositionNew
	}

	world.Timestamp++
}

func benchmark() {
	t0 := time.Now()
	world := NewGrainWorld()
	steps := 300 * 4
	for iter := 0; iter < steps; iter++ {
		world.Step()
	}
	log.Printf("Benchmark: %.3fs for %d steps", float64(time.Since(t0))*1e-6, steps)
}

// TODO: split internal / external representation.
func StartChunk() *Chunk {
	benchmark()
	go func() {

	}()
	return nil
}
