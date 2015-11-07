"use strict";

// Experimental grain interaction
class Grain {
    constructor(is_water, pos) {
        if (pos === undefined) {
            pos = new THREE.Vector3(
                Math.random(), Math.random(), Math.random() + 0.5);
        }
        this.type_is_water = is_water;
        this.position = pos.clone();
        this.velocity = new THREE.Vector3(0, 0, 0);

        // Temporary buffer for calculating new position.this.grains.push(new Grain(false, new THREE.Vector3(0.1, 0.1, 1.0)));
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

// A source that emits constant flow of given type of particle from
// nearly fixed location, until total_num particles are emitted from this source.
// ParticleSource is physically implausible, so it has high change of being
// removed in final version, but very useful for debugging / showing demo.
class ParticleSource {
    constructor(is_water, total_num, center_pos) {
        this.is_water = is_water;
        this.total_num = total_num;

        this.frames_per_particle = 4;
        this.base_positions = [
            center_pos.clone().add(new THREE.Vector3(-0.1, -0.1, 0)),
            center_pos.clone().add(new THREE.Vector3(-0.1,  0.1, 0)),
            center_pos.clone().add(new THREE.Vector3( 0.1, -0.1, 0)),
            center_pos.clone().add(new THREE.Vector3( 0.1,  0.1, 0))
        ];
    }

    // Return particles to be inserted to the scene at given timestamp.
    maybe_emit(timestamp) {
        if (timestamp >= this.frames_per_particle * this.total_num) {
            return [];
        }
        if (timestamp % this.frames_per_particle !== 0) {
            return [];
        }
        const phase = (timestamp / this.frames_per_particle) % this.base_positions.length;
        const initial_pos = this.base_positions[phase].clone()
            .add(new THREE.Vector3(Math.random() * 0.01, Math.random() * 0.01, 0));
        this.num_emitted += 1;
        return [new Grain(this.is_water, initial_pos)];
    }
};

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
class Client {
    constructor() {
        this.debug = (location.hash === '#debug');
        this.grains = [];
        this.sources = [
            new ParticleSource(true, 1000, new THREE.Vector3(0.5, 0.5, 2.0)),
            new ParticleSource(false, 1000, new THREE.Vector3(0.1, 0.1, 1.0))
        ];
        this.timestamp = 0;
    	this.init();
    }

    // return :: ()
    init() {
    	this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.005, 18);
    	this.camera.up = new THREE.Vector3(0, 0, 1);
        this.camera.position.x = 1.5;
        this.camera.position.y = 2.0;
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

        this.grains_objects = [];

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
        const floor_static = 0.7;
        const floor_dynamic = 0.5;
        const cfm_epsilon = 1e-3;
        const sand_water_equiv = 0.3;

        // Global water config.
        const reflection_coeff = 0.3; // Must be in (0, 1)
        const density_base = 1000.0;  // kg/m^3
        const h = 0.1;
        const mass_grain = 0.1 * 113 / 20;  // V_sphere(h) * density_base
        const num_iter = 3;

        // Sand config.
        const sand_radius = 0.04;
        const sand_stiffness = 2e-2;
        const friction_static = 0.5; // must be in [0, 1)
        const friction_dynamic = 0.3; // must be in [0, friction_static)

        _.each(this.sources, (source) => {
            this.grains = this.grains.concat(source.maybe_emit(this.timestamp));
        }, this)

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
            console.assert(grains[ix_target].is_water());
            return _.reduce(neighbors[ix_target], (acc, ix_other) => {
                let equiv = 1.0;
                if (grains[ix_other].is_sand()) {
                    equiv = sand_water_equiv;
                }
                let weight = sph_kernel(grains[ix_target].position_new.clone().sub(grains[ix_other].position_new), h);
                return acc + weight * mass_grain * equiv;
            }, 0);
        };

        let density_constraint_deriv = function(ix_target) {
            console.assert(grains[ix_target].is_water());
            return _.reduce(neighbors[ix_target], (m, ix_deriv) => {
                let equiv = 1.0;
                if (grains[ix_deriv].is_sand()) {
                    equiv = sand_water_equiv;
                }
                return m.set(ix_deriv,
                    _.reduce(neighbors[ix_target], (acc, ix_other) => {
                        if (ix_other === ix_target) {
                            return acc;
                        }
                        let other_equiv = 1.0;
                        if (grains[ix_other].is_sand()) {
                            other_equiv = sand_water_equiv;
                        }
                        if (ix_deriv === ix_other) {
                            return acc.add(
                                sph_kernel_grad(
                                    grains[ix_other].position_new.clone().sub(grains[ix_target].position_new),
                                    h).multiplyScalar(equiv * other_equiv));
                        } else if (ix_deriv === ix_target) {
                            return acc.add(
                                sph_kernel_grad(
                                    grains[ix_target].position_new.clone().sub(grains[ix_other].position_new),
                                    h).multiplyScalar(equiv * other_equiv));
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
                        var f_normal = penetration * sand_stiffness;
                        dp.normalize();
                        let grads = new Map();
                        grads.set(ix_other, dp.clone().multiplyScalar(sand_stiffness));
                        grads.set(ix_target, dp.clone().multiplyScalar(-sand_stiffness));
                        cs.push({
                            constraint: f_normal,
                            gradients: grads
                        });

                        // Tangential friction constraint.
                        let dv = grains[ix_target].position_new.clone().sub(grains[ix_target].position).sub(
                            grains[ix_other].position_new.clone().sub(grains[ix_other].position));
                        let dir_tangent = dv.clone().projectOnPlane(dp).normalize();

                        // Both max static friction & dynamic friction are proportional to
                        // force along normal (collision).
                        if (dv.length() > 0) {
                            let grads_t = new Map();
                            let f_tangent = dv.length();
                            if (f_tangent < f_normal * friction_static) {
                                // Static friction.
                                grads_t.set(ix_other, dir_tangent.clone().multiplyScalar(-f_tangent));
                                grads_t.set(ix_target, dir_tangent.clone().multiplyScalar(f_tangent));
                            } else {
                                // Dynamic friction.
                                f_tangent = f_normal * friction_dynamic;
                                console.assert(f_tangent < dv.length());
                                grads_t.set(ix_other, dir_tangent.clone().multiplyScalar(-f_tangent));
                                grads_t.set(ix_target, dir_tangent.clone().multiplyScalar(f_tangent));
                            }
                            cs.push({
                                constraint: f_tangent,
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

            // Box collision & floor friction.
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
                    let dz = -grain.position_new.z * (1 + reflection_coeff);
                    grain.position_new.z += dz;

                    let dxy = grain.position_new.clone().sub(grain.position).projectOnPlane(new THREE.Vector3(0, 0, 1));
                    if (dxy.length() < dz * floor_static) {
                        // Static friction.
                        grain.position_new.x = grain.position.x;
                        grain.position_new.y = grain.position.y;
                    } else {
                        // Dynamic friction.
                        let dxy_capped = Math.min(dxy.length(), dz * floor_dynamic);
                        grain.position_new.sub(dxy.normalize().multiplyScalar(dxy_capped));
                    }
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
        this.timestamp += 1;
    }

    apply_grains() {
        let delta_num = this.grains.length - this.grains_objects.length;
        if (delta_num > 0) {
            this.grains_objects = this.grains_objects.concat(_.map(this.grains.slice(this.grains_objects.length), (grain) => {
                let ball = new THREE.Mesh(
                    new THREE.IcosahedronGeometry(0.1 / 3),  // make it smaller for visualization
                    //new THREE.MeshNormalMaterial()
                    grain.is_water() ? new THREE.MeshNormalMaterial() : new THREE.MeshLambertMaterial({color: '#fcc'})
                );
                this.scene.add(ball);
                return ball;
            }, this));
        } else if (delta_num < 0) {
            this.grains_objects = this.grains_objects.slice(0, this.grains.length);
        }

        console.assert(this.grains.length === this.grains_objects.length);
        _.each(this.grains_objects, (go, ix) => {
            go.position.copy(this.grains[ix].position);
        }, this);
    }
}

// run app
$(document).ready(function() {
    new Client().animate();
});
