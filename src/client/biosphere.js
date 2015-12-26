"use strict";


// Return ajax future (that is returned by $.ajax) for calling jsonpb RPC.
function call_fe(rpc_name, data) {
    return $.ajax('/api/' + rpc_name, {
        "data": {
            "pb": JSON.stringify(data)
        }
    });
}

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
        call_fe('biosphere_frames', {
            "biosphere_id": document.biosphere_id,
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

    on_frame_received(data) {
        let geom = new THREE.BufferGeometry();
        let vertices = new Float32Array(data.content.vertices.length * 3);
        let vertices_color = new Float32Array(data.content.vertices.length * 3);
        _.each(data.content.vertices, (vertex, ix) => {
            vertices[ix * 3 + 0] = vertex.px;
            vertices[ix * 3 + 1] = vertex.py;
            vertices[ix * 3 + 2] = vertex.pz;
            vertices_color[ix * 3 + 0] = vertex.r;
            vertices_color[ix * 3 + 1] = vertex.g;
            vertices_color[ix * 3 + 2] = vertex.b;
        });
        geom.addAttribute('position', new THREE.BufferAttribute(vertices, 3));
        geom.addAttribute('color', new THREE.BufferAttribute(vertices_color, 3));

        let material = new THREE.MeshBasicMaterial({
            vertexColors: THREE.VertexColors,
            side: THREE.DoubleSide,
        });
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

    	this.renderer.setSize($('#main').width(), 600);
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
    var biosphere_id = document.location.pathname.split('/')[2];
    document.biosphere_id = biosphere_id;

    $('#button_start').click(() => {
        call_fe('biosphere_frames', {
            'biosphere_id': document.biosphere_id,
            "ensure_start": true,
        }).done(data => {
            console.log(data);
        })
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
                call_fe('/biospheres', {}).done(data => {
                    this.loading = false;
                    var name = _.find(data.biospheres, (biosphere) => {
                        return biosphere.biosphere_id === biosphere_id;
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
