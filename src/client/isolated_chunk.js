importScripts('./underscore.js');
importScripts('./three.js');
importScripts('./chunk.js');
importScripts('./genome.js');

var ChunkServer = function() {
	var _this = this;
	
	var scene = new THREE.Scene();
	this.chunk = new Chunk(scene);

	// Should be moved to bonsai.js
	var stress = false;
	if(!stress) {
		_.each(_.range(-2, 3), function(ix) {
			_.each(_.range(-2, 3), function(iy) {
				this.current_plant = _this.chunk.add_default_plant(
					new THREE.Vector3(ix * 0.07, iy * 0.07, 0));
			});
		});
	} else {
		_.each(_.range(-15, 16), function(ix) {
			_.each(_.range(-15, 16), function(iy) {
				this.current_plant = _this.chunk.add_default_plant(
					new THREE.Vector3(ix * 0.01, iy * 0.01, 0));
			});
		});
	}

	self.addEventListener('message', function(ev) {
		try {
			if(ev.data.type === 'step') {
				var sim_stat = _this.chunk.step();
				self.postMessage({
					type: 'stat-sim',
					data: sim_stat
				});
			} else if(ev.data.type === 'kill') {
				_this.chunk.kill(ev.data.data.id);
			} else if(ev.data.type === 'serialize') {
				self.postMessage({
					type: 'serialize',
					data: _this.chunk.serialize()
				});
			} else if(ev.data.type === 'stat') {
				self.postMessage({
					type: 'stat-chunk',
					data: _this.chunk.get_stat()
				});
			} else if(ev.data.type === 'stat-plant') {
				self.postMessage({
					type: 'stat-plant',
					data: {
						id: ev.data.data.id,
						stat: _this.chunk.get_plant_stat(ev.data.data.id)
					}
				});
			} else if(ev.data.type === 'genome-plant') {
				self.postMessage({
					type: 'genome-plant',
					data: {
						id: ev.data.data.id,
						genome: _this.chunk.get_plant_genome(ev.data.data.id)
					}
				});
			}
		} catch(e) {
			self.postMessage({
				type: 'exception',
				data: e.message
			});
		}
	});
};

new ChunkServer();
