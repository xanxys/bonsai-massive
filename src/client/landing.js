"use strict";
// ECMAscript 6


function onSignIn(googleUser) {
    console.log(googleUser);
    document.googleUser = googleUser;
    //let id_token = googleUser.getAuthResponse().id_token;
}

$(document).ready(() => {
    var bs = new Vue({
        el: '#biospheres',
        data: {
            biospheres: [],
            loading: true
        },
        methods: {
            // For some reason, () => doesn't work.
            update: function() {
                var biospheres = this.biospheres;
                this.loading = true;
                call_fe('biospheres', {}).done(data => {
                    this.loading = false;
                    bs.$set('biospheres', data.biospheres);
                });
            },
            enter: function(biosphere) {
                console.log('entering', biosphere.biosphere_id);
                window.location.href = '/biosphere/' + biosphere.biosphere_id;
            }
        }
    });
    bs.update();

    $('#create_biosphere').click(() => {
        $('#create_biosphere').hide();
        $('#create_biosphere_dialog').show();
        $('#create_biosphere_name_input').focus();

        var bs = new Vue({
            el: '#create_biosphere_dialog',
            data: {
                name: "",
            },
            computed: {
                // For some reason, when I write ""() => ..." here, vue.js
                // fails to detect dependency and do not auto-update.
                create_ready: function() {
                    return this.name != '';
                },
                est_price_usd: function() {
                    return 0.015 * (this.nx * this.ny) / 5;
                },
            },
            methods: {
                // For some reason, () => doesn't get this.name properly.
                create: function() {
                    call_fe('add_biosphere',  {
                        test_only: false,
                        config: {
                            name: this.name,
                            nx: this.nx,
                            ny: this.ny,
                        }
                    }, true).done(data => {
                        console.log(data);
                    })
                    $('#create_biosphere_dialog').hide();
                    $('#create_biosphere').show();
                },
                cancel: () => {
                    $('#create_biosphere_dialog').hide();
                    $('#create_biosphere').show();
                }
            }
        });
    });
});
