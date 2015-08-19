declare var $;
declare var _;

class Cell {
    // 4 bit field
    // (X-, X+, Y-, Y+)
    // LSB
    public state : number;

    constructor(state : number) {
        this.state = state;
    }
}

class GasLattice {
    public n : number;
    public timestep : number;
    private grid;

    constructor(n : number) {
        this.n = n;
        var grid = {};
        _.map(_.range(n), function(ix) {
            _.map(_.range(n), function(iy) {
                var cell : Cell;
                if (ix < n * 0.1 && n * 0.4 < iy && iy < n * 0.6) {
                    cell = new Cell(2);
                } else {
                    if (Math.random() < 0.1) {
                        cell = new Cell(Math.floor(Math.random() * 15));
                    } else {
                        cell = new Cell(0);
                    }
                }

                var key = ix + ":" + iy;
                grid[key] = cell;
            });
        });
        this.grid = grid;
        this.timestep = 0;
    }

    public step() {
        var new_grid = {};
        var grid = this.grid;
        var n = this.n;
        _.map(_.range(n), function(ix) {
            _.map(_.range(n), function(iy) {
                var key = ix + ":" + iy;
                var s = 0;
                // X- flow
                if (ix < n - 1) {
                    s |= (grid[(ix + 1) + ":" + iy].state & 1) ? 1 : 0;
                } else {
                    s |= (grid[key].state & 2) ? 1 : 0;
                }
                // X+ flow
                if (ix > 0) {
                    s |= (grid[(ix - 1) + ":" + iy].state & 2) ? 2 : 0;
                } else {
                    s |= (grid[key].state & 1) ? 2 : 0;
                }
                // Y- flow
                if (iy < n - 1) {
                    s |= (grid[ix + ":" + (iy + 1)].state & 4) ? 4 : 0;
                } else {
                    s |= (grid[key].state & 8) ? 4 : 0;
                }
                // Y+ flow
                if (iy > 0) {
                    s |= (grid[ix + ":" + (iy - 1)].state & 8) ? 8 : 0;
                } else {
                    s |= (grid[key].state & 4) ? 8 : 0;
                }

                // Collision handling.
                if (s === 3) {
                    s = 12;
                } else if (s === 12) {
                    s = 3;
                }
                new_grid[key] = new Cell(s);
            });
        });
        this.grid = new_grid;
        this.timestep++;
    }

    public at(ix : number, iy : number) : Cell {
        var key : string = ix + ":" + iy;
        return this.grid[key];
    }
}

class HashGasCell {
    // 2 ^ level = size
    // e.g. 0: 1 (base)
    public level : number;

    // Leaf value (only when level == 0)
    public v : number;

    // Children (only when level >= 1)
    // Y^
    //  |c01 c11
    //  |c00 c10
    // -|--------> X
    public c00 : HashGasCell;
    public c10 : HashGasCell;
    public c01 : HashGasCell;
    public c11 : HashGasCell;

    public static newLeaf(v : number) : HashGasCell {
        var c = new HashGasCell();
        c.level = 0;
        c.v = v;
        return c;
    }

    public static newNode(c00 : HashGasCell, c10 : HashGasCell, c01 : HashGasCell, c11 : HashGasCell) : HashGasCell {
        var c = new HashGasCell();
        c.level = c00.level + 1;
        c.c00 = c00;
        c.c10 = c10;
        c.c01 = c01;
        c.c11 = c11;
        return c;
    }

