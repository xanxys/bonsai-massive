package main

import (
	"./api"
	"fmt"
	"log"
	"math"
	"math/rand"
)

// Global world config.
const dt = 1 / 30.00

// Global simulation config.
const floor_static = 0.7
const floor_dynamic = 0.5
const cfm_epsilon = 1e-3
const sand_water_equiv = 0.3
const cell_water_equiv = 0.5

// Global water config.
const reflection_coeff = 0.3 // Must be in (0, 1)
const density_base = 1000.0  // kg/m^3
const h = 0.1
const mass_grain = 0.1 * 113.0 / 20.0 // V_sphere(h) * density_base
const num_iter = 3

// Sand config.
const sand_radius = 0.04
const sand_stiffness = 2e-2
const friction_static = 0.5  // must be in [0, 1)
const friction_dynamic = 0.3 // must be in [0, friction_static)

type Grain struct {
	Kind api.Grain_Kind

	Position Vec3f
	Velocity Vec3f

	// A unique id (for entire life of a biosphere) to track identity of grain.
	Id uint64

	// Cell-specific internals.
	CellProp *api.CellProp

	// Temporary buffer to store intermediate position during Step().
	positionNew Vec3f
}

func NewGrain(kind api.Grain_Kind, initialPos Vec3f) *Grain {
	// It's no longer safe to issue ids randomly when we issue more than a few
	// million ids. But for now, it's ok.
	grain := &Grain{
		Kind:     kind,
		Position: initialPos,
		Velocity: NewVec3f0(),
		Id:       uint64(rand.Uint32())<<32 | uint64(rand.Uint32()),
	}
	if kind == api.Grain_CELL {
		grain.CellProp = &api.CellProp{
			Quals: make(map[string]int32),
			Cycle: &api.CellProp_Cycle{
				IsDividing: false,
			},
		}
		grain.CellProp.Quals["zd"] = 1
	}
	return grain
}

// Create an imperfect clone of given cell.
func (parent *Grain) CloneCell() *Grain {
	if parent.Kind != api.Grain_CELL {
		log.Panicf("Expecting cell grain, got %#v", parent)
	}
	return &Grain{
		Kind:     api.Grain_CELL,
		Position: parent.Position.Sub(parent.Velocity.Normalized().MultS(0.01)),
		Velocity: parent.Velocity.MultS(0.5),
		Id:       uint64(rand.Uint32())<<32 | uint64(rand.Uint32()),
		CellProp: &api.CellProp{
			Quals: make(map[string]int32),
			Cycle: &api.CellProp_Cycle{
				IsDividing: false,
			},
		},
	}
}

// Return b^exp. Takes O(log(exp)) time.
func powInt(b float32, exp uint) float32 {
	power := b
	accum := float32(1.0)
	// Example: exp == 5
	// Create {b^1, b^2, b^4, b^8, ...}
	//  (LSB)    1    0    1            == 5
	// b^5 ==  b^1 *     b^4
	for {
		if exp%2 == 1 {
			accum *= power
		}
		exp /= 2
		if exp == 0 {
			return accum
		}
		power = power * power
	}
}

// Poly6 kernel
func SphKernel(dp Vec3f, h float32) float32 {
	lenSq := dp.LengthSq()
	if lenSq < h*h {
		return powInt(h*h-lenSq, 3) * (315.0 / 64.0 / math.Pi / powInt(h, 9))
	} else {
		return 0
	}
}

// Spiky kernel
func SphKernelGrad(dp Vec3f, h float32) Vec3f {
	dpLen := dp.Length()
	if 0 < dpLen && dpLen < h {
		return dp.MultS(powInt(h-dpLen, 2) / dpLen)
	} else {
		return NewVec3f0()
	}
}

// A source that emits constant flow of given type of particle from
// nearly fixed location, until total_num particles are emitted from this source.
// ParticleSource is physically implausible, so it has high change of being
// removed in final version, but very useful for debugging / showing demo.
type ParticleSource struct {
	kind              api.Grain_Kind
	totalNum          int
	framesPerParticle int
	basePositions     []Vec3f
}

