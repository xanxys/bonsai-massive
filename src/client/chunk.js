(function() {

if(console.assert === undefined) {
	console.assert = function(cond) {
		if(!cond) {
			throw "assertion failed";
		}
	};
}

var now = function() {
	if(typeof performance !== 'undefined') {
		return performance.now();
	} else {
		return new Date().getTime();
	}
};


// Collections of cells that forms a "single" plant.
// This is not biologically accurate depiction of plants,
// (e.g. vegetative growth, physics)
// Most plants seems to have some kind of information sharing system within them
// via transportation of regulation growth factors.
//
// 100% efficiency, 0-latency energy storage and transportation within Plant.
// (n.b. energy = power * time)
//
// position :: THREE.Vector3<World>
var Plant = function(position, unsafe_chunk, energy, genome, plant_id) {
	this.unsafe_chunk = unsafe_chunk;

	// tracer
	this.age = 0;
	this.id = plant_id;

	// physics
	this.seed_innode_to_world = new THREE.Matrix4().compose(
		position,
		new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 0, 1), Math.random() * 2 * Math.PI),
		new THREE.Vector3(1, 1, 1));
	this.position = position;

	// biophysics
	this.energy = energy;
	this.seed = new Cell(this, Signal.SHOOT_END);

	// genetics
	this.genome = genome;
};

Plant.prototype.step = function() {
	// Step cells (w/o collecting/stepping separation, infinite growth will occur)
	this.age += 1;
	_.each(this.collectCells(), function(cell) {
		cell.step();
	});
	
	console.assert(this.seed.age === this.age);

	var mech_valid = this.seed.checkMechanics();
	this.seed.updatePose(this.seed_innode_to_world);

	// Consume/store in-Plant energy.
	this.energy += this._powerForPlant() * 1;

	if(this.energy <= 0 || !mech_valid) {
		// die
		this.unsafe_chunk.remove_plant(this);
	}
};

// Approximates lifetime of the plant.
// Max growth=1, zero growth=0.
// return :: [0,1]
Plant.prototype.growth_factor = function() {
	return Math.exp(-this.age / 20);
};

// return :: THREE.Object3D<world>
Plant.prototype.materialize = function(merge) {
	var proxies = _.map(this.collectCells(), function(cell) {
		var m = cell.materializeSingle();

		var trans = new THREE.Vector3();
		var q = new THREE.Quaternion();
		var s = new THREE.Vector3();
		cell.loc_to_world.decompose(trans, q, s);

		m.cell = cell;
		m.position = trans;
		m.quaternion = q;
		return m;
	});

	if(merge) {
		var merged_geom = new THREE.Geometry();
		_.each(proxies, function(proxy) {
			THREE.GeometryUtils.merge(merged_geom, proxy);
		});

		var merged_plant = new THREE.Mesh(
			merged_geom,
			new THREE.MeshLambertMaterial({vertexColors: THREE.VertexColors}));

		return merged_plant;
	} else {
		var three_plant = new THREE.Object3D();
		_.each(proxies, function(proxy) {
			three_plant.add(proxy);
		});
		return three_plant;
	}
};

Plant.prototype.collectCells = function() {
	var all_cells = [];
	var collect_cell_recursive = function(cell) {
		all_cells.push(cell);
		_.each(cell.children, collect_cell_recursive);
	}
	collect_cell_recursive(this.seed);
	return all_cells;
};

Plant.prototype.get_stat = function() {
	var stat_cells = _.map(this.collectCells(), function(cell) {
		return cell.signals;
	});

	var stat = {};
	stat["#cells"] = stat_cells.length;
	stat['cells'] = stat_cells;
	stat['age/T'] = this.age;
	stat['stored/E'] = this.energy;
	stat['delta/(E/T)'] = this._powerForPlant();
	return stat;
};

Plant.prototype.get_genome = function() {
	return this.genome;
};

Plant.prototype._powerForPlant = function() {
	var sum_power_cell_recursive = function(cell) {
		return cell.powerForPlant() +
			sum(_.map(cell.children, sum_power_cell_recursive));
	};
	return sum_power_cell_recursive(this.seed);
}