    // Get center level-1 cell after 2^(level-2) steps.
    // e.g. level==2
    public nextExp() : HashGasCell {
        console.assert(this.level >= 2);
        var c00 = this.c00;
        var c10 = this.c10;
        var c01 = this.c01;
        var c11 = this.c11;
        if (this.level === 2) {
            // Normal execution of 1 step.
            var C00 = HashGasCell.newLeaf(HashGasLattice.step1(
                c00.c01.v, c10.c01.v, c00.c10.v, c01.c10.v, c00.c11.v));
            var C10 = HashGasCell.newLeaf(HashGasLattice.step1(
                c00.c11.v, c10.c11.v, c10.c00.v, c11.c00.v, c10.c01.v));
            var C01 = HashGasCell.newLeaf(HashGasLattice.step1(
                c01.c00.v, c11.c00.v, c00.c11.v, c01.c11.v, c01.c10.v));
            var C11 = HashGasCell.newLeaf(HashGasLattice.step1(
                c01.c10.v, c11.c10.v, c10.c01.v, c11.c01.v, c11.c00.v));
            return HashGasCell.newNode(C00, C10, C01, C11);
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
            var t00 = c00.nextExp();
            var t10 = HashGasCell.newNode(c00.c10, c10.c00, c00.c11, c10.c01).nextExp();
            var t20 = c10.nextExp();
            var t01 = HashGasCell.newNode(c00.c01, c00.c11, c01.c00, c01.c10).nextExp();
            var t11 = HashGasCell.newNode(c00.c11, c10.c01, c01.c10, c11.c00).nextExp();
            var t21 = HashGasCell.newNode(c10.c01, c10.c11, c11.c00, c11.c10).nextExp();
            var t02 = c01.nextExp();
            var t12 = HashGasCell.newNode(c01.c10, c11.c00, c01.c11, c11.c01).nextExp();
            var t22 = c11.nextExp();

            // Step another half.
            //  t02 t12 t22      ^
            //  t01 t11 t21 ^    _c*1'
            //  t00 t10 t20 _c*0'
            //  | c0*' |
            //      | c1*' |
            var C00 = HashGasCell.newNode(t00, t10, t01, t11).nextExp();
            var C10 = HashGasCell.newNode(t10, t20, t11, t21).nextExp();
            var C01 = HashGasCell.newNode(t01, t11, t02, t12).nextExp();
            var C11 = HashGasCell.newNode(t11, t21, t12, t22).nextExp();

            return HashGasCell.newNode(C00, C10, C01, C11);
        }
    }

    public ref(x : number, y : number) : HashGasCell {
        if (this.level === 0) {
            console.assert(x === 0 && y === 0);
            return this;
        } else {
            var h_sz = Math.pow(2, this.level - 1);
            if (x < h_sz) {
                if (y < h_sz) {
                    return this.c00.ref(x, y);
                } else {
                    return this.c01.ref(x, y - h_sz);
                }
            } else {
                if (y < h_sz) {
                    return this.c10.ref(x - h_sz, y);
                } else {
                    return this.c11.ref(x - h_sz, y - h_sz);
                }
            }
        }
    }
}

// (Hopefully) super-accelerated Gas Lattice using hashlife.
class HashGasLattice {
    public n : number;

    // Current time.
    public timestep : number;

    // Root tree at t = 0. This will get bigger if we step further
    // (to account for boundary).
    public snapshot : HashGasCell;

    // Region of interest.
    public s_x0 : number;
    public s_y0 : number;
    public s_dx : number;
    public s_dy : number;

    constructor(lattice : GasLattice) {
        var n = lattice.n;
        this.timestep = 0;
        this.s_x0 = 0;
        this.s_y0 = 0;
        this.s_dx = n;
        this.s_dy = n;
        this.n = n;

        var level = Math.ceil(Math.log(n) / Math.log(2));
        var fn = (ix, iy) => {
            if (ix < 0 || iy < 0 || ix >= n || iy >= n) {
                return -1;
            } else {
                return lattice.at(ix, iy).state;
            }
        };
        this.snapshot = HashGasLattice.recreate(level, fn);
    }

    public stepN() {
        this.upgrade();
        var dt = Math.pow(2, this.snapshot.level - 2);
        var q_sz = dt;
        this.snapshot = this.snapshot.nextExp();
        this.s_x0 -= q_sz;
        this.s_y0 -= q_sz;
        this.timestep += dt;
    }

