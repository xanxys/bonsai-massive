package main

import (
	"./api"
	"fmt"
	"github.com/kr/pretty"
	"log"
	"math"
	"math/rand"
)

// Global world config.
const dt = 1 / 30.00

// Global simulation config.
const floorStatic = 0.7
const floorDynamic = 0.5
const cfmEpsilon = 1e-2
const sandWaterEquiv = 0.3
const cellWaterEquiv = 0.5

// Global water config.
const reflectionCoeff = 0.3 // Must be in (0, 1)
const densityBase = 1000.0  // kg/m^3
const h = 0.1
const massGrain = 0.1 * 113.0 / 20.0 // V_sphere(h) * densityBase
const numIter = 3

// Sand config.
const sandRadius = 0.05
const sandStiffness = 2e-2
const frictionStatic = 1.5  // must be in [0, inf)
const frictionDynamic = 0.7 // must be in [0, frictionStatic)
const adhesion = 45         // Pa

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
// Must be called after halving of energy.
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
			Energy: parent.CellProp.Energy,
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

// Calculate approximate contact surface from distance of two particles.
func ContactSurface(penetration float32, r float32) float32 {
	if penetration <= 0 {
		return 0
	} else {
		return float32(math.Pi) * powInt(r, 2) * (1 - powInt(penetration/r-1, 2))
	}
}

func ContractSurfaceGrad(penetration float32, r float32) float32 {
	if penetration <= 0 {
		return 0
	} else {
		dist := 2*r - penetration
		return -2 * float32(math.Pi) * dist
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
	Timestamp uint64

	gravityAccel Vec3f

	// TODO: This should be moved to frontend.
	Sources []*ParticleSource

	// Enable error checking. This will cause a few times slowdown.
	checkErrors bool
}

// True: wall exists
// False: transparent (empty unless external particles are supplied)
type ChunkWall struct {
	Xm, Xp, Ym, Yp bool
}

func NewGrainChunk(checkErrors bool) *GrainChunk {
	return &GrainChunk{
		Timestamp:    0,
		checkErrors:  checkErrors,
		gravityAccel: Vec3f{0, 0, -1},
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
				equiv = sandWaterEquiv
			} else if world.Grains[ixOther].Kind == api.Grain_CELL {
				equiv = cellWaterEquiv
			}
			weight := SphKernel(world.Grains[ixTarget].positionNew.Sub(world.Grains[ixOther].positionNew), h)
			acc += weight * massGrain * equiv
		}
		return acc
	}

	densityConstraintDeriv := func() []CGrad {
		if world.Grains[ixTarget].Kind != api.Grain_WATER {
			log.Fatal("gradient of density is only defined for water")
		}
		grads := make([]CGrad, 0, len(neighbors[ixTarget])-1)
		for _, ixDeriv := range neighbors[ixTarget] {
			equiv := float32(1.0)
			if world.Grains[ixDeriv].Kind == api.Grain_SOIL {
				equiv = sandWaterEquiv
			} else if world.Grains[ixDeriv].Kind == api.Grain_CELL {
				equiv = cellWaterEquiv
			}

			gradAccum := NewVec3f0()
			for _, ixOther := range neighbors[ixTarget] {
				if ixOther == ixTarget {
					continue
				}
				other_equiv := float32(1.0)
				if world.Grains[ixOther].Kind == api.Grain_SOIL {
					other_equiv = sandWaterEquiv
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
				grad:       gradAccum.MultS(-equiv / densityBase),
			})
		}
		return grads
	}

	if world.Grains[ixTarget].Kind == api.Grain_WATER {
		return []Constraint{
			Constraint{
				Value: density()/densityBase - 1,
				Grads: densityConstraintDeriv(),
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
			penetration := sandRadius*2 - dp.Length()
			if penetration <= 0 {
				continue
			}
			dirNormal := dp.Normalized()
			adhesionForce := adhesion * ContactSurface(penetration, sandRadius)
			adhesionConstraintV := (powInt(dt, 2) / massGrain) * adhesionForce
			adhesionGrad := adhesion * (powInt(dt, 2) / massGrain) * ContractSurfaceGrad(penetration, sandRadius) // d(penetration)/dp = - d|dp|/dp
			// Collision (no penetration) constraint.
			// This is basically sum of linear repulsion and parabolic adhesion.
			// See https://docs.google.com/drawings/d/1me1s4YCbvSEOfAmPU4O_ZVZMv3w-k9KrmjdBdqibf3g/edit?usp=sharing
			// for diagram.
			fNormal := penetration*sandStiffness - adhesionConstraintV
			cs = append(cs, Constraint{
				Value: fNormal,
				Grads: []CGrad{
					CGrad{
						grainIndex: ixOther,
						grad:       dirNormal.MultS(sandStiffness - adhesionGrad),
					},
					CGrad{
						grainIndex: ixTarget,
						grad:       dirNormal.MultS(-sandStiffness + adhesionGrad),
					},
				},
			})

			// Tangential friction constraint.
			// Both max static friction & dynamic friction are proportional to
			// force along normal (collision).
			dv := world.Grains[ixTarget].positionNew.Sub(world.Grains[ixTarget].Position).Sub(
				world.Grains[ixOther].positionNew.Sub(world.Grains[ixOther].Position))
			dirTangent := dv.ProjectOnPlane(dp)
			dirTangentLen := dirTangent.Length()

			dvLen := dv.Length()
			if dirTangentLen > 1e-6 && dvLen > 1e-6 {
				dirTangent := dirTangent.MultS(1 / dirTangentLen)
				fTangent := dvLen // Static friction by default.
				if fTangent >= fNormal*frictionStatic {
					// Switch to dynamic friction if force is too large.
					fTangent = fNormal * frictionDynamic
					if fTangent >= dvLen {
						log.Panicln("Dynamic friction condition breached")
					}
				}
				gradsT := []CGrad{
					CGrad{grainIndex: ixOther, grad: dirTangent.MultS(-fTangent - adhesionConstraintV)},
					CGrad{grainIndex: ixTarget, grad: dirTangent.MultS(fTangent + adhesionConstraintV)},
				}
				cs = append(cs, Constraint{
					Value: fTangent + adhesionConstraintV,
					Grads: gradsT,
				})
			}

		}
		return cs
	}
}

