package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime/pprof"
	"time"
)

// Global world config.
const dt = 1 / 30.00

// Global simulation config.
const floor_static = 0.7
const floor_dynamic = 0.5
const cfm_epsilon = 1e-3
const sand_water_equiv = 0.3

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
	isWater           bool
	totalNum          int
	framesPerParticle int
	basePositions     []Vec3f
}

func NewParticleSource(isWater bool, totalNum int, centerPos Vec3f) *ParticleSource {
	return &ParticleSource{
		isWater:           isWater,
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
			NewParticleSource(true, 300, Vec3f{0.5, 0.5, 2.0}),
			NewParticleSource(false, 300, Vec3f{0.1, 0.1, 1.0}),
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
						if world.Grains[gIndex].PositionNew.Sub(grain.PositionNew).LengthSq() < h*h {
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
			weight := SphKernel(world.Grains[ixTarget].PositionNew.Sub(world.Grains[ixOther].PositionNew), h)
			acc += weight * mass_grain * equiv
		}
		return acc
	}

	density_constraint_deriv := func() []CGrad {
		if !world.Grains[ixTarget].IsWater {
			log.Fatal("gradient of density is only defined for water")
		}
		grads := make([]CGrad, 0, len(neighbors[ixTarget])-1)
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
							world.Grains[ixOther].PositionNew.Sub(world.Grains[ixTarget].PositionNew),
							h).MultS(other_equiv))
				} else if ixDeriv == ixTarget {
					gradAccum = gradAccum.Add(
						SphKernelGrad(
							world.Grains[ixTarget].PositionNew.Sub(world.Grains[ixOther].PositionNew),
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

	if world.Grains[ixTarget].IsWater {
		return []Constraint{
			Constraint{
				Value: density()/density_base - 1,
				Grads: density_constraint_deriv(),
			},
		}
	} else {
		cs := make([]Constraint, 0, len(neighbors[ixTarget]))
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
			dp := world.Grains[ixTarget].PositionNew.Sub(world.Grains[ixOther].PositionNew)
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
				dv := world.Grains[ixTarget].PositionNew.Sub(world.Grains[ixTarget].Position).Sub(
					world.Grains[ixOther].PositionNew.Sub(world.Grains[ixOther].Position))
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

// Position-based dynamics.
func (world *GrainWorld) Step() {
	accel := Vec3f{0, 0, -1}

	// Emit new particles.
	for _, source := range world.Sources {
		world.Grains = append(world.Grains, source.MaybeEmit(world.Timestamp)...)
	}
	// Apply gravity & velocity.
	for _, grain := range world.Grains {
		grain.PositionNew = grain.Position.Add(grain.Velocity.MultS(dt)).Add(accel.MultS(0.5 * dt * dt))
	}
	// Index spatially.
	neighbors := world.IndexNeighbors(h)

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
					world.Grains[grad.grainIndex].PositionNew = world.Grains[grad.grainIndex].PositionNew.Add(grad.grad.MultS(scale))
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

				dxy := grain.PositionNew.Sub(grain.Position).ProjectOnPlane(Vec3f{0, 0, 1})
				dxyLen := dxy.Length()
				if dxyLen < dz*floor_static {
					// Static friction.
					grain.PositionNew.X = grain.Position.X
					grain.PositionNew.Y = grain.Position.Y
				} else {
					// Dynamic friction.
					dxy_capped := dxyLen
					if dxy_capped > dz*floor_dynamic {
						dxy_capped = dz * floor_dynamic
					}
					grain.PositionNew = grain.PositionNew.Sub(dxy.Normalized().MultS(dxy_capped))
				}
			}
		}
	}

	// Actually update velocity & position.
	// PositionNew is destroyed after this.
	for _, grain := range world.Grains {
		grain.Velocity = grain.PositionNew.Sub(grain.Position).MultS(1.0 / dt)
		grain.Position = grain.PositionNew
	}

	world.Timestamp++
}

// Run chunk benchmark and output to local files.
func Benchmark() {
	t0 := time.Now()
	world := NewGrainWorld()
	steps := 300 * 4
	for iter := 0; iter < steps; iter++ {
		world.Step()
	}
	log.Printf("Benchmark: %.3fs for %d steps\n", float64(time.Since(t0))*1e-9, steps)

	log.Printf("Profiling\n")
	world = NewGrainWorld()
	f, err := os.Create("chunk-grains-benchmark.prof")
	if err != nil {
		log.Fatal("Failed to output profile file")
	}
	pprof.StartCPUProfile(f)
	for iter := 0; iter < steps; iter++ {
		world.Step()
	}
	pprof.StopCPUProfile()
	log.Printf("Done\n")

	log.Printf("Dumping\n")

	f, err = os.Create("./client/grains-dump.json")
	if err != nil {
		log.Fatal("Failed to open dump file")
	}
	w := bufio.NewWriter(f)
	defer w.Flush()
	w.WriteString("[")
	world = NewGrainWorld()
	for iter := 0; iter < steps; iter++ {
		world.Step()
		w.WriteString("[")
		for ix, grain := range world.Grains {
			w.WriteString("{")
			fmt.Fprintf(w, "\"is_water\": %t,", grain.IsWater)
			fmt.Fprintf(w, "\"position\": [%f, %f, %f]",
				grain.Position.X, grain.Position.Y, grain.Position.Z)
			w.WriteString("}")
			if ix == len(world.Grains)-1 {
				w.WriteString("]")
			} else {
				w.WriteString(",")
			}
		}
		if iter == steps-1 {
			w.WriteString("]\n")
		} else {
			w.WriteString(",\n")
		}
	}
	log.Print("Done\n")
}