// Cell's local coordinates is symmetric for X,Y, but not Z.
// Normally Z is growth direction, assuming loc_to_parent to near to identity.
//
//  Power Generation (<- Light):
//    sum of photosynthesis (LEAF)
//  Power Consumption:
//    basic (minimum cell volume equivalent)
//    linear-volume
var Cell = function(plant, initial_signal) {
	// tracer
	this.age = 0;

	// in-sim (light)
	this.photons = 0;

	// in-sim (phys + bio)
	this.loc_to_parent = new THREE.Quaternion();
	this.sx = 1e-3;
	this.sy = 1e-3;
	this.sz = 1e-3;
	this.loc_to_world = new THREE.Matrix4();

	// in-sim (bio)
	this.plant = plant;
	this.children = [];  // out_conn
	this.power = 0;

	// in-sim (genetics)
	this.signals = [initial_signal];
};

// Run pseudo-mechanical stability test based solely
// on mass and cross-section.
// return :: valid :: bool
Cell.prototype.checkMechanics = function() {
	return this._checkMass().valid;
};

// return: {valid: bool, total_mass: num}
Cell.prototype._checkMass = function() {
	var mass = 1e3 * this.sx * this.sy * this.sz;  // kg

	var total_mass = mass;
	var valid = true;
	_.each(this.children, function(cell) {
		var child_result = cell._checkMass();
		total_mass += child_result.total_mass;
		valid &= child_result.valid;
	});

	// 4mm:30g max
	// mass[kg] / cross_section[m^2] = 7500.
	if(total_mass / (this.sx * this.sy) > 7500 * 5) {
		valid = false;
	}

	// 4mm:30g * 1cm max
	// mass[kg]*length[m] / cross_section[m^2] = 75
	if(total_mass * this.sz / (this.sx * this.sy) > 75 * 5) {
		valid = false;
	}

	return {
		valid: valid,
		total_mass: total_mass
	};
};

// sub_cell :: Cell
// return :: ()
Cell.prototype.add = function(sub_cell) {
	if(this === sub_cell) {
		throw new Error("Tried to add itself as child.", sub_cell);
	} else {
		this.children.push(sub_cell);
	}
};

// Return net usable power for Plant.
// return :: float<Energy>
Cell.prototype.powerForPlant = function() {
	return this.power;
};

Cell.prototype._beginUsePower = function() {
	this.power = 0;
};

// return :: bool
Cell.prototype._withdrawEnergy = function(amount) {
	if(this.plant.energy > amount) {
		this.plant.energy -= amount;
		this.power -= amount;

		return true;
	} else {
		return false;
	}
}

Cell.prototype._withdrawVariableEnergy = function(max_amount) {
	var amount = Math.min(Math.max(0, this.plant.energy), max_amount);
	this.plant.energy -= amount;
	this.power -= amount;
	return amount;
}

Cell.prototype._withdrawStaticEnergy = function() {
	var delta_static = 0;

	// +: photo synthesis
	var efficiency = this._getPhotoSynthesisEfficiency();
	delta_static += this.photons * 1e-9 * 15000 * efficiency;

	// -: basic consumption (stands for common func.)
	delta_static -= 10 * 1e-9;

	// -: linear-volume consumption (stands for cell substrate maintainance)
	var volume_consumption = 1.0;
	delta_static -= this.sx * this.sy * this.sz * volume_consumption;
	
	this.photons = 0;

	if(this.plant.energy < delta_static) {
		this.plant.energy = -1e-3;  // set death flag (TODO: implicit value encoding is bad idea)
	} else {
		this.power += delta_static;
		this.plant.energy += delta_static;
	}
};

Cell.prototype._getPhotoSynthesisEfficiency = function() {
	// 1:1/2, 2:3/4, etc...
	var num_chl = sum(_.map(this.signals, function(sig) {
		return (sig === Signal.CHLOROPLAST) ? 1 : 0;
	}));

	return 1 - Math.pow(0.5, num_chl);
}

