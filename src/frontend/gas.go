package main

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
)

// 4 bit field in Lattice Gas Automata.
// (X-, X+, Y-, Y+)
// LSB
type Cell int

type Lattice interface {
	GetN() int
	At(ix int, iy int) Cell
}

type GasLattice struct {
	N        int
	Timestep int64
	grid     map[string]Cell
}

func NewGasLattice(n int, temperature float64) *GasLattice {
	grid := make(map[string]Cell)
	for ix := 0; ix < n; ix++ {
		for iy := 0; iy < n; iy++ {
			key := fmt.Sprintf("%d:%d", ix, iy)
			if ix < n/10 && n/10*4 < iy && iy < n/10*6 {
				grid[key] = Cell(2)
			} else {
				if rand.Float64() < temperature {
					grid[key] = Cell(1 + rand.Int31n(15))
				} else {
					grid[key] = 0
				}
			}
		}
	}
	return &GasLattice{
		N:        n,
		Timestep: 0,
		grid:     grid,
	}
}

func (lattice *GasLattice) GetN() int {
	return lattice.N
}

func (lattice *GasLattice) Step() {
	new_grid := make(map[string]Cell)
	for ix := 0; ix < lattice.N; ix++ {
		for iy := 0; iy < lattice.N; iy++ {
			key := fmt.Sprintf("%d:%d", ix, iy)
			s := 0
			// X- flow
			if ix < lattice.N-1 {
				if lattice.At(ix+1, iy)&1 != 0 {
					s |= 1
				}
			} else {
				if lattice.At(ix, iy)&2 != 0 {
					s |= 1
				}
			}
			// X+ flow
			if ix > 0 {
				if lattice.At(ix-1, iy)&2 != 0 {
					s |= 2
				}
			} else {
				if lattice.At(ix, iy)&1 != 0 {
					s |= 2
				}
			}
			// Y- flow
			if iy < lattice.N-1 {
				if lattice.At(ix, iy+1)&4 != 0 {
					s |= 4
				}

			} else {
				if lattice.At(ix, iy)&8 != 0 {
					s |= 4
				}
			}
			// Y+ flow
			if iy > 0 {
				if lattice.At(ix, iy-1)&8 != 0 {
					s |= 8
				}
			} else {
				if lattice.At(ix, iy)&4 != 0 {
					s |= 8
				}
			}

			// Collision handling.
			if s == 3 {
				s = 12
			} else if s == 12 {
				s = 3
			}
			new_grid[key] = Cell(s)
		}
	}
	lattice.grid = new_grid
	lattice.Timestep++
}

func (lattice *GasLattice) At(ix int, iy int) Cell {
	return lattice.grid[fmt.Sprintf("%d:%d", ix, iy)]
}

type HashGasCell struct {
	// 2 ^ level = size
	// e.g. 0: 1 (base)
	Level int

	// Leaf value (only when level == 0)
	Value Cell

	// Children (only when level >= 1)
	// Y^
	//  |c01 c11
	//  |c00 c10
	// -|--------> X
	c00 *HashGasCell
	c10 *HashGasCell
	c01 *HashGasCell
	c11 *HashGasCell

	cacheHash uint64
	cacheNext *HashGasCell
}

type HashCellRepo struct {
	repo map[uint64]*HashGasCell
}

func NewHashCellRepo() *HashCellRepo {
	return &HashCellRepo{
		repo: make(map[uint64]*HashGasCell),
	}
}

func (repo *HashCellRepo) PrintStat() {
	fmt.Printf("Repo size=%d\n", len(repo.repo))

	counts := make(map[int]int)
	for _, cell := range repo.repo {
		_, exist := counts[cell.Level]
		if exist {
			counts[cell.Level]++
		} else {
			counts[cell.Level] = 1
		}
	}

	levels := make([]int, 0)
	for level, _ := range counts {
		levels = append(levels, level)
	}
	sort.Ints(levels)
	for _, level := range levels {
		fmt.Printf("@%d: %d\n", level, counts[level])
	}
}

func (repo *HashCellRepo) Memoize(cell *HashGasCell) *HashGasCell {
	h := cell.Hash()
	existing, ok := repo.repo[h]
	if ok {
		return existing
	} else {
		repo.repo[h] = cell
		return cell
	}
}

func NewLeaf(repo *HashCellRepo, s Cell) *HashGasCell {
	return repo.Memoize(&HashGasCell{
		Level:     0,
		Value:     s,
		c00:       nil,
		c10:       nil,
		c01:       nil,
		c11:       nil,
		cacheHash: 0,
		cacheNext: nil,
	})
}

func NewNode(repo *HashCellRepo, c00, c10, c01, c11 *HashGasCell) *HashGasCell {
	return repo.Memoize(&HashGasCell{
		Level:     c00.Level + 1,
		c00:       c00,
		c10:       c10,
		c01:       c01,
		c11:       c11,
		cacheHash: 0,
		cacheNext: nil,
	})
}

