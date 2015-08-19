declare var $;
declare var _;

class Cell {
    // 4 bit field
    // (X-, X+, Y-, Y+)
    // LSB
    public state : number;

    constructor() {
        if (Math.random() < 0.1) {
            this.state = Math.floor(Math.random() * 15);
        } else {
            this.state = 0;
        }
    }
}

class GasLattice {
    public n : number;
    private grid;

    constructor(n : number) {
        this.n = n;
        var grid = {};
        _.map(_.range(n), function(ix) {
            _.map(_.range(n), function(iy) {
                var cell = new Cell();
                if (ix < n * 0.1 && n * 0.4 < iy && iy < n * 0.6) {
                    cell.state = 2;
                }

                var key = ix + ":" + iy;
                grid[key] = cell;
            });
        });
        this.grid = grid;
    }

    public step() {
        var new_grid = {};
        var grid = this.grid;
        var n = this.n;
        _.map(_.range(n), function(ix) {
            _.map(_.range(n), function(iy) {
                var key = ix + ":" + iy;
                var c = new Cell();
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
                c.state = s;
                new_grid[key] = c;
            });
        });
        this.grid = new_grid;
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


}

// (Hopefully) super-accelerated Gas Lattice using hashlife.
class HashGasLattice {
    // Current time.
    public timestep : number;

    // Root tree at t = 0. This will get bigger if we step further
    // (to account for boundary).
    /*
    public root : HashGasCell;
    public r_x0 : number;
    public r_y0 : number;
    */

    public snapshot : HashGasCell;
    public s_x0 : number;
    public s_y0 : number;
    public s_dx : number;
    public s_dy : number;

    constructor() {
        this.timestep = 0;
        /*
        this.root = HashGasCell.newLeaf(1);
        this.r_x0 = 0;
        this.r_y0 = 0;
        */
        this.snapshot = HashGasCell.newLeaf(1);
        this.s_x0 = 0;
        this.s_y0 = 0;
        this.s_dx = 1;
        this.s_dy = 1;
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

    private static getEmpty(level : number) : HashGasCell {
        if (level === 0) {
            return HashGasCell.newLeaf(-1);
        } else {
            var e = HashGasLattice.getEmpty(level - 1);
            return HashGasCell.newNode(e, e, e, e);
        }
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


$(document).ready(function() {
    var canvas = $('#cv_gas')[0];

    var lattice = new GasLattice(50);
    var h_lattice = new HashGasLattice();

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

    iter();
});