// return :: ()
Cell.prototype.step = function() {
	var _this = this;
	this.age += 1;
	this._beginUsePower();
	this._withdrawStaticEnergy();

	// Unified genome.
	function unity_calc_prob_term(signal) {
		if(signal === Signal.HALF) {
			return 0.5;
		} else if(signal === Signal.GROWTH) {
			return _this.plant.growth_factor();
		} else if(signal.length >= 2 && signal[0] === Signal.INVERT) {
			return 1 - unity_calc_prob_term(signal.substr(1));
		} else if(_.contains(_this.signals, signal)) {
			return 1;
		} else {
			return 0.001;
		}
	}

	function unity_calc_prob(when) {
		return product(_.map(when, unity_calc_prob_term));
	}
	
	// Gene expression and transcription.
	_.each(this.plant.genome.unity, function(gene) {
		if(unity_calc_prob(gene['when']) > Math.random()) {
			var num_codon = sum(_.map(gene['emit'], function(sig) {
				return sig.length
			}));

			if(_this._withdrawEnergy(num_codon * 1e-10)) {
				_this.signals = _this.signals.concat(gene['emit']);
			}
		}
	});

	// Bio-physics.
	// TODO: define remover semantics.
	var removers = {};
	_.each(this.signals, function(signal) {
		if(signal.length >= 2 && signal[0] === Signal.REMOVER) {
			var rm = signal.substr(1);
			if(removers[rm] !== undefined) {
				removers[rm] += 1;
			} else {
				removers[rm] = 1;
			}
		}
	});

	var new_signals = [];
	_.each(this.signals, function(signal) {
		if(signal.length === 3 && signal[0] === Signal.DIFF) {
			_this.add_cont(signal[1], signal[2]);
		} else if(signal === Signal.G_DX) {
			_this.sx += 1e-3;
		} else if(signal === Signal.G_DY) {
			_this.sy += 1e-3;
		} else if(signal === Signal.G_DZ) {
			_this.sz += 1e-3;
		} else if(removers[signal] !== undefined && removers[signal] > 0) {
			removers[signal] -= 1;
		} else {
			new_signals.push(signal);
		}
	});
	this.signals = new_signals;

	// Physics
	if(_.contains(this.signals, Signal.FLOWER)) {
		// Disperse seed once in a while.
		// TODO: this should be handled by physics, not biology.
		// Maybe dead cells with stored energy survives when fallen off.
		if(Math.random() < 0.01) {
			var seed_energy = _this._withdrawVariableEnergy(Math.pow(20e-3, 3) * 10);

			// Get world coordinates.
			var trans = new THREE.Vector3();
			var _rot = new THREE.Quaternion();
			var _scale = new THREE.Vector3();
			this.loc_to_world.decompose(trans, _rot, _scale);

			// TODO: should be world coodinate of the flower
			this.plant.unsafe_chunk.disperse_seed_from(
				trans, seed_energy, this.plant.genome.naturalClone());
		}
	}
};

Cell.prototype.updatePose = function(innode_to_world) {
	// Update this.
	var parent_to_loc = this.loc_to_parent.clone().inverse();

	var innode_to_center = new THREE.Matrix4().compose(
		new THREE.Vector3(0, 0, -this.sz / 2),
		parent_to_loc,
		new THREE.Vector3(1, 1, 1));
	var center_to_innode = new THREE.Matrix4().getInverse(innode_to_center);
	this.loc_to_world = innode_to_world.clone().multiply(
		center_to_innode);

	var innode_to_outnode = new THREE.Matrix4().compose(
		new THREE.Vector3(0, 0, -this.sz),
		parent_to_loc,
		new THREE.Vector3(1, 1, 1));

	var outnode_to_innode = new THREE.Matrix4().getInverse(innode_to_outnode);
	var outnode_to_world = innode_to_world.clone().multiply(
		outnode_to_innode);

	_.each(this.children, function(child) {
		child.updatePose(outnode_to_world);
	});
};

