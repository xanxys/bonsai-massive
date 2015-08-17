$(document).ready(function() {
    var canvas = $('#cv_grain')[0];

    var BGrain = function(id) {
        this.id = id;
        // mm
        this.p = new THREE.Vector2(Math.random() * 100, Math.random() * 100);
        this.v = new THREE.Vector2(Math.random() * 10 - 5, Math.random() * 10 - 5);
        this.f = new THREE.Vector2(0, 0);
        this.mass = 1;
        this.radius = 2;
    };

    var dt = 1.0 / 300;



    var grains = _.map(_.range(100), function(ix) {
        return new BGrain(ix);
    });

    function iter() {
        step();
        draw();
        setTimeout(iter, 1000 / 30);
    }

    function forceFirst(g1, g2, d) {
        var dir = g2.p.clone().sub(g1.p).normalize();

        return dir.multiplyScalar(-1 / (d * d));
    }

    function step() {
        var force = new THREE.Vector2();
        _.each(grains, function(grain) {
            // Accumulate force.
            grain.f.set(0, 0);
            var coll = false;
            _.each(grains, function(grain_o) {
                if (grain_o.id === grain.id) {
                    return;
                }
                var d = grain.p.distanceTo(grain_o.p);
                if (d >= grain.radius + grain_o.radius) {
                    return;
                }
                var dp = grain_o.p.clone().sub(grain.p);
                var dv = grain_o.v.clone().sub(grain.v);
                if (dp.dot(dv) > 0) {
                    return;
                }
                coll = true;

                grain.f.add(forceFirst(grain, grain_o, d));
            });
            if (coll) {
                grain.v.multiplyScalar(-1);
            }
        });

        _.each(grains, function(grain) {
            // gravity
             grain.f.y -= grain.mass * 100;
            // weak constraining force.
            /*
            if (grain.p.x < 10) {
                force.x += 100;
            }
            if (grain.p.x > 90) {
                force.x -= 100;
            }
            */

            grain.v.add(grain.f.multiplyScalar(dt / grain.mass));
            grain.p.add(grain.v.clone().multiplyScalar(dt));

            // Drag.

            if (grain.p.y - grain.radius <= 0) {
                grain.v.y *= -1;
                grain.p.y = 0 + grain.radius;
            }
            if (grain.p.x - grain.radius <= 10) {
                grain.v.x *= -1;
                grain.p.x = 10 + grain.radius;
            }
            if (grain.p.x + grain.radius >= 90) {
                grain.v.x *= -1;
                grain.p.x = 90 - grain.radius;
            }

        });
    }

    function draw() {
        var ctx = canvas.getContext('2d');

        ctx.fillStyle = "rgb(0, 0, 0)";
        ctx.beginPath();
        ctx.rect(0, 0, 500, 500);
        ctx.fill();

        ctx.save();
        ctx.translate(0, 500);
        ctx.scale(5, -5);

        _.each(grains, function(grain) {
            ctx.fillStyle = 'rgba(100, 100, 255, 0.5)';
            ctx.beginPath();
            ctx.arc(grain.p.x, grain.p.y, grain.radius, 0, Math.PI * 2);
            ctx.fill();
        });
        ctx.restore();
    }

    iter();
});
