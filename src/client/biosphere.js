"use strict";

// Experimental grain interaction
class Grain {
    constructor() {
        this.position = new THREE.Vector3(
            Math.random(), Math.random(), Math.random() * 0.3);
        this.velocity = new THREE.Vector3(0, 0, 0);

        // Temporary buffer for calculating new position.
        this.position_new = new THREE.Vector3();
    }
}

// Poly6 kernel
function sph_kernel(dp, h) {
    var dp_sq = dp.lengthSq();
    if (dp_sq < h * h) {
        return Math.pow(h * h - dp_sq, 3) * (315 / 64 / Math.PI / Math.pow(h, 9));
    } else {
        return 0;
    }
}

// Spiky kernel
function sph_kernel_grad(dp, h) {
    var dp_len = dp.length();
    if (0 < dp_len && dp_len < h) {
        return dp.clone().multiplyScalar(Math.pow(h - dp_len, 2) / dp_len);
    } else {
        return new THREE.Vector3(0, 0, 0);
    }
}

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
class Client {
    constructor() {
        this.debug = (location.hash === '#debug');
        this.grains = _.map(_.range(50), () => {
            return new Grain();
        });
    	this.init();
    }

    // return :: ()
    init() {
    	this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.005, 15);
    	this.camera.up = new THREE.Vector3(0, 0, 1);
        this.camera.position.x = 0.3;
        this.camera.position.y = 0.3;
        this.camera.position.z = 0.4;
    	this.camera.lookAt(new THREE.Vector3(0, 0, 0));

    	this.scene = new THREE.Scene();

    	var sunlight = new THREE.DirectionalLight(0xcccccc);
    	sunlight.position.set(0, 0, 1).normalize();
    	this.scene.add(sunlight);

    	this.scene.add(new THREE.AmbientLight(0x333333));

    	var bg = new THREE.Mesh(
    		new THREE.IcosahedronGeometry(8, 1),
    		new THREE.MeshBasicMaterial({
    			wireframe: true,
    			color: '#ccc'
    		}));
    	this.scene.add(bg);

        var box = new THREE.Mesh(
    		new THREE.CubeGeometry(1, 1, 2),
    		new THREE.MeshBasicMaterial({
    			wireframe: true,
    			color: '#fcc'
    		}));
        box.position.x = 0.5;
        box.position.y = 0.5;
        box.position.z = 1;
    	this.scene.add(box);


        this.grains_objects = _.map(this.grains, (grain) => {
            var ball = new THREE.Mesh(
                new THREE.IcosahedronGeometry(0.03),
        		new THREE.MeshBasicMaterial({
        			color: '#ccf'
    		}));
            this.scene.add(ball);
            return ball;
        }, this);
        this.apply_grains();

    	// new, web worker API
    	// Selection
    	this.inspect_plant_id = null;
    	var curr_selection = null;

    	// start canvas
    	this.renderer = new THREE.WebGLRenderer({
    		antialias: true
    	});
    	this.renderer.setSize(window.innerWidth, window.innerHeight);
    	this.renderer.setClearColor('#eee');
    	$('#main').append(this.renderer.domElement);

    	// add mouse control (do this after canvas insertion)
    	this.controls = new THREE.TrackballControls(this.camera, this.renderer.domElement);
        this.controls.noZoom = false;
		this.controls.noPan = false;
        var _this = this;
    	this.controls.maxDistance = 10;
    }

    /* UI Utils */
    animate() {
    	// note: three.js includes requestAnimationFrame shim
    	var _this = this;
    	requestAnimationFrame(function(){_this.animate();});
        this.controls.update();
        this.update_grains();
        this.apply_grains();
    	this.renderer.render(this.scene, this.camera);
    }

    // Position-based dynamics.
    update_grains() {
        // Global world config.
        var dt = 1/30;
        var accel = new THREE.Vector3(0, 0, -1);

        // Global water config.
        var density_base = 1000.0;  // kg/m^3
        var h = 0.05;
        var mass_grain = 113 / 27 / 8;  // V_sphere(h) * density_base
        var cfm_epsilon = 1e-3;

        var grains = this.grains;

        // Apply gravity & velocity.
        _.each(grains, (grain) => {
            grain.position_new.copy(grain.position);
            grain.position_new.add(grain.velocity.clone().multiplyScalar(dt));
            grain.position_new.add(accel.clone().multiplyScalar(0.5 * dt * dt));
        });

        var density = function(ix_target) {
            return _.reduce(grains, (acc, grain) => {
                var weight = sph_kernel(grains[ix_target].position_new.clone().sub(grain.position_new), h);
                return acc + weight * mass_grain;
            }, 0);
        };

        var constraint = function(ix_target) {
            return density(ix_target) / density_base - 1;
        };

        var grad_constraint = function(ix_deriv, ix_target) {
            var result = new THREE.Vector3(0, 0, 0);
            _.each(grains, (grain) => {
                result.add(
                    sph_kernel_grad(
                        grains[ix_target].position_new.clone().sub(grain.position_new),
                        h));
            });
            return result.divideScalar(density_base);
        };

        // Iteratively resolve collisions & fluid constraints.
        _.each(_.range(3), () => {
            var lambdas = _.map(grains, (grain, ix) => {
                return -constraint(ix) / (_.reduce(grains, (acc, grain, ix_other) => {
                    return acc + grad_constraint(ix_other, ix).lengthSq();
                }, 0) + cfm_epsilon);
            });

            _.each(grains, (grain, ix) => {
                var delta_p = _.reduce(grains, (acc, grain_other, ix_other) => {
                    return acc.add(
                        grad_constraint(ix_other, ix).multiplyScalar(
                            lambdas[ix] + lambdas[ix_other]));
                }, new THREE.Vector3(0, 0, 0));

                grain.position_new.add(delta_p);

                // Box collision.
                if (grain.position_new.x < 0) {
                    grain.position_new.x *= -0.5;
                } else if (grain.position_new.x > 1) {
                    grain.position_new.x = 1 + (grain.position_new.x - 1) * -0.5;
                }
                if (grain.position_new.y < 0) {
                    grain.position_new.y *= -0.5;
                } else if (grain.position_new.y > 1) {
                    grain.position_new.y = 1 + (grain.position_new.y - 1) * -0.5;
                }
                if (grain.position_new.z < 0) {
                    grain.position_new.z *= -0.5;
                }
            });
        });

        // Actually update velocity & position.
        // position_new is destroyed after this.
        _.each(this.grains, (grain) => {
            grain.velocity
                .copy(grain.position_new)
                .sub(grain.position)
                .divideScalar(dt);

            grain.position.copy(grain.position_new);
        }, this);
    }

    apply_grains() {
        _.each(this.grains_objects, (go, ix) => {
            go.position.copy(this.grains[ix].position);
        }, this);
    }
}

// run app
$(document).ready(function() {
    new Client().animate();
});
