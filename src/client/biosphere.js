"use strict";

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
class Client {
    constructor(playback_data) {
        this.debug = (location.hash === '#debug');
        this.timestamp = 0;
    	this.init();

        var _this = this;
        $.ajax('/api/biosphere_frames', {
            "data": {
                "pb": JSON.stringify({})
            }
        }).done(function(data) {
            _this.on_frame_received(data);
        });
    }

    on_frame_received(data) {
        let geom = new THREE.BufferGeometry();
        let vertices = new Float32Array(data.content.vertices.length * 3);
        _.each(data.content.vertices, (vertex, ix) => {
            vertices[ix * 3 + 0] = vertex.px;
            vertices[ix * 3 + 1] = vertex.py;
            vertices[ix * 3 + 2] = vertex.pz;
        });
        geom.addAttribute('position', new THREE.BufferAttribute(vertices, 3));

        let material = new THREE.MeshBasicMaterial({color: 0xdddddd, side:THREE.DoubleSide });
        let mesh = new THREE.Mesh(geom, material);

        this.scene.add(mesh);
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

    	this.renderer.render(this.scene, this.camera);
    }
}

// run app
$(document).ready(function() {
    new Client().animate();
});
