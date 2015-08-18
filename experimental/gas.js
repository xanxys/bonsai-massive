$(document).ready(function() {
    var canvas = $('#cv_gas')[0];

    var Cell = function() {
        if (Math.random() < 0.1) {
            this.state = Math.floor(Math.random() * 15);  // 4 bit field (X-, X+, Y-, Y+) no flow by default.
        } else {
            this.state = 0;
        }
    };

    var n = 50;
    var grid = {};
    _.map(_.range(n), function(ix) {
        _.map(_.range(n), function(iy) {
            var key = ix + ":" + iy;
            grid[key] = new Cell();
            if (ix < n * 0.1 && n * 0.4 < iy && iy < n * 0.6) {
                grid[key].state = 2;
            }
        });
    });

    function iter() {
        step();
        draw();
        setTimeout(iter, 1000 / 30);
    }


    function step() {
        var new_grid = {};
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
        grid = new_grid;
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
            _.map(_.range(n), function(ix) {
                _.map(_.range(n), function(iy) {
                    ctx.save();
                    ctx.translate(ix, iy);
                    var key = ix + ":" + iy;
                    var st = grid[key].state;

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
            _.map(_.range(n / k), function(sx) {
                _.map(_.range(n / k), function(sy) {
                    // Average expected flow velocity.
                    var xaccum = 0;
                    var yaccum = 0;
                    ctx.save();
                    ctx.translate(sx, sy);
                    _.map(_.range(k), function(dx) {
                        _.map(_.range(k), function(dy) {
                            var ix = sx * k + dx;
                            var iy = sy * k + dy;
                            var key = ix + ":" + iy;
                            var st = grid[key].state;
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