// Create origin-centered, colored AABB for this Cell.
// return :: THREE.Mesh
Cell.prototype.materializeSingle = function() {
	// Create cell object [-sx/2,sx/2] * [-sy/2,sy/2] * [0, sz]
	var flr_ratio = (_.contains(this.signals, Signal.FLOWER)) ? 0.5 : 1;
	var chl_ratio = 1 - this._getPhotoSynthesisEfficiency();

	var color_diffuse = new THREE.Color();
	color_diffuse.setRGB(
		chl_ratio,
		flr_ratio,
		flr_ratio * chl_ratio);

	if(this.photons === 0) {
		color_diffuse.offsetHSL(0, 0, -0.2);
	}
	if(this.plant.energy < 1e-4) {
		var t = 1 - this.plant.energy * 1e4;
		color_diffuse.offsetHSL(0, -t, 0);
	}

	var geom_cube = new THREE.CubeGeometry(this.sx, this.sy, this.sz);
	for(var i = 0; i < geom_cube.faces.length; i++) {
		for(var j = 0; j < 3; j++) {
			geom_cube.faces[i].vertexColors[j] = color_diffuse;
		}
	}

	return new THREE.Mesh(
		geom_cube,
		new THREE.MeshLambertMaterial({
			vertexColors: THREE.VertexColors}));
};

Cell.prototype.givePhoton = function() {
	this.photons += 1;
};

// Get Cell age in ticks.
// return :: int (tick)
Cell.prototype.get_age = function() {
	return this.age;
};

// counter :: dict(string, int)
// return :: dict(string, int)
Cell.prototype.count_type = function(counter) {
	var key = this.signals[0];

	counter[key] = 1 + (_.has(counter, key) ? counter[key] : 0);

	_.each(this.children, function(child) {
		child.count_type(counter);
	}, this);

	return counter;
};

// initial :: Signal
// locator :: LocatorSignal
// return :: ()
Cell.prototype.add_cont = function(initial, locator) {
	function calc_rot(desc) {
		if(desc === Signal.CONICAL) {
			return new THREE.Quaternion().setFromEuler(new THREE.Euler(
				Math.random() - 0.5,
				Math.random() - 0.5,
				0));
		} else if(desc === Signal.HALF_CONICAL) {
			return new THREE.Quaternion().setFromEuler(new THREE.Euler(
				(Math.random() - 0.5) * 0.5,
				(Math.random() - 0.5) * 0.5,
				0));
		} else if(desc === Signal.FLIP) {
			return new THREE.Quaternion().setFromEuler(new THREE.Euler(
				-Math.PI / 2,
				0,
				0));
		} else if(desc === Signal.TWIST) {
			return new THREE.Quaternion().setFromEuler(new THREE.Euler(
				0,
				0,
				(Math.random() - 0.5) * 1));
		} else {
			return new THREE.Quaternion();
		}
	}


	var new_cell = new Cell(this.plant, initial);
	new_cell.loc_to_parent = calc_rot(locator);	
	this.add(new_cell);
};

// Represents soil surface state by a grid.
// parent :: Chunk
// size :: float > 0
var Soil = function(parent, size) {
	this.parent = parent;

	this.n = 35;
	this.size = size;
};

// return :: ()
Soil.prototype.step = function() {
};

// return :: THREE.Object3D
Soil.prototype.materialize = function() {
	// Create texture.
	var canvas = this._generateTexture();

	// Attach tiles to the base.
	var tex = new THREE.Texture(canvas);
	tex.needsUpdate = true;

	var soil_plate = new THREE.Mesh(
		new THREE.CubeGeometry(this.size, this.size, 1e-3),
		new THREE.MeshBasicMaterial({
			map: tex
		}));
	return soil_plate;
};

Soil.prototype.serialize = function() {
	var array = [];
	_.each(_.range(this.n), function(y) {
		_.each(_.range(this.n), function(x) {
			var v = this.parent.light.shadow_map[x + y * this.n] > 1e-3 ? 0.1 : 0.5;
			array.push(v);
		}, this);
	}, this);
	return {
		luminance: array,
		n: this.n,
		size: this.size
	};
};

// return :: Canvas
Soil.prototype._generateTexture = function() {
	var canvas = document.createElement('canvas');
	canvas.width = this.n;
	canvas.height = this.n;
	var context = canvas.getContext('2d');
	_.each(_.range(this.n), function(y) {
		_.each(_.range(this.n), function(x) {
			var v = this.parent.light.shadow_map[x + y * this.n] > 1e-3 ? 0.1 : 0.5;
			var lighting = new THREE.Color().setRGB(v, v, v);

			context.fillStyle = lighting.getStyle();
			context.fillRect(x, this.n - y, 1, 1);
		}, this);
	}, this);
	return canvas;
};