// Call this before every Step().
func (world *GrainChunk) IncorporateAddition(inGrains []*Grain) {
	// Emit new particles & accept incoming grains.
	for _, source := range world.Sources {
		world.Grains = append(world.Grains, source.MaybeEmit(world.Timestamp)...)
	}
	world.Grains = append(world.Grains, inGrains...)
}

// Take environmental grains (outside chunk) and wall configuration and
// calculate single step using very stable position-based dynamics. Returns
// grains that escaped this chunk. (they'll be removed from world)
func (world *GrainChunk) Step(envGrains []*Grain, wall *ChunkWall) []*Grain {
	// Biological / chemical process (might emit new grain when dividing).
	var newGrains []*Grain
	for _, grain := range world.Grains {
		if grain.Kind != api.Grain_CELL {
			newGrains = append(newGrains, grain)
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
				halfEnergy := grain.CellProp.Energy / 2
				grain.CellProp.Cycle.IsDividing = false
				grain.CellProp.Energy = halfEnergy
				newGrains = append(newGrains, grain.CloneCell())
			}
		} else {
			if grain.CellProp.Quals["zd"] > 0 {
				grain.CellProp.Cycle.IsDividing = true
				grain.CellProp.Cycle.DivisionCount = 0
			}
		}
		grain.CellProp.Energy -= 1

		// Remove from world when energy level is out-of-normal.
		if grain.CellProp.Energy <= 0 || grain.CellProp.Energy >= 10000 {
			log.Printf("Cell %d died (energy=%d)", grain.Id, grain.CellProp.Energy)
			continue
		}
		newGrains = append(newGrains, grain)
	}
	world.Grains = newGrains

	// We won't add / remove grains in this Step anymore, so we can safely append env grains
	// which will be removed later in Step.
	world.Grains = append(world.Grains, envGrains...)

	// Apply gravity & velocity.
	for _, grain := range world.Grains {
		grain.positionNew = grain.Position.Add(grain.Velocity.MultS(dt)).Add(world.gravityAccel.MultS(0.5 * dt * dt))
	}
	// Index spatially.
	neighbors := world.IndexNeighbors(h)

	if world.checkErrors {
		world.verifyFinite("before iteration")

		// Common error case: multiple grains at same position.
		for ixSelf, ixsN := range neighbors {
			for _, ixN := range ixsN {
				if ixN == ixSelf {
					continue
				}
				distSq := world.Grains[ixSelf].Position.Sub(world.Grains[ixN].Position).LengthSq()
				if distSq < 1e-10 {
					log.Panicf("Different grains at same position: distSq=%f [%d]=%# v and [%d]=%# v",
						distSq,
						ixSelf, pretty.Formatter(world.Grains[ixSelf]),
						ixN, pretty.Formatter(world.Grains[ixN]))
				}
			}
		}
	}
	// Iteratively resolve collisions & constraints.
	for iter := 0; iter < numIter; iter++ {
		for ix, _ := range world.Grains {
			constraints := world.ConstraintsFor(neighbors, ix)
			for ixConst, constraint := range constraints {
				var gradLengthSq float32
				for _, grad := range constraint.Grads {
					gradLengthSq += grad.grad.LengthSq()
				}
				scale := -constraint.Value / (gradLengthSq + cfmEpsilon)

				for ixGrad, grad := range constraint.Grads {
					world.Grains[grad.grainIndex].positionNew = world.Grains[grad.grainIndex].positionNew.Add(grad.grad.MultS(scale))
					if world.checkErrors && !isFiniteVec(world.Grains[grad.grainIndex].positionNew) {
						neighborDump := ""
						for _, neighborIx := range neighbors[ix] {
							if neighborIx == ix {
								continue
							}
							neighborDump += fmt.Sprintf("neighbor:grain[%d]=%# v\n", neighborIx, pretty.Formatter(world.Grains[neighborIx]))
						}
						log.Panicf("positionNew is NaN timestamp:iter:ixConst:ixGrad=%d:%d:%d:%d grainIx=%d\ngrain=%# v\nconstraints=%# v\nneighbors[grainIx]=%#v\n%s==================",
							world.Timestamp, iter, ixConst, ixGrad,
							ix, pretty.Formatter(world.Grains[grad.grainIndex]), pretty.Formatter(constraints), neighbors[ix], neighborDump)
					}
				}
			}
		}
		if world.checkErrors {
			world.verifyFinite(fmt.Sprintf("after constraint solving in iteration %d", iter))
		}

		// Box collision & floor friction.
		for _, grain := range world.Grains {
			if grain.positionNew.X < 0 {
				if wall.Xm {
					grain.positionNew.X *= -reflectionCoeff
				}
			} else if grain.positionNew.X > 1 {
				if wall.Xp {
					grain.positionNew.X = 1 - (grain.positionNew.X-1)*reflectionCoeff
				}
			}
			if grain.positionNew.Y < 0 {
				if wall.Ym {
					grain.positionNew.Y *= -reflectionCoeff
				}
			} else if grain.positionNew.Y > 1 {
				if wall.Yp {
					grain.positionNew.Y = 1 - (grain.positionNew.Y-1)*reflectionCoeff
				}
			}
			if grain.positionNew.Z < 0 {
				dz := -grain.positionNew.Z * (1 + reflectionCoeff)
				grain.positionNew.Z += dz

				dxy := grain.positionNew.Sub(grain.Position).ProjectOnPlane(Vec3f{0, 0, 1})
				dxyLen := dxy.Length()
				if dxyLen < dz*floorStatic {
					// Static friction.
					grain.positionNew.X = grain.Position.X
					grain.positionNew.Y = grain.Position.Y
				} else {
					// Dynamic friction.
					dxyCapped := dxyLen
					if dxyCapped > dz*floorDynamic {
						dxyCapped = dz * floorDynamic
					}
					grain.positionNew = grain.positionNew.Sub(dxy.Normalized().MultS(dxyCapped))
				}
			}
		}
		if world.checkErrors {
			world.verifyFinite(fmt.Sprintf("after collision/friction in iteration %d", iter))
		}
	}
	// Force no-escape-through-wall.
	for _, grain := range world.Grains {
		if grain.positionNew.X < 0 {
			if wall.Xm {
				grain.positionNew.X *= -reflectionCoeff
			}
		} else if grain.positionNew.X > 1 {
			if wall.Xp {
				grain.positionNew.X = 1 - (grain.positionNew.X-1)*reflectionCoeff
			}
		}
		if grain.positionNew.Y < 0 {
			if wall.Ym {
				grain.positionNew.Y *= -reflectionCoeff
			}
		} else if grain.positionNew.Y > 1 {
			if wall.Yp {
				grain.positionNew.Y = 1 - (grain.positionNew.Y-1)*reflectionCoeff
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
	// This check is super important, so check regardless of flag.
	world.verifyFinite("after v/p update")

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
	numInvalidGrains := 0
	var invalidSample *Grain
	for _, grain := range world.Grains {
		if !isFiniteVec(grain.Velocity) || !isFiniteVec(grain.Position) || !isFiniteVec(grain.positionNew) {
			numInvalidGrains++
			invalidSample = grain
		}
	}
	if numInvalidGrains > 0 {
		log.Panicf("NaN observed in %# v at T=%d, %s; %d of %d grains contained NaN",
			pretty.Formatter(invalidSample), world.Timestamp, context, numInvalidGrains, len(world.Grains))
	}
}

func isFiniteVec(v Vec3f) bool {
	return !(math.IsNaN(float64(v.X)) || math.IsNaN(float64(v.Y)) || math.IsNaN(float64(v.Z)) ||
		math.IsInf(float64(v.X), 0) || math.IsInf(float64(v.Y), 0) || math.IsInf(float64(v.Z), 0))
}