func NewParticleSource(kind api.Grain_Kind, totalNum int, centerPos Vec3f) *ParticleSource {
	return &ParticleSource{
		kind:              kind,
		totalNum:          totalNum,
		framesPerParticle: 4,
		basePositions: []Vec3f{
			centerPos.Add(Vec3f{-0.1, -0.1, 0}),
			centerPos.Add(Vec3f{-0.1, 0.1, 0}),
			centerPos.Add(Vec3f{0.1, -0.1, 0}),
			centerPos.Add(Vec3f{0.1, 0.1, 0}),
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
		Vec3f{rand.Float32() * 0.01, rand.Float32() * 0.01, 0})
	return []*Grain{NewGrain(ps.kind, initialPos)}
}

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
type GrainChunk struct {
	Grains    []*Grain
	Sources   []*ParticleSource
	Timestamp uint64
	// Enable error checking. This will cause a few times slowdown.
	checkErrors bool
}

// True: wall exists
// False: transparent (empty unless external particles are supplied)
type ChunkWall struct {
	Xm, Xp, Ym, Yp bool
}

func NewGrainChunk() *GrainChunk {
	return &GrainChunk{
		Timestamp:   0,
		checkErrors: true,
	}
}

type BinKey struct {
	X, Y, Z int
}

// Return neighbor indices for each grain index.
// Neighbors of g = {g'.positionNew.DistanceTo(g.positionNew) < h | g in grains }
// Note that neighbors contains itself.
func (world *GrainChunk) IndexNeighbors(h float32) [][]int {
	toBinKey := func(grain *Grain) BinKey {
		indexF := grain.positionNew.MultS(1 / h)
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
		bins[key] = append(bins[key], ix)
	}

	// For each grain, lookup nearby bins and filter actual neighbors.
	neighborIndex := make([][]int, len(world.Grains))
	for ix, grain := range world.Grains {
		var neighbors []int
		key := toBinKey(grain)
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for dz := -1; dz <= 1; dz++ {
					nKey := BinKey{key.X + dx, key.Y + dy, key.Z + dz}
					gs := bins[nKey]
					for _, gIndex := range gs {
						if world.Grains[gIndex].positionNew.Sub(grain.positionNew).LengthSq() < h*h {
							neighbors = append(neighbors, gIndex)
						}
					}
				}
			}
		}
		neighborIndex[ix] = neighbors
	}
	return neighborIndex
}

type Constraint struct {
	Value float32
	Grads []CGrad
}

type CGrad struct {
	grainIndex int
	grad       Vec3f
}

// :: [{
//    constraint: number,
//    gradient: Map Index Vector3
// }]
// Typically, gradient contains ixTarget.
// Result can be empty when there's no active constraint for given
// particle.
// gradient(ix) == Deriv[constraint, pos[ix]]
func (world *GrainChunk) ConstraintsFor(neighbors [][]int, ixTarget int) []Constraint {
	density := func() float32 {
		if world.Grains[ixTarget].Kind != api.Grain_WATER {
			log.Fatal("density is only defined for water grains")
		}
		var acc float32
		for _, ixOther := range neighbors[ixTarget] {
			equiv := float32(1.0)
			if world.Grains[ixOther].Kind == api.Grain_SOIL {
				equiv = sand_water_equiv
			} else if world.Grains[ixOther].Kind == api.Grain_CELL {
				equiv = cell_water_equiv
			}
			weight := SphKernel(world.Grains[ixTarget].positionNew.Sub(world.Grains[ixOther].positionNew), h)
			acc += weight * mass_grain * equiv
		}
		return acc
	}

	density_constraint_deriv := func() []CGrad {
		if world.Grains[ixTarget].Kind != api.Grain_WATER {
			log.Fatal("gradient of density is only defined for water")
		}
		grads := make([]CGrad, 0, len(neighbors[ixTarget])-1)
		for _, ixDeriv := range neighbors[ixTarget] {
			equiv := float32(1.0)
			if world.Grains[ixDeriv].Kind == api.Grain_SOIL {
				equiv = sand_water_equiv
			} else if world.Grains[ixDeriv].Kind == api.Grain_CELL {
				equiv = cell_water_equiv
			}

			gradAccum := NewVec3f0()
			for _, ixOther := range neighbors[ixTarget] {
				if ixOther == ixTarget {
					continue
				}
				other_equiv := float32(1.0)
				if world.Grains[ixOther].Kind == api.Grain_SOIL {
					other_equiv = sand_water_equiv
				}

				if ixDeriv == ixOther {
					gradAccum = gradAccum.Add(
						SphKernelGrad(
							world.Grains[ixOther].positionNew.Sub(world.Grains[ixTarget].positionNew),
							h).MultS(other_equiv))
				} else if ixDeriv == ixTarget {
					gradAccum = gradAccum.Add(
						SphKernelGrad(
							world.Grains[ixTarget].positionNew.Sub(world.Grains[ixOther].positionNew),
							h).MultS(other_equiv))
				}
			}
			grads = append(grads, CGrad{
				grainIndex: ixDeriv,
				grad:       gradAccum.MultS(-equiv / density_base),
			})
		}
		return grads
	}

	if world.Grains[ixTarget].Kind == api.Grain_WATER {
		return []Constraint{
			Constraint{
				Value: density()/density_base - 1,
				Grads: density_constraint_deriv(),
			},
		}
	} else {
		// SOIL & CELL
		cs := make([]Constraint, 0, len(neighbors[ixTarget]))
		// This will result in 2 same constraints per particle pair,
		// but there's no problem (other than performance) for repeating
		// same constraint.
		for _, ixOther := range neighbors[ixTarget] {
			if ixTarget == ixOther {
				continue // no collision with self
			}
			if world.Grains[ixOther].Kind == api.Grain_WATER {
				continue // No sand-other interaction for now.
			}
			dp := world.Grains[ixTarget].positionNew.Sub(world.Grains[ixOther].positionNew)
			penetration := sand_radius*2 - dp.Length()
			if penetration > 0 {
				// Collision (no penetration) constraint.
				f_normal := penetration * sand_stiffness
				dp = dp.Normalized()
				cs = append(cs, Constraint{
					Value: f_normal,
					Grads: []CGrad{
						CGrad{
							grainIndex: ixOther,
							grad:       dp.MultS(sand_stiffness),
						},
						CGrad{
							grainIndex: ixTarget,
							grad:       dp.MultS(-sand_stiffness),
						},
					},
				})

				// Tangential friction constraint.
				dv := world.Grains[ixTarget].positionNew.Sub(world.Grains[ixTarget].Position).Sub(
					world.Grains[ixOther].positionNew.Sub(world.Grains[ixOther].Position))
				dir_tangent := dv.ProjectOnPlane(dp).Normalized()

				// Both max static friction & dynamic friction are proportional to
				// force along normal (collision).
				dvLen := dv.Length()
				if dvLen > 0 {
					f_tangent := dvLen // Static friction by default.
					if f_tangent >= f_normal*friction_static {
						// Switch to dynamic friction if force is too large.
						f_tangent = f_normal * friction_dynamic
						if f_tangent >= dvLen {
							log.Panicln("Dynamic friction condition breached")
						}
					}
					grads_t := []CGrad{
						CGrad{grainIndex: ixOther, grad: dir_tangent.MultS(-f_tangent)},
						CGrad{grainIndex: ixTarget, grad: dir_tangent.MultS(-f_tangent)},
					}
					cs = append(cs, Constraint{
						Value: f_tangent,
						Grads: grads_t,
					})
				}
			}
		}
		return cs
	}
}

