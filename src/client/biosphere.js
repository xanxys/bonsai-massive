"use strict";

// Experimental grain interaction
class Grain {
    constructor(is_water) {
        this.type_is_water = is_water;
        if (is_water) {
            this.position = new THREE.Vector3(
                Math.random(), Math.random(), Math.random() * 0.3 + 0.3);
        } else {
            this.position = new THREE.Vector3(
                Math.random() * 0.3 + 0.2, Math.random() * 0.3 + 0.2, Math.random() * 3);
        }

        this.velocity = new THREE.Vector3(0, 0, 0);

        // Temporary buffer for calculating new position.
        this.position_new = new THREE.Vector3();
    }

    is_water() {
        return this.type_is_water;
    }

    is_sand() {
        return !this.type_is_water;
    }
}

// Poly6 kernel
function sph_kernel(dp, h) {
    let dp_sq = dp.lengthSq();
    if (dp_sq < h * h) {
        return Math.pow(h * h - dp_sq, 3) * (315 / 64 / Math.PI / Math.pow(h, 9));
    } else {
        return 0;
    }
}

// Spiky kernel
function sph_kernel_grad(dp, h) {
    let dp_len = dp.length();
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
        this.grains = [];
        // Water
        /*
        _.each(_.range(500), () => {
            this.grains.push(new Grain(true));
        }, this);
        */
        // Sand
        _.each(_.range(300), () => {
            this.grains.push(new Grain(false));
        }, this);
    	this.init();
    }

    // return :: ()
    init() {
    	this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.005, 18);
    	this.camera.up = new THREE.Vector3(0, 0, 1);
        this.camera.position.x = 0.3;
        this.camera.position.y = 0.3;
        this.camera.position.z = 0.4;
    	this.camera.lookAt(new THREE.Vector3(0, 0, 0));

    	this.scene = new THREE.Scene();

    	let sunlight = new THREE.DirectionalLight(0xcccccc);
    	sunlight.position.set(0, 0, 1).normalize();
    	this.scene.add(sunlight);

    	this.scene.add(new THREE.AmbientLight(0x333333));

    	let bg = new THREE.Mesh(
    		new THREE.IcosahedronGeometry(8, 1),
    		new THREE.MeshBasicMaterial({
    			wireframe: true,
    			color: '#ccc'
    		}));
    	this.scene.add(bg);

        let box = new THREE.Mesh(
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
            let ball = new THREE.Mesh(
                new THREE.IcosahedronGeometry(0.1 / 2),  // make it smaller for visualization
                //new THREE.MeshNormalMaterial()
                grain.is_water() ? new THREE.MeshNormalMaterial() : new THREE.MeshBasicMaterial({color: '#fcc'})
            );
            this.scene.add(ball);
            return ball;
        }, this);
        this.apply_grains();

    	// new, web worker API
    	// Selection
    	this.inspect_plant_id = null;
    	let curr_selection = null;

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
        let _this = this;
    	this.controls.maxDistance = 8;
    }

    /* UI Utils */
    animate() {
    	// note: three.js includes requestAnimationFrame shim
    	let _this = this;
    	requestAnimationFrame(function(){_this.animate();});
        this.controls.update();
        this.update_grains();
        this.apply_grains();
    	this.renderer.render(this.scene, this.camera);
    }

    // Position-based dynamics.
    update_grains() {
        // Global world config.
        const dt = 1/30;
        const accel = new THREE.Vector3(0, 0, -1);

        // Global simulation config.
        const cfm_epsilon = 1e-3;

        // Global water config.
        const reflection_coeff = 0.5; // Must be in (0, 1)
        const density_base = 1000.0;  // kg/m^3
        const h = 0.1;
        const mass_grain = 0.1 * 113 / 20;  // V_sphere(h) * density_base
        const num_iter = 3;

        // Sand config.
        const sand_radius = 0.04;
        const sand_stiffness = 1e-3;
        const dyn_friction = 0.5; // must be in [0, 1)

        let grains = this.grains;

        // Apply gravity & velocity.
        _.each(grains, (grain) => {
            grain.position_new.copy(grain.position);
            grain.position_new.add(grain.velocity.clone().multiplyScalar(dt));
            grain.position_new.add(accel.clone().multiplyScalar(0.5 * dt * dt));
        });

        // Calculate closest neighbors.
        let bins = new Map();
        let pos_to_key = (position) => {
            return Math.floor(position.x / h) + ":" + Math.floor(position.y / h) + ":" + Math.floor(position.z / h);
        };
        let pos_to_neighbor_keys = (position) => {
            const ix = Math.floor(position.x / h);
            const iy = Math.floor(position.y / h);
            const iz = Math.floor(position.z / h);
            let result = [];
            for (let dx = -1; dx <= 1; dx++) {
                for (let dy = -1; dy <= 1; dy++) {
                    for (let dz = -1; dz <= 1; dz++) {
                        result.push((ix + dx) + ":" + (iy + dy) + ":" + (iz + dz));
                    }
                }
            }
            return result;
        };

        _.each(grains, (grain, ix) => {
            let key = pos_to_key(grain.position_new);
            let val = {pos: grain.position_new, data: ix};
            if (bins.has(key)) {
                bins.get(key).push(val);
            } else {
                bins.set(key, [val]);
            }
        });
        let neighbors = _.map(grains, (grain) => {
            let ns = [];
            _.each(pos_to_neighbor_keys(grain.position_new), (key) => {
                if (!bins.has(key)) {
                    return;
                }
                _.each(bins.get(key), (val) => {
                    if (grain.position_new.distanceTo(val.pos) < h) {
                        ns.push(val.data);
                    }
                });
            });
            return ns;
        });

        let density = function(ix_target) {
            return _.reduce(neighbors[ix_target], (acc, ix_other) => {
                if (!grains[ix_other].is_water()) {
                    return acc;
                }
                let weight = sph_kernel(grains[ix_target].position_new.clone().sub(grains[ix_other].position_new), h);
                return acc + weight * mass_grain;
            }, 0);
        };

        let density_constraint_deriv = function(ix_target) {
            return _.reduce(neighbors[ix_target], (m, ix_deriv) => {
                if (!grains[ix_deriv].is_water()) {
                    return m;
                }
                return m.set(ix_deriv,
                    _.reduce(neighbors[ix_target], (acc, ix_other) => {
                        if (ix_other === ix_target) {
                            return acc;
                        }
                        if (ix_deriv === ix_other) {
                            return acc.add(
                                sph_kernel_grad(
                                    grains[ix_other].position_new.clone().sub(grains[ix_target].position_new),
                                    h));
                        } else if (ix_deriv === ix_target) {
                            return acc.add(
                                sph_kernel_grad(
                                    grains[ix_target].position_new.clone().sub(grains[ix_other].position_new),
                                    h));
                        } else {
                            return acc;
                        }
                    }, new THREE.Vector3(0, 0, 0)).divideScalar(density_base * -1));
            }, new Map());
        };

        // :: [{
        //    constraint: number,
        //    gradient: Map Index Vector3
        // }]
        // Typically, gradient contains ix_target.
        // Result can be empty when there's no active constraint for given
        // particle.
        // gradient(ix) == Deriv[constraint, pos[ix]]
        let constraints_with_deriv = function(ix_target) {
            let cs = [];
            if (grains[ix_target].is_water()) {
                cs.push({
                    constraint: density(ix_target) / density_base - 1,
                    gradients: density_constraint_deriv(ix_target)
                });
            }
            if (grains[ix_target].is_sand()) {
                // This will result in 2 same constraints per particle pair,
                // but there's no problem (other than performance) for repeating
                // same constraint.
                _.each(neighbors[ix_target], (ix_other) => {
                    if (ix_target === ix_other) {
                        return; // no collision with self
                    }
                    if (!grains[ix_other].is_sand()) {
                        return; // No sand-other interaction for now.
                    }
                    let dp = grains[ix_target].position_new.clone().sub(grains[ix_other].position_new);
                    let penetration = sand_radius * 2 - dp.length();
                    if (penetration > 0) {
                        // Collision (no penetration) constraint.
                        dp.normalize();
                        let grads = new Map();
                        grads.set(ix_other, dp.clone().multiplyScalar(sand_stiffness));
                        grads.set(ix_target, dp.clone().multiplyScalar(-sand_stiffness));
                        cs.push({
                            constraint: penetration,
                            gradients: grads
                        });

                        // Tangential friction constraint.
                        let dv = grains[ix_target].position_new.clone().sub(grains[ix_target].position).sub(
                            grains[ix_other].position_new.clone().sub(grains[ix_other].position));
                        let d_tangent = dv.clone().projectOnPlane(dp).normalize();

                        // Both max static friction & dynamic friction are proportional to
                        // force along normal (collision).
                        if (dv.length() > 0) {
                            let sand_fric = dyn_friction * sand_stiffness;
                            let grads_t = new Map();
                            grads_t.set(ix_other, d_tangent.clone().multiplyScalar(-sand_fric));
                            grads_t.set(ix_target, d_tangent.clone().multiplyScalar(sand_fric));
                            cs.push({
                                constraint: dv.length(),
                                gradients: grads_t
                            });
                        }
                    }
                });
            }
            return cs;
        };

        // Iteratively resolve collisions & constraints.
        _.each(_.range(num_iter), () => {
            _.each(grains, (grain, ix) => {
                _.each(constraints_with_deriv(ix), (constraint) => {
                    let scale = - constraint.constraint / _.reduce(constraint.gradients.values(), (acc, grad) => {
                        return acc + grad.lengthSq();
                    }, cfm_epsilon);

                    constraint.gradients.forEach((grad, ix_feedback) => {
                        grains[ix_feedback].position_new.add(
                            grad.multiplyScalar(scale));
                    });
                });
            });

            // Box collision.
            _.each(grains, (grain, ix) => {
                if (grain.position_new.x < 0) {
                    grain.position_new.x *= -reflection_coeff;
                } else if (grain.position_new.x > 1) {
                    grain.position_new.x = 1 + (grain.position_new.x - 1) * -reflection_coeff;
                }
                if (grain.position_new.y < 0) {
                    grain.position_new.y *= -reflection_coeff;
                } else if (grain.position_new.y > 1) {
                    grain.position_new.y = 1 + (grain.position_new.y - 1) * -reflection_coeff;
                }
                if (grain.position_new.z < 0) {
                    grain.position_new.z *= -reflection_coeff;
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
