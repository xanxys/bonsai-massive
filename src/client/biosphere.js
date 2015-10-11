"use strict";

// Experimental grain interaction
class Grain {
    constructor() {
        this.position = new THREE.Vector3(
            Math.random(), Math.random(), Math.random() * 0.3);
        this.velocity = new THREE.Vector3(0, 0, 0);
    }
}

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
class Client {
    constructor() {
        this.debug = (location.hash === '#debug');
        this.grains = _.map(_.range(100), () => {
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

        this.grains_objects = _.map(this.grains, (grain) => {
            var ball = new THREE.Mesh(
                new THREE.IcosahedronGeometry(0.01),
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
        var dt = 1/30;
        var accel = new THREE.Vector3(0, 0, -0.01);
        var new_pos = new THREE.Vector3();
        _.each(this.grains, (grain) => {
            new_pos.copy(grain.position);
            new_pos.add(grain.velocity.clone().multiplyScalar(dt));
            new_pos.add(accel.clone().multiplyScalar(0.5 * dt * dt));

            // Resolve collisions.

            // Update.
            grain.velocity.copy(new_pos.clone().sub(grain.position).divideScalar(dt));
            grain.position.copy(new_pos);
        });
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