// Get center level-1 cell after 2^(level-2) steps.
// e.g. level==2
func (cell *HashGasCell) NextExp(repo *HashCellRepo) *HashGasCell {
	if cell.cacheNext == nil {
		cell.cacheNext = cell.nextExpAux(repo)
	}
	return cell.cacheNext
}

func (cell *HashGasCell) nextExpAux(repo *HashCellRepo) *HashGasCell {
	if cell.Level < 2 {
		panic("strange level")
	}
	c00 := cell.c00
	c01 := cell.c01
	c10 := cell.c10
	c11 := cell.c11
	if cell.Level == 2 {
		// Normal execution of 1 step.
		C00 := NewLeaf(repo, Step1Ext(
			c00.c01.Value, c10.c01.Value, c00.c10.Value, c01.c10.Value, c00.c11.Value))
		C10 := NewLeaf(repo, Step1Ext(
			c00.c11.Value, c10.c11.Value, c10.c00.Value, c11.c00.Value, c10.c01.Value))
		C01 := NewLeaf(repo, Step1Ext(
			c01.c00.Value, c11.c00.Value, c00.c11.Value, c01.c11.Value, c01.c10.Value))
		C11 := NewLeaf(repo, Step1Ext(
			c01.c10.Value, c11.c10.Value, c10.c01.Value, c11.c01.Value, c11.c00.Value))
		return NewNode(repo, C00, C10, C01, C11)
	} else {
		// Create intermediate cells with a half of the requires steps.
		//  _
		//  t*2  c01.c01 c01.c11 | c11.c01 c11.c11
		//    /\ c01.c00 c01.c10 | c11.c00 c11.c10
		//  -t*1 ----------------+----------------
		//    \/ c00.c01 c00.c11 | c10.c01 c10.c11
		//  t*0  c00.c00 c00.c10 | c10.c00 c10.c10
		//              |       t1*       |
		//       |     t0*       |        t2*     |
		t00 := c00.NextExp(repo)
		t10 := NewNode(repo, c00.c10, c10.c00, c00.c11, c10.c01).NextExp(repo)
		t20 := c10.NextExp(repo)
		t01 := NewNode(repo, c00.c01, c00.c11, c01.c00, c01.c10).NextExp(repo)
		t11 := NewNode(repo, c00.c11, c10.c01, c01.c10, c11.c00).NextExp(repo)
		t21 := NewNode(repo, c10.c01, c10.c11, c11.c00, c11.c10).NextExp(repo)
		t02 := c01.NextExp(repo)
		t12 := NewNode(repo, c01.c10, c11.c00, c01.c11, c11.c01).NextExp(repo)
		t22 := c11.NextExp(repo)

		// Step another half.
		//  t02 t12 t22      ^
		//  t01 t11 t21 ^    _c*1'
		//  t00 t10 t20 _c*0'
		//  | c0*' |
		//      | c1*' |
		C00 := NewNode(repo, t00, t10, t01, t11).NextExp(repo)
		C10 := NewNode(repo, t10, t20, t11, t21).NextExp(repo)
		C01 := NewNode(repo, t01, t11, t02, t12).NextExp(repo)
		C11 := NewNode(repo, t11, t21, t12, t22).NextExp(repo)

		return NewNode(repo, C00, C10, C01, C11)
	}
}

func (cell *HashGasCell) Ref(x int, y int) *HashGasCell {
	if cell.Level == 0 {
		if x != 0 || y != 0 {
			panic("Invalid coordinate")
		}
		return cell
	} else {
		h_sz := 1 << uint(cell.Level-1)
		if x < h_sz {
			if y < h_sz {
				return cell.c00.Ref(x, y)
			} else {
				return cell.c01.Ref(x, y-h_sz)
			}
		} else {
			if y < h_sz {
				return cell.c10.Ref(x-h_sz, y)
			} else {
				return cell.c11.Ref(x-h_sz, y-h_sz)
			}
		}
	}
}

func (cell *HashGasCell) Hash() uint64 {
	if cell.cacheHash == 0 {
		h := fnv.New64a()
		if cell.Level == 0 {
			binary.Write(h, binary.BigEndian, byte(cell.Value+1))
		} else {
			for _, child := range [4]*HashGasCell{cell.c00, cell.c10, cell.c01, cell.c11} {
				binary.Write(h, binary.BigEndian, child.Hash())
			}
		}
		cell.cacheHash = h.Sum64()
	}
	return cell.cacheHash
}

// (Hopefully) super-accelerated Gas Lattice using hashlife.
type HashGasLattice struct {
	N int

	// Current time.
	Timestep uint64

	Snapshot *HashGasCell

	// Region of interest.
	sX0 int
	sY0 int
	sDx int
	sDy int

	repo *HashCellRepo
}