// Downward directional light.
var Light = function(chunk, size) {
	this.chunk = chunk;

	this.n = 35;
	this.size = size;

	this.shadow_map = new Float32Array(this.n * this.n);
};

Light.prototype.step = function() {
	this.updateShadowMapHierarchical();
};

Light.prototype.updateShadowMapHierarchical = function() {
	var _this = this;

	// Put Plants to all overlapping 2D uniform grid cells.
	var ng = 15;
	var grid = _.map(_.range(0, ng), function(ix) {
		return _.map(_.range(0, ng), function(iy) {
			return [];
		});
	});
	
	_.each(this.chunk.children, function(plant) {
		var object = plant.materialize(false);
		object.updateMatrixWorld();

		var v_min = new THREE.Vector3();
		var v_max = new THREE.Vector3();
		var v_temp = new THREE.Vector3();
		_.each(object.children, function(child) {
			// Calculate AABB.
			v_min.set(1e3, 1e3, 1e3);
			v_max.set(-1e3, -1e3, -1e3);
			_.each(child.geometry.vertices, function(vertex) {
				v_temp.set(vertex.x, vertex.y, vertex.z);
				child.localToWorld(v_temp);
				v_min.min(v_temp);
				v_max.max(v_temp);
			});

			// Store to uniform grid.
			var vi0 = toIxV_unsafe(v_min);
			var vi1 = toIxV_unsafe(v_max);

			var ix0 = Math.max(0, Math.floor(vi0.x));
			var iy0 = Math.max(0, Math.floor(vi0.y));
			var ix1 = Math.min(ng, Math.ceil(vi1.x));
			var iy1 = Math.min(ng, Math.ceil(vi1.y));

			for(var ix = ix0; ix < ix1; ix++) {
				for(var iy = iy0; iy < iy1; iy++) {
					grid[ix][iy].push(child);
				}
			}
		});
	});

	function toIxV_unsafe(v3) {
		v3.multiplyScalar(ng / _this.size);
		v3.x += ng * 0.5;
		v3.y += ng * 0.5;
		return v3;
	}

	// Accelerated ray tracing w/ the uniform grid.
	function intersectDown(origin, near, far) {
		var i = toIxV_unsafe(origin.clone());
		var ix = Math.floor(i.x);
		var iy = Math.floor(i.y);

		if(ix < 0 || iy < 0 || ix >= ng || iy >= ng) {
			return [];
		}
		
		return new THREE.Raycaster(origin, new THREE.Vector3(0, 0, -1), near, far)
			.intersectObjects(grid[ix][iy], true);
	}

	for(var i = 0; i < this.n; i++) {
		for(var j = 0; j < this.n; j++) {
			var isect = intersectDown(
				new THREE.Vector3(
					((i + Math.random()) / this.n - 0.5) * this.size,
					((j + Math.random()) / this.n - 0.5) * this.size,
					10),
				0.1,
				1e2);

			if(isect.length > 0) {
				isect[0].object.cell.givePhoton();
				this.shadow_map[i + j * this.n] = isect[0].point.z;
			} else {
				this.shadow_map[i + j * this.n] = 0;
			}
		}
	}
};


// Chunk world class. There's no interaction between bonsai instances,
// and Chunk just borrows scene, not owns it.
// Cells changes doesn't show up until you call re_materialize.
// re_materialize is idempotent from visual perspective.
var Chunk = function(scene) {
	this.scene = scene;

	// tracer
	this.age = 0;
	this.new_plant_id = 0;

	// dummy material
	this.land = new THREE.Object3D();
	this.scene.add(this.land);

	// Chunk spatail
	this.size = 0.5;

	// Soil (Cell sim)
	this.soil = new Soil(this, this.size);
	this.seeds = [];

	// Light
	this.light = new Light(this, this.size);

	// Plants (bio-phys)
	this.children = [];
};

// Add standard plant seed.
Chunk.prototype.add_default_plant = function(pos) {
	return this.add_plant(
		pos,
		Math.pow(20e-3, 3) * 100, // allow 2cm cube for 100T)
		new Genome());
};

