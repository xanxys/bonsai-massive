package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"
)

// 4 bit field in Lattice Gas Automata.
// (X-, X+, Y-, Y+)
// LSB
type Cell int

type GasLattice struct {
	N        int
	Timestep int64
	grid     map[string]Cell
}

func NewGrid(n int) *GasLattice {
	grid := make(map[string]Cell)
	for ix := 0; ix < n; ix++ {
		for iy := 0; iy < n; iy++ {
			key := fmt.Sprintf("%d:%d", ix, iy)
			if ix < n/10 && n/10*4 < iy && iy < n/10*6 {
				grid[key] = Cell(2)
			} else {
				if rand.Float32() < 0.1 {
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

/*
class CellRepo {
    private repo : any;
    private memoize_count : number;

    constructor() {
        this.repo = {};
        this.memoize_count = 0;
    }

    public memoized(cell : HashGasCell) : HashGasCell {
        this.memoize_count++;
        if (this.memoize_count % 5000 === 0) {
            console.log(
                "HashRepoSize:", _.size(this.repo),
                "MemoizeAccess:", this.memoize_count);
        }
        var key = cell.hash();
        if (this.repo[key] !== undefined) {
            if (cell.level !== this.repo[key].level) {
                console.log("Collision!", key, cell, this.repo[key]);
                return null;
            }
            return this.repo[key];
        } else {
            this.repo[key] = cell;
            return cell;
        }
    }
}

var repo = new CellRepo();

*/

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
}

func NewLeaf(repo *HashCellRepo, s Cell) *HashGasCell {
	repo.Memoize()
	return &HashGasCell{
		Level:     0,
		Value:     s,
		c00:       nil,
		c10:       nil,
		c01:       nil,
		c11:       nil,
		cacheHash: nil,
		cacheNext: nil,
	}
}

func NewNode(repo *HashCellRepo, c00, c10, c01, c11 *HashGasCell) *HashGasCell {
	repo.Memoize()
	return &HashGasCell{
		Level:     c00.Level + 1,
		c00:       c00,
		c10:       c10,
		c01:       c01,
		c11:       c11,
		cacheHash: nil,
		cacheNext: nil,
	}
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
		fmt.Panic("strange level")
	}
	if cell.Level == 2 {
		// Normal execution of 1 step.
		C00 := newLeaf(repo, Step1Ext(
			c00.c01.Value, c10.c01.Value, c00.c10.Value, c01.c10.Value, c00.c11.Value))
		C10 := newLeaf(repo, Step1Ext(
			c00.c11.Value, c10.c11.Value, c10.c00.Value, c11.c00.Value, c10.c01.Value))
		C01 := newLeaf(repo, Step1Ext(
			c01.c00.Value, c11.c00.Value, c00.c11.Value, c01.c11.Value, c01.c10.Value))
		C11 := newLeaf(repo, Step1Ext(
			c01.c10.Value, c11.c10.Value, c10.c01.Value, c11.c01.Value, c11.c00.Value))
		return newNode(repo, C00, C10, C01, C11)
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
		t00 := c00.nextExp(repo)
		t10 := HashGasCell.newNode(repo, c00.c10, c10.c00, c00.c11, c10.c01).nextExp(repo)
		t20 := c10.nextExp(repo)
		t01 := HashGasCell.newNode(repo, c00.c01, c00.c11, c01.c00, c01.c10).nextExp(repo)
		t11 := HashGasCell.newNode(repo, c00.c11, c10.c01, c01.c10, c11.c00).nextExp(repo)
		t21 := HashGasCell.newNode(repo, c10.c01, c10.c11, c11.c00, c11.c10).nextExp(repo)
		t02 := c01.nextExp(repo)
		t12 := HashGasCell.newNode(repo, c01.c10, c11.c00, c01.c11, c11.c01).nextExp(repo)
		t22 := c11.nextExp(repo)

		// Step another half.
		//  t02 t12 t22      ^
		//  t01 t11 t21 ^    _c*1'
		//  t00 t10 t20 _c*0'
		//  | c0*' |
		//      | c1*' |
		C00 := newNode(repo, t00, t10, t01, t11).nextExp(repo)
		C10 := newNode(repo, t10, t20, t11, t21).nextExp(repo)
		C01 := newNode(repo, t01, t11, t02, t12).nextExp(repo)
		C11 := newNode(repo, t11, t21, t12, t22).nextExp(repo)

		return HashGasCell.newNode(C00, C10, C01, C11)
	}
}

func (cell *HashGasCell) Ref(x int, y int) *HashGasCell {
	if cell.Level == 0 {
		if x != 0 || y != 0 {
			fmt.Panic("Invalid coordinate")
		}
		return cell
	} else {
		h_sz := 1 << (cell.Level - 1)
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
				binary.Write(h, binary.BigEndian(), child.Hash())
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
	sDY int

	repo *HashCellRepo
}

func newHashGasLattice(lattice *GasLattice) *HashGasLattice {
	level := Math.ceil(Math.log(n) / Math.log(2))
	fn := func(ix int, iy int) {
		if ix < 0 || iy < 0 || ix >= n || iy >= n {
			return Cell(-1)
		} else {
			return lattice.At(ix, iy)
		}
	}

	return &HashGasLattice{
		N:        lattice.N,
		Timestep: 0,
		sX0:      0,
		sY0:      0,
		sDx:      lattice.N,
		sDy:      lattice.N,
		snapshot: Recreate(level, fn),
	}
}

func (lattice *HashGasLattice) StepN() {
	this.upgrade()
	var dt = Math.pow(2, this.snapshot.level-2)
	var q_sz = dt
	this.snapshot = this.snapshot.nextExp()
	this.s_x0 -= q_sz
	this.s_y0 -= q_sz
	this.timestep += dt
}

// Ensure snapshot & root is steppable, and stepped smaller cell contains
// region of interest by inserting boundaries properly.
//
//    |L0|  |
//    | L1  |
//|  L2     |
// ... (alternate)
func (lattice *HashGasLattice) upgrade() {
	sz := 1 << lattice.Snapshot.Level
	if !(0 <= this.s_x0 && 0 <= this.s_y0) {
		fmt.Panic("Impossible to upgrade")
	}
	if !(this.s_x0+this.s_dx <= sz) {
		fmt.Panic("Impossible to upgrade")
	}
	if !(this.s_y0+this.s_dy <= sz) {
		fmt.Panic("Impossible to upgrade")
	}

	for !lattice.isSteppable() {
		e := getWall(lattice.Snapshot.Level)
		if lattice.Snapshot.Level%2 == 0 {
			lattice.Snapshot = newNode(lattice.repo, lattice.Snapshot, e, e, e)
		} else {
			eSize := 1 << lattice.Ssnapshot.level
			lattice.Snapshot = newNode(lattice.repo, e, e, e, this.snapshot)
			lattice.sX0 += eSize
			lattice.sY0 += eSize
		}
	}
}

func (lattice *HashGasLattice) isSteppable() bool {
	if lattice.Snapshot.level < 2 {
		return false
	}
	q_sz := 1 << (lattice.Snapshot.Level - 2)
	return
	q_sz <= lattice.s_x0 &&
		q_sz <= lattice.s_y0 &&
		lattice.s_x0+lattice.s_dx <= q_sz*3 &&
		lattice.s_y0+lattice.s_dy <= q_sz*3
}

func (lattice *HashGasLattice) Recreate(level int, fn func(int, int) Cell) *HashGasCell {
	return lattice.recreateAux(level, fn, 0, 0)
}

func (lattice *HashGasLattice) recreateAux(level int, fn func(int, int) Cell, dx int, dy int) *HashGasCell {
	if level == 0 {
		return newLeaf(lattice.repo, fn(dx, dy))
	} else {
		h_sz := 1 << (level - 1)
		c00 := lattice.recreateAux(level-1, fn, dx+0, dy+0)
		c10 := lattice.recreateAux(level-1, fn, dx+h_sz, dy+0)
		c01 := lattice.recreateAux(level-1, fn, dx+0, dy+h_sz)
		c11 := lattice.recreateAux(level-1, fn, dx+h_sz, dy+h_sz)
		return newNode(lattice.repo, c00, c10, c01, c11)
	}
}

func getWall(level int) *HashGasCell {
	if level == 0 {
		return HashGasCell.newLeaf(-1)
	} else {
		var e = HashGasLattice.getEmpty(level - 1)
		return HashGasCell.newNode(e, e, e, e)
	}
}

func (lattice *HashGasLattice) At(ix int, iy int) Cell {
	return lattice.snapshot.Ref(this.s_x0+ix, this.s_y0+iy)
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