// Take immigrating grains (position must be inside chunk), environmental grains (outside chunk)
// and wall configuration and calculate single step using very stable position-based dynamics.
// Returns grains that escaped this chunk. (they'll be removed from world)
func (world *GrainChunk) Step(inGrains []*Grain, envGrains []*Grain, wall *ChunkWall) []*Grain {
	accel := Vec3f{0, 0, -1}

	// Emit new particles & accept incoming grains.
	for _, source := range world.Sources {
		world.Grains = append(world.Grains, source.MaybeEmit(world.Timestamp)...)
	}
	world.Grains = append(world.Grains, inGrains...)

	// Biological / chemical process (might emit new grain when dividing).
	var cloned []*Grain
	for _, grain := range world.Grains {
		if grain.Kind != api.Grain_CELL {
			continue
		}
		for _, gene := range grain.CellProp.Genome {
			actProb := float32(1.0)
			for _, act := range gene.Activator {
				actProb *= 1 - powInt(0.5, uint(grain.CellProp.Quals[act]))
			}
			gene.ActivationCount += uint32(actProb * 1000)
			if gene.ActivationCount >= 1000 {
				gene.ActivationCount = 0
				for _, prod := range gene.Products {
					grain.CellProp.Quals[prod] = 1 + grain.CellProp.Quals[prod]
				}
			}
		}
		if grain.CellProp.Cycle.IsDividing {
			grain.CellProp.Cycle.DivisionCount++
			if grain.CellProp.Cycle.DivisionCount > 1000 && grain.Velocity.LengthSq() > 0 {
				log.Printf("Cell %d divided", grain.Id)
				grain.CellProp.Cycle.IsDividing = false
				cloned = append(cloned, grain.CloneCell())
			}
		} else {
			if grain.CellProp.Quals["zd"] > 0 {
				grain.CellProp.Cycle.IsDividing = true
				grain.CellProp.Cycle.DivisionCount = 0
			}
		}
	}
	world.Grains = append(world.Grains, cloned...)

	// We won't add / remove grains in this Step anymore, so we can safely append env grains
	// which will be removed later in Step.
	world.Grains = append(world.Grains, envGrains...)

	// Apply gravity & velocity.
	for _, grain := range world.Grains {
		grain.positionNew = grain.Position.Add(grain.Velocity.MultS(dt)).Add(accel.MultS(0.5 * dt * dt))
	}
	// Index spatially.
	neighbors := world.IndexNeighbors(h)

	if world.checkErrors {
		world.verifyFinite("before iteration")
	}
	// Iteratively resolve collisions & constraints.
	for iter := 0; iter < num_iter; iter++ {
		for ix, _ := range world.Grains {
			constraints := world.ConstraintsFor(neighbors, ix)
			for _, constraint := range constraints {
				var gradLengthSq float32
				for _, grad := range constraint.Grads {
					gradLengthSq += grad.grad.LengthSq()
				}
				scale := -constraint.Value / (gradLengthSq + cfm_epsilon)

				for _, grad := range constraint.Grads {
					world.Grains[grad.grainIndex].positionNew = world.Grains[grad.grainIndex].positionNew.Add(grad.grad.MultS(scale))
				}
			}
		}

		// Box collision & floor friction.
		for _, grain := range world.Grains {
			if grain.positionNew.X < 0 {
				if wall.Xm {
					grain.positionNew.X *= -reflection_coeff
				}
			} else if grain.positionNew.X > 1 {
				if wall.Xp {
					grain.positionNew.X = 1 - (grain.positionNew.X-1)*reflection_coeff
				}
			}
			if grain.positionNew.Y < 0 {
				if wall.Ym {
					grain.positionNew.Y *= -reflection_coeff
				}
			} else if grain.positionNew.Y > 1 {
				if wall.Ym {
					grain.positionNew.Y = 1 - (grain.positionNew.Y-1)*reflection_coeff
				}
			}
			if grain.positionNew.Z < 0 {
				dz := -grain.positionNew.Z * (1 + reflection_coeff)
				grain.positionNew.Z += dz

				dxy := grain.positionNew.Sub(grain.Position).ProjectOnPlane(Vec3f{0, 0, 1})
				dxyLen := dxy.Length()
				if dxyLen < dz*floor_static {
					// Static friction.
					grain.positionNew.X = grain.Position.X
					grain.positionNew.Y = grain.Position.Y
				} else {
					// Dynamic friction.
					dxy_capped := dxyLen
					if dxy_capped > dz*floor_dynamic {
						dxy_capped = dz * floor_dynamic
					}
					grain.positionNew = grain.positionNew.Sub(dxy.Normalized().MultS(dxy_capped))
				}
			}
		}
		if world.checkErrors {
			world.verifyFinite(fmt.Sprintf("after iteration %d", iter))
		}
	}
	// Force no-escape-through-wall.
	for _, grain := range world.Grains {
		if grain.positionNew.X < 0 {
			if wall.Xm {
				grain.positionNew.X *= -reflection_coeff
			}
		} else if grain.positionNew.X > 1 {
			if wall.Xp {
				grain.positionNew.X = 1 - (grain.positionNew.X-1)*reflection_coeff
			}
		}
		if grain.positionNew.Y < 0 {
			if wall.Ym {
				grain.positionNew.Y *= -reflection_coeff
			}
		} else if grain.positionNew.Y > 1 {
			if wall.Ym {
				grain.positionNew.Y = 1 - (grain.positionNew.Y-1)*reflection_coeff
			}
		}
	}
	if world.checkErrors {
		world.verifyFinite("after wall enforcement")
	}

	// We don't need envGrains any more. Throw them away.
	world.Grains = world.Grains[0 : len(world.Grains)-len(envGrains)]

	// Actually update velocity & position.
	// positionNew is destroyed after this.
	for _, grain := range world.Grains {
		grain.Velocity = grain.positionNew.Sub(grain.Position).MultS(1.0 / dt)
		grain.Position = grain.positionNew
	}
	if world.checkErrors {
		world.verifyFinite("after v/p update")
	}

	internalGrains := make([]*Grain, 0)
	externalGrains := make([]*Grain, 0)
	for _, grain := range world.Grains {
		if (grain.Position.X < 0 && !wall.Xm) || (grain.Position.X > 1 && !wall.Xp) ||
			(grain.Position.Y < 0 && !wall.Ym) || (grain.Position.Y > 1 && !wall.Yp) {
			externalGrains = append(externalGrains, grain)
		} else {
			internalGrains = append(internalGrains, grain)
		}
	}
	world.Grains = internalGrains
	world.Timestamp++
	return externalGrains
}

func (world *GrainChunk) verifyFinite(context string) {
	for _, grain := range world.Grains {
		if !isFiniteVec(grain.Velocity) || !isFiniteVec(grain.Position) || !isFiniteVec(grain.positionNew) {
			log.Panicf("NaN observed in %#v at %d, %s", grain, world.Timestamp, context)
		}
	}
}

func isFiniteVec(v Vec3f) bool {
	return !(math.IsNaN(float64(v.X)) || math.IsNaN(float64(v.Y)) || math.IsNaN(float64(v.Z)) ||
		math.IsInf(float64(v.X), 0) || math.IsInf(float64(v.Y), 0) || math.IsInf(float64(v.Z), 0))
}