// pos :: THREE.Vector3 (z must be 0)
// energy :: Total starting energy for the new plant.
// genome :: genome for new plant
// return :: Plant
Chunk.prototype.add_plant = function(pos, energy, genome) {
	console.assert(Math.abs(pos.z) < 1e-3);

	// Torus-like boundary
	pos = new THREE.Vector3(
		(pos.x + 1.5 * this.size) % this.size - this.size / 2,
		(pos.y + 1.5 * this.size) % this.size - this.size / 2,
		pos.z);

	var shoot = new Plant(pos, this, energy, genome, this.new_plant_id);
	this.new_plant_id += 1;
	this.children.push(shoot);

	return shoot;
};

// pos :: THREE.Vector3
// return :: ()
Chunk.prototype.disperse_seed_from = function(pos, energy, genome) {
	console.assert(pos.z >= 0);
	// Discard seeds thrown from too low altitude.
	if(pos.z < 0.01) {
		return;
	}

	var angle = Math.PI / 3;

	var sigma = Math.tan(angle) * pos.z;

	// TODO: Use gaussian 
	var dx = sigma * 2 * (Math.random() - 0.5);
	var dy = sigma * 2 * (Math.random() - 0.5);

	this.seeds.push({
		pos: new THREE.Vector3(pos.x + dx, pos.y + dy, 0),
		energy: energy,
		genome: genome
	});
};

// Plant :: must be returned by add_plant
// return :: ()
Chunk.prototype.remove_plant = function(plant) {
	this.children = _.without(this.children, plant);
};

// return :: dict
Chunk.prototype.get_stat = function() {
	var stored_energy = sum(_.map(this.children, function(plant) {
		return plant.energy;
	}));

	return {
		'age/T': this.age,
		'plant': this.children.length,
		'stored/E': stored_energy
	};
};

// Retrieve current statistics about specified plant id.
// id :: int (plant id)
// return :: dict | null
Chunk.prototype.get_plant_stat = function(id) {
	var stat = null;
	_.each(this.children, function(plant) {
		if(plant.id === id) {
			stat = plant.get_stat();
		}
	});
	return stat;
};

// return :: array | null
Chunk.prototype.get_plant_genome = function(id) {
	var genome = null;
	_.each(this.children, function(plant) {
		if(plant.id === id) {
			genome = plant.get_genome();
		}
	});
	return genome;
};

// return :: object (stats)
Chunk.prototype.step = function() {
	this.age += 1;

	var t0 = 0;
	var sim_stats = {};

	t0 = now();
	_.each(this.children, function(plant) {
		plant.step();
	}, this);

	_.each(this.seeds, function(seed) {
		this.add_plant(seed.pos, seed.energy, seed.genome);
	}, this);
	this.seeds = [];
	sim_stats['plant/ms'] = now() - t0;

	t0 = now();
	this.light.step();
	sim_stats['light/ms'] = now() - t0;

	t0 = now();
	this.soil.step();
	sim_stats['soil/ms'] = now() - t0;

	return sim_stats;
};

// options :: dict(string, bool)
// return :: ()
Chunk.prototype.re_materialize = function(options) {
	// Throw away all children of pot.
	_.each(_.clone(this.land.children), function(three_cell_or_debug) {
		this.land.remove(three_cell_or_debug);
	}, this);

	// Materialize soil.
	var soil = this.soil.materialize();
	this.land.add(soil);

	// Materialize all Plant.
	_.each(this.children, function(plant) {
		this.land.add(plant.materialize(true));
	}, this);
};


Chunk.prototype.serialize = function() {
	var ser = {};
	ser['plants'] = _.map(this.children, function(plant) {
		var mesh = plant.materialize(true);

		return {
			'id': plant.id,
			'vertices': mesh.geometry.vertices,
			'faces': mesh.geometry.faces
		};
	}, this);
	ser['soil'] = this.soil.serialize();

	return ser;
};

// Kill plant with specified id.
Chunk.prototype.kill = function(id) {
	this.children = _.filter(this.children, function(plant) {
		return (plant.id !== id);
	});
};

// xs :: [num]
// return :: num
function sum(xs) {
	return _.reduce(xs, function(x, y) {
		return x + y;
	}, 0);
}

// xs :: [num]
// return :: num
function product(xs) {
	return _.reduce(xs, function(x, y) {
		return x * y;
	}, 1);
}

this.Chunk = Chunk;

})(this);
