declare var $;
declare var _;
declare var bigInt;

class Cell {
    // 4 bit field
    // (X-, X+, Y-, Y+)
    // LSB
    public state : number;

    constructor(state : number) {
        this.state = state;
    }
}

class RemoteLattice {
    public n : number;
    public timestep : number;
    private grid;

    constructor() {
        var t = this;
        $.ajax('http://localhost:8000/api/test').done(function(data) {
            console.log("Got", data.N, "^2 lattice");
            t.n = data.N;
            t.grid = data.State;
        });
    }

    public step() {
        var t = this;
        $.ajax('http://localhost:8000/api/test').done(function(data) {
            console.log("Got", data.N, "^2 lattice");
            t.grid = data.State;
        });
    }

    public at(ix : number, iy : number) : Cell {
        var key : string = ix + ":" + iy;
        return new Cell(this.grid[key]);
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


$(document).ready(function() {
    var canvas = $('#cv_gas')[0];

    var lattice = new RemoteLattice();
    console.log(lattice);

    function iter() {
        lattice.step();
        draw();
        setTimeout(iter, 500);
    }

    function draw() {
        var k = 4;
        var scale = 3;

        var ctx = canvas.getContext('2d');
        ctx.fillStyle = 'black';
        ctx.rect(0, 0, 500, 500);
        ctx.fill();

        if (false) {
            ctx.save();
            ctx.scale(scale, scale);
            ctx.lineWidth = 0.1;
            ctx.strokeStyle = 'white';
            _.map(_.range(lattice.n), function(ix) {
                _.map(_.range(lattice.n), function(iy) {
                    ctx.save();
                    ctx.translate(ix, iy);
                    var st = lattice.at(ix, iy).state;

                    if (st === -1) {
                        ctx.fillStyle = 'rgb(200, 100, 100)';
                    } else {
                        ctx.fillStyle = 'rgb(100, 100, 100)';
                    }
                    ctx.beginPath();
                    ctx.rect(0.05, 0.05, 0.9, 0.9);
                    ctx.fill();
                    if (st !== -1) {
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
                    }

                    ctx.restore();
                });
            });
            ctx.restore();
        }
        if (true) {
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

    /*test();
    test_performance();
    */
    iter();
});