func NewHashGasLattice(lattice *GasLattice) *HashGasLattice {
	level := int(math.Ceil(math.Log2(float64(lattice.N))))
	fn := func(ix int, iy int) Cell {
		if ix < 0 || iy < 0 || ix >= lattice.N || iy >= lattice.N {
			return Cell(-1)
		} else {
			return lattice.At(ix, iy)
		}
	}

	repo := NewHashCellRepo()
	return &HashGasLattice{
		repo:     repo,
		N:        lattice.N,
		Timestep: 0,
		sX0:      0,
		sY0:      0,
		sDx:      lattice.N,
		sDy:      lattice.N,
		Snapshot: Recreate(repo, level, fn),
	}
}

func (lattice *HashGasLattice) StepN() {
	lattice.upgrade()
	dt := 1 << uint(lattice.Snapshot.Level-2)
	q_sz := dt
	lattice.Snapshot = lattice.Snapshot.NextExp(lattice.repo)
	lattice.sX0 -= q_sz
	lattice.sY0 -= q_sz
	lattice.Timestep += uint64(dt)
}

// Ensure snapshot & root is steppable, and stepped smaller cell contains
// region of interest by inserting boundaries properly.
//
//    |L0|  |
//    | L1  |
//|  L2     |
// ... (alternate)
func (lattice *HashGasLattice) upgrade() {
	sz := 1 << uint(lattice.Snapshot.Level)
	if !(0 <= lattice.sX0 && 0 <= lattice.sY0) {
		panic("Impossible to upgrade")
	}
	if !(lattice.sX0+lattice.sDx <= sz) {
		panic("Impossible to upgrade")
	}
	if !(lattice.sY0+lattice.sDy <= sz) {
		panic("Impossible to upgrade")
	}

	for !lattice.isSteppable() {
		e := getWall(lattice.repo, lattice.Snapshot.Level)
		if lattice.Snapshot.Level%2 == 0 {
			lattice.Snapshot = NewNode(lattice.repo, lattice.Snapshot, e, e, e)
		} else {
			eSize := 1 << uint(lattice.Snapshot.Level)
			lattice.Snapshot = NewNode(lattice.repo, e, e, e, lattice.Snapshot)
			lattice.sX0 += eSize
			lattice.sY0 += eSize
		}
	}
}

func (lattice *HashGasLattice) PrintStat() {
	lattice.repo.PrintStat()
}

func (lattice *HashGasLattice) isSteppable() bool {
	if lattice.Snapshot.Level < 2 {
		return false
	}
	q_sz := 1 << uint(lattice.Snapshot.Level-2)
	return q_sz <= lattice.sX0 &&
		q_sz <= lattice.sY0 &&
		lattice.sX0+lattice.sDx <= q_sz*3 &&
		lattice.sY0+lattice.sDy <= q_sz*3
}

func Recreate(repo *HashCellRepo, level int, fn func(int, int) Cell) *HashGasCell {
	return recreateAux(repo, uint(level), fn, 0, 0)
}

func recreateAux(repo *HashCellRepo, level uint, fn func(int, int) Cell, dx, dy int) *HashGasCell {
	if level == 0 {
		return NewLeaf(repo, fn(dx, dy))
	} else {
		h_sz := 1 << (level - 1)
		c00 := recreateAux(repo, level-1, fn, dx+0, dy+0)
		c10 := recreateAux(repo, level-1, fn, dx+h_sz, dy+0)
		c01 := recreateAux(repo, level-1, fn, dx+0, dy+h_sz)
		c11 := recreateAux(repo, level-1, fn, dx+h_sz, dy+h_sz)
		return NewNode(repo, c00, c10, c01, c11)
	}
}

func getWall(repo *HashCellRepo, level int) *HashGasCell {
	if level == 0 {
		return NewLeaf(repo, -1)
	} else {
		e := getWall(repo, level-1)
		return NewNode(repo, e, e, e, e)
	}
}

func (lattice *HashGasLattice) At(ix int, iy int) Cell {
	return lattice.Snapshot.Ref(lattice.sX0+ix, lattice.sY0+iy).Value
}

// First, we extend the CA to include walls as one of state.
// We denote wall as -1, and now have 17 states.
func Step1Ext(l, r, b, t, self Cell) Cell {
	// A wall is forver a wall.
	if self == -1 {
		return Cell(-1)
	}
	// Stream, wall-reflection.
	s := 0
	if l == -1 {
		if self&1 != 0 {
			s |= 2
		}
	} else {
		if l&2 != 0 {
			s |= 2
		}
	}
	if r == -1 {
		if self&2 != 0 {
			s |= 1
		}
	} else {
		if r&1 != 0 {
			s |= 1
		}
	}
	if b == -1 {
		if self&4 != 0 {
			s |= 8
		}
	} else {
		if b&8 != 0 {
			s |= 8
		}
	}
	if t == -1 {
		if self&8 != 0 {
			s |= 4
		}
	} else {
		if t&4 != 0 {
			s |= 4
		}
	}
	// Collision.
	if s == 3 {
		s = 12
	} else if s == 12 {
		s = 3
	}
	return Cell(s)
}

func (lattice *HashGasLattice) GetN() int {
	return lattice.N
}
