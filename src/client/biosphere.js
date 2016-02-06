"use strict";

var Long = dcodeIO.Long;

class Client {
    constructor(bs_main, bs_time) {
        this.debug = (location.hash === '#debug');
        this.timestamp = 0;
        this.bs_main = bs_main;
        this.bs_time = bs_time;
    	this.init();

        this.paused = true;
        this.refresh_data();

        this.cells_proxy_data = null;
        this.cells_proxy_object = null;
        let geometry = new THREE.CylinderGeometry(0.01, 0.01, 0.5);
        let material = new THREE.MeshBasicMaterial( {color: 0xffff00} );
        this.cursor = new THREE.Mesh(geometry, material);
        this.cursor.rotateOnAxis(new THREE.Vector3(1, 0, 0), Math.PI / 2);
        this.cursor.visible = false;
        this.scene.add(this.cursor);

        this.frame_constructed = false;

        let _this = this;
        $(window).resize(() => {
            _this.renderer.setSize($('#viewport').width(), $('#viewport').height());
            _this.camera.aspect = $('#viewport').width() / $('#viewport').height();
            _this.camera.updateProjectionMatrix();
        });
    }

    refresh_data() {
        var _this = this;
        if (this.paused) {
            setTimeout(()=> {
                _this.refresh_data();
            }, 1000);
            return;
        }

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
            if (!_this.paused) {
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
                _this.bs_main.update_composition(data.stat);
                _this.cells_proxy_data = data.cells;
                _this.on_frame_received(data);
            }
            _this.refresh_data();
        });
    }

    set_inspeting(inspecting) {
        let _this = this;
        if (inspecting) {
            this.renderer.setClearColor('#888');
            if (this.cells_proxy_data === null) {
                return;
            }
            this.cells_proxy_object = new THREE.Object3D();
            _.each(this.cells_proxy_data, (cell_data, cell_ix) => {
                let geom = new THREE.SphereGeometry(0.05, 3, 2);
                let mat = new THREE.MeshBasicMaterial( {color: 0xffff00} );
                let sphere = new THREE.Mesh( geom, mat );
                sphere.cell_ix = cell_ix;
                sphere.position.set(cell_data.pos.x, cell_data.pos.y, cell_data.pos.z);
                this.cells_proxy_object.add(sphere);
            }, this);
            this.scene.add(this.cells_proxy_object);

            $('#viewport').on('mousemove', (ev) => {
                let raycaster = new THREE.Raycaster();
                let mouse = new THREE.Vector2();
                mouse.x = ( event.offsetX / ev.toElement.width ) * 2 - 1;
                mouse.y = - ( event.offsetY / ev.toElement.height ) * 2 + 1;
                raycaster.setFromCamera(mouse, _this.camera);
                let isects = raycaster.intersectObject(_this.cells_proxy_object, true);
                if (isects.length > 0) {
                    let cell_data = _this.cells_proxy_data[isects[0].object.cell_ix];
                    _this.notify_cell_selection(cell_data);

                    _this.cursor.visible = true;
                    _this.cursor.position.set(cell_data.pos.x, cell_data.pos.y, cell_data.pos.z + 0.25);
                }
            });
        } else {
            this.renderer.setClearColor('#eee');
            $('#viewport').off('mousemove');
            if (this.cells_proxy_object !== null) {
                this.scene.remove(this.cells_proxy_object);
                this.cells_proxy_object = null;
            }
            _this.cursor.visible = false;
        }
    }

    notify_cell_selection(cell_data) {
        this.bs_main.stats =
            JSON.stringify({
                cycle: cell_data.prop.cycle,
                genome: cell_data.prop.genome,
                quals: cell_data.prop.quals.map,
            }, null, '  ');
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

    	// start canvas
    	this.renderer = new THREE.WebGLRenderer({
    		antialias: true
    	});

    	this.renderer.setSize($('#viewport').width(), 600);
    	this.renderer.setClearColor('#eee');
    	$('#viewport').append(this.renderer.domElement);

    	// add mouse control (do this after canvas insertion)
    	this.controls = new THREE.TrackballControls(this.camera, this.renderer.domElement);
        this.controls.noZoom = false;
		this.controls.noPan = false;
        let _this = this;
    	this.controls.maxDistance = 15;
    }

    construct_frames(nx, ny) {
        if (this.frame_constructed) {
            return;
        }
        _.each(_.range(nx), (ix) => {
            _.each(_.range(ny), (iy) => {
                let box = new THREE.Mesh(
                    new THREE.CubeGeometry(1, 1, 2),
                    new THREE.MeshBasicMaterial({
                        wireframe: true,
                        color: '#fcc'
                    }));
                    box.position.x = 0.5 + ix;
                    box.position.y = 0.5 + iy;
                    box.position.z = 1;
                    this.scene.add(box);
            });
        });

        _.each(_.range(nx), (ix) => {
            _.each(_.range(ny), (iy) => {
                let box = new THREE.Mesh(
                    new THREE.CubeGeometry(0.92, 0.92, 0.02),
                    new THREE.MeshBasicMaterial({
                        color: '#ccc',
                        transparent: true,
                        opacity: 0.5,
                    }));
                    box.position.x = 0.5 + ix;
                    box.position.y = 0.5 + iy;
                    box.position.z = -0.02;
                    this.scene.add(box);
            });
        });

        _.each(_.range(nx + 1), (ix) => {
            let box = new THREE.Mesh(
                new THREE.CubeGeometry(0.03, ny, 0.03),
                new THREE.MeshBasicMaterial({
                    color: '#888'
                }));
                box.position.x = ix;
                box.position.y = ny / 2;
                box.position.z = -0.02;
                this.scene.add(box);
        });
        _.each(_.range(ny + 1), (iy) => {
            let box = new THREE.Mesh(
                new THREE.CubeGeometry(nx, 0.03, 0.03),
                new THREE.MeshBasicMaterial({
                    color: '#888'
                }));
                box.position.x = nx / 2;
                box.position.y = iy;
                box.position.z = -0.02;
                this.scene.add(box);
        });
        this.frame_constructed = true;
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

    google.charts.load("current", {packages:["corechart"]});

    Vue.component('bs-header', {
        props: ['biosphereName', 'loading'],
    });

    Vue.component('bs-composition', {
        props: ['stats'],
    });

    Vue.component('bs-inspector', {
        props: ['stats'],
    });

    Vue.component('bs-time', {
        template: '#time-template',
        props: ['state'],
        data: () => {
            return {
                is_playing: false,  // only applicable when is_running.
                current_timestamp: null,
                years: [],
            };
        },
        computed: {
            // For some reason, when I write ""() => ..." here, vue.js
            // fails to detect dependency and do not auto-update.
            play_visible: function() {
                return this.is_running && !this.is_playing;
            },
            pause_visible: function() {
                return  this.is_running && this.is_playing;
            },
            processing: function() {
                return !this.is_stopped && !this.is_running;
            },
            is_running: function() {
                return this.state === 1;
            },
            is_stopped: function() {
                return this.state === 2;
            }
        },
        methods: {
            start: function() {
                this.$parent.start_server();
            },
            stop: function() {
                this.$parent.stop_server();
            },
            play: function() {
                this.is_playing = true;
                client.paused = false;
            },
            pause: function() {
                this.is_playing = false;
                client.paused = true;
            },
        },
    });

    var bs_main = new Vue({
        el: 'body',
        data: {
            loading: true,
            biosphere_name: "",
            state: 0, // UNKNOWN
            stats: "",
            inspecting: false,
            nx: 0,
            ny: 0,
        },
        methods: {
            start_server: function() {
                var _this = this;
                this.state = 3; // T_RUN
                call_fe('change_exec', {
                    biosphere_id: document.biosphere_id,
                    target_state: 1, // RUNNING
                }, true).done((data) => {
                    _this.update();
                });
            },
            stop_server: function() {
                var _this = this;
                this.state = 4; // T_STOP
                call_fe('change_exec', {
                    biosphere_id: document.biosphere_id,
                    target_state: 0, // STOPPED
                }).done((data) => {
                    _this.update();
                });
            },
            // For some reason, () => doesn't work.
            update: function() {
                var _this = this;
                var biospheres = this.biospheres;
                this.loading = (this.biosphere_name === ""); // only show UI when loading name.
                call_fe('biospheres', {}).done(data => {
                    _this.loading = false;
                    var bs = _.find(data.biospheres, (biosphere) => {
                        return document.biosphere_id.eq(biosphere.biosphere_id);
                    });
                    console.log('This biosphere:', bs);
                    _this.state = bs.state;
                    _this.biosphere_name = bs.name;
                    _this.nx = bs.nx;
                    _this.ny = bs.ny;
                    client.construct_frames(bs.nx, bs.ny);
                    if (bs.state === 3 || bs.state === 4) {
                        // Continue to reload when it's transitioning.
                        setTimeout(() => {
                            _this.update();
                        }, 5000);
                    }
                });
            },
            update_composition: function(stat) {
                var arr = [['Kind', '#grains']];
                arr = arr.concat(_.map(stat, (num, kind) => {return [kind, num];}));
                var data = google.visualization.arrayToDataTable(arr);
                var options = {
                    title: 'Composition',
                    pieHole: 0.4,
                };
                var chart = new google.visualization.PieChart($('#grain_composition')[0]);
                chart.draw(data, options);
            },
            enter_inspect: function() {
                this.inspecting = true;
                client.set_inspeting(this.inspecting);
            },
            exit_inspect: function() {
                this.inspecting = false;
                client.set_inspeting(this.inspecting);
            },
        }
    });
    bs_main.update();

    // TODO: Deprecate this access pattern
    var bs_time = bs_main.$children[1];
    var client = new Client(bs_main, bs_time);
    client.animate();
});
