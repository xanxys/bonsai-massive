"use strict";

var Long = dcodeIO.Long;

// Separate into
// 1. master class (holds chunk worker)
// 1': 3D GUI class
// 2. Panel GUI class
class Client {
    constructor(bs_time) {
        this.debug = (location.hash === '#debug');
        this.timestamp = 0;
        this.bs_time = bs_time;
    	this.init();

        this.refresh_data();

        let _this = this;
        $(window).resize(() => {
            _this.renderer.setSize($('#main').width(), $('#main').height());
            _this.camera.aspect = $('#main').width() / $('#main').height();
            _this.camera.updateProjectionMatrix();
        });
    }

    refresh_data() {
        var _this = this;
        var c_dir = this.camera.getWorldDirection();
        call_fe('biosphere_frames', {
            biosphere_id: document.biosphere_id,
            visible_region: {
                px: this.camera.position.x,
                py: this.camera.position.y,
                pz: this.camera.position.z,
                dx: c_dir.x,
                dy: c_dir.y,
                dz: c_dir.z,
                half_angle: this.get_cone_half_angle(),
            },
        }).done(function(data) {
            var current_day = Math.floor(data.content_timestamp/5000);
            var years = _.map(_.range(Math.ceil(data.content_timestamp/(5000 * 10))), (year_index) => {
                var sol_begin = 10 * year_index;
                var sol_end = Math.min(10 * (year_index + 1), Math.ceil(data.content_timestamp/5000));

                var sols = _.map(_.range(sol_begin, sol_end), (sol_index) => {
                    return {
                        "index": sol_index,
                        "index_in_year": sol_index % 10,
                        "active": sol_index === current_day
                    };
                });
                return {
                    "index": year_index,
                    "sols": sols,
                };
            });

            _this.bs_time.$set('current_timestamp', data.content_timestamp);
            _this.bs_time.$set('years', years);
            _this.on_frame_received(data);
            _this.refresh_data();
        });
    }

    // Get half angle of cone that contains camera. The angle is same as
    // diagonal fov / 2, in radians.
    get_cone_half_angle() {
        let vert_half_sz = Math.tan(this.camera.fov / 180 * Math.PI / 2);
        let horz_half_sz = vert_half_sz * this.camera.aspect;
        let diag_half_sz = Math.hypot(vert_half_sz, horz_half_sz);
        return Math.atan(diag_half_sz);
    }

    on_frame_received(data) {
        let geom = new THREE.BufferGeometry();
        let vertices = new Float32Array(data.content.vertices.length * 3);
        let vertices_color = new Float32Array(data.content.vertices.length * 3);
        let indices = new Uint32Array(data.content.indices.length);
        _.each(data.content.vertices, (vertex, ix) => {
            vertices[ix * 3 + 0] = vertex.px;
            vertices[ix * 3 + 1] = vertex.py;
            vertices[ix * 3 + 2] = vertex.pz;
            vertices_color[ix * 3 + 0] = vertex.r;
            vertices_color[ix * 3 + 1] = vertex.g;
            vertices_color[ix * 3 + 2] = vertex.b;
        });
        _.each(data.content.indices, (v_index, ix) => {
            indices[ix] = v_index;
        });
        geom.setIndex(new THREE.BufferAttribute(indices, 1));
        geom.addAttribute('position', new THREE.BufferAttribute(vertices, 3));
        geom.addAttribute('color', new THREE.BufferAttribute(vertices_color, 3));

        let material = new THREE.MeshBasicMaterial({
            vertexColors: THREE.VertexColors,
            side: THREE.DoubleSide,
        });
        let mesh = new THREE.Mesh(geom, material);

        if (this.received_mesh !== undefined) {
            this.scene.remove(this.received_mesh);
            this.received_mesh.geometry.dispose();
            this.received_mesh.material.dispose();
        }
        this.received_mesh = mesh;
        this.scene.add(mesh);
    }

    // return :: ()
    init() {
    	this.camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.01, 30);
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
    		new THREE.IcosahedronGeometry(15, 1),
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

    	this.renderer.setSize($('#main').width(), 600);
    	this.renderer.setClearColor('#eee');
    	$('#main').append(this.renderer.domElement);

    	// add mouse control (do this after canvas insertion)
    	this.controls = new THREE.TrackballControls(this.camera, this.renderer.domElement);
        this.controls.noZoom = false;
		this.controls.noPan = false;
        let _this = this;
    	this.controls.maxDistance = 15;
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
    document.biosphere_id = Long.fromString(
        document.location.pathname.split('/')[2], true);

    $('#button_start').click(() => {
        call_fe('change_exec', {
            biosphere_id: document.biosphere_id,
            target_state: 1, // RUNNING
        }, true);
    });

    $('#button_stop').click(() => {
        call_fe('change_exec', {
            biosphere_id: document.biosphere_id,
            target_state: 0, // STOPPED
        });
    });

    var bs_main = new Vue({
        el: '#header',
        data: {
            loading: true,
            biosphere_name: "",
        },
        methods: {
            // For some reason, () => doesn't work.
            update: function() {
                var biospheres = this.biospheres;
                this.loading = true;
                call_fe('biospheres', {}).done(data => {
                    this.loading = false;
                    var name = _.find(data.biospheres, (biosphere) => {
                        return document.biosphere_id.eq(biosphere.biosphere_id);
                    }).name;
                    bs_main.$set('biosphere_name', name);
                });
            },
            enter: function(biosphere) {
                console.log('entering', biosphere.biosphere_id);
                window.location.href = '/biosphere/' + biosphere.biosphere_id;
            }
        }
    });
    var bs_time = new Vue({
        el: '#time',
        data: {
            current_timestamp: null,
            years: [],
        },
    });
    bs_main.update();

    new Client(bs_time).animate();
});