    // Ensure snapshot & root is steppable, and stepped smaller cell contains
    // region of interest by inserting boundaries properly.
    //
    //    |L0|  |
    //    | L1  |
    //|  L2     |
    // ... (alternate)
    public upgrade() {
        var sz = Math.pow(2, this.snapshot.level);
        console.assert(0 <= this.s_x0 && 0 <= this.s_y0);
        console.assert(this.s_x0 + this.s_dx <= sz);
        console.assert(this.s_y0 + this.s_dy <= sz);

        while (!this.isSteppable()) {
            var e = HashGasLattice.getEmpty(this.snapshot.level);
            if (this.snapshot.level % 2 === 0) {
                this.snapshot = HashGasCell.newNode(this.snapshot, e, e, e);
            } else {
                var e_size = Math.pow(2, this.snapshot.level);
                this.snapshot = HashGasCell.newNode(e, e, e, this.snapshot);
                this.s_x0 += e_size;
                this.s_y0 += e_size;
            }
        }
    }

    private isSteppable() : boolean {
        if (this.snapshot.level < 2) {
            return false;
        }
        var q_sz = Math.pow(2, this.snapshot.level - 2);
        return q_sz <= this.s_x0 && q_sz <= this.s_y0
                && this.s_x0 + this.s_dx <= q_sz * 3
                && this.s_y0 + this.s_dy <= q_sz * 3;
    }

    private static recreate(level : number, fn : (x:number, y:number) => number) : HashGasCell {
        return this.recreateAux(level, fn, 0, 0);
    }

    private static recreateAux(level : number, fn : (x:number, y:number) => number, dx : number, dy : number) : HashGasCell {
        if (level === 0) {
            return HashGasCell.newLeaf(fn(dx, dy));
        } else {
            var h_sz = Math.pow(2, level - 1);
            var c00 = this.recreateAux(level - 1, fn, 0, 0);
            var c10 = this.recreateAux(level - 1, fn, h_sz, 0);
            var c01 = this.recreateAux(level - 1, fn, 0, h_sz);
            var c11 = this.recreateAux(level - 1, fn, h_sz, h_sz);
            return HashGasCell.newNode(c00, c10, c01, c11);
        }
    }

    private static getEmpty(level : number) : HashGasCell {
        if (level === 0) {
            return HashGasCell.newLeaf(-1);
        } else {
            var e = HashGasLattice.getEmpty(level - 1);
            return HashGasCell.newNode(e, e, e, e);
        }
    }

    // Compat method to get a single old Cell.
    public at(ix : number, iy : number) : Cell {
        return new Cell(this.snapshot.ref(this.s_x0 + ix, this.s_y0 + iy).v);
    }

    // First, we extend the CA to include walls as one of state.
    // We denote wall as -1, and now have 17 states.
    public static step1(
            l : number, r : number, b : number, t : number, self : number) : number {
        if (self === - 1) {
            return -1;
        }
        // Stream, wall-reflection.
        var s = 0
        if (l === -1) {
            s |= (self & 1) ? 2 : 0;
        } else {
            s |= (l & 2) ? 2 : 0;
        }
        if (r === -1) {
            s |= (self & 2) ? 1 : 0;
        } else {
            s |= (r & 1) ? 1 : 0;
        }
        if (b === -1) {
            s |= (self & 4) ? 8 : 0;
        } else {
            s |= (b & 8) ? 8 : 0;
        }
        if (t === -1) {
            s |= (self & 8) ? 4 : 0;
        } else {
            s |= (t & 4) ? 4 : 0;
        }
        // Collision.
        if (s === 3) {
            s = 12;
        } else if (s === 12) {
            s = 3;
        }
        return s;
    }

}

function test() {
    var n = 2;
    var lattice_ref = new GasLattice(n);
    var lattice = new HashGasLattice(lattice_ref);
    console.log(lattice);
    _.map(_.range(10), i_case => {
        console.log("Testing at t=", lattice_ref.timestep, "case=", i_case);
        _.map(_.range(n), ix => {
            _.map(_.range(n), iy => {
                if (lattice.at(ix, iy).state !== lattice_ref.at(ix, iy).state) {
                    console.log(
                        "@(", ix, iy, ") ",
                        "Expected:", lattice_ref.at(ix, iy).state,
                        "Observed:", lattice.at(ix, iy).state);
                }
            });
        });

        lattice.stepN();
        while (lattice_ref.timestep < lattice.timestep) {
            lattice_ref.step();
        }
    });

}

$(document).ready(function() {
    var canvas = $('#cv_gas')[0];

    var lattice = new GasLattice(50);

    function iter() {
        lattice.step();
        draw();
        setTimeout(iter, 1000 / 30);
    }

    function draw() {
        var k = 2;
        var scale = 10;

        var ctx = canvas.getContext('2d');
        ctx.fillStyle = 'black';
        ctx.rect(0, 0, 500, 500);
        ctx.fill();

        if (true) {
            ctx.save();
            ctx.scale(scale, scale);
            ctx.fillStyle = 'rgb(100, 100, 100)';
            ctx.lineWidth = 0.1;
            ctx.strokeStyle = 'white';
            _.map(_.range(lattice.n), function(ix) {
                _.map(_.range(lattice.n), function(iy) {
                    ctx.save();
                    ctx.translate(ix, iy);
                    var st = lattice.at(ix, iy).state;

                    ctx.beginPath();
                    ctx.rect(0.05, 0.05, 0.9, 0.9);
                    ctx.fill();

                    ctx.beginPath();
                    if (st & 1) {
                        ctx.moveTo(0.5, 0.5);
                        ctx.lineTo(0, 0.5);
                    }
                    if (st & 2) {
                        ctx.moveTo(0.5, 0.5);
                        ctx.lineTo(1, 0.5);
                    }
                    if (st & 4) {
                        ctx.moveTo(0.5, 0.5);
                        ctx.lineTo(0.5, 0);
                    }
                    if (st & 8) {
                        ctx.moveTo(0.5, 0.5);
                        ctx.lineTo(0.5, 1);
                    }
                    ctx.stroke();

                    ctx.restore();
                });
            });
            ctx.restore();
        }
        if (false) {
            ctx.save();
            ctx.scale(scale * k, scale * k);
            ctx.fillStyle = 'rgba(100, 100, 200, 0.8)';
            ctx.lineWidth = 0.1;
            ctx.strokeStyle = 'white';
            _.map(_.range(lattice.n / k), function(sx) {
                _.map(_.range(lattice.n / k), function(sy) {
                    // Average expected flow velocity.
                    var xaccum = 0;
                    var yaccum = 0;
                    ctx.save();
                    ctx.translate(sx, sy);
                    _.map(_.range(k), function(dx) {
                        _.map(_.range(k), function(dy) {
                            var ix = sx * k + dx;
                            var iy = sy * k + dy;
                            var st = lattice.at(ix, iy).state;
                            if (st & 1) {
                                xaccum -= 1;
                            }
                            if (st & 2) {
                                xaccum += 1;
                            }
                            if (st & 4) {
                                yaccum -= 1;
                            }
                            if (st & 8) {
                                yaccum += 1;
                            }
                        });
                    })
                    xaccum /= k * k;
                    yaccum /= k * k;

                    ctx.beginPath();
                    ctx.rect(0.05, 0.05, 0.9, 0.9);
                    ctx.fill();

                    ctx.beginPath();
                    ctx.moveTo(0.5, 0.5);
                    ctx.lineTo(0.5 + xaccum / 2, 0.5 + yaccum / 2);
                    ctx.stroke();

                    ctx.restore();
                });
            });
            ctx.restore();
        }

    }

    test();
    iter();
});
