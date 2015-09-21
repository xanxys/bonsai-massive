// ECMAscript 6

$(document).ready(() => {
    var bs = new Vue({
        el: '#biospheres',
        data: {
            biospheres: []
        },
        methods: {
            // For some reason, () => doesn't work.
            update: function() {
                var biospheres = this.biospheres;
                $.ajax('/api/biospheres', {
                    "data": {
                        "pb": JSON.stringify({})
                    }
                }).done(data => {
                    bs.$set('biospheres', data.biospheres);
                });
            }
        }
    });
    bs.update();

    $('#create_biosphere').click(() => {
        $('#create_biosphere_dialog').show();

        var bs = new Vue({
            el: '#create_biosphere_dialog',
            data: {
                name: ""
            },
            computed: {
                // For some reason, when I write ""() => ..." here, vue.js
                // fails to detect dependency and do not auto-update.
                create_ready: function() {
                    return this.name != '';
                }
            },
            methods: {
                // For some reason, () => doesn't get this.name properly.
                create: function() {
                    console.log('CREATE', this.name);
                    var request = {
                        type: "ADD",
                        desc: {
                            name: this.name
                        }
                    };
                    $.ajax('/api/biosphere_delta', {
                        "data": {
                            "pb": JSON.stringify(request)
                        }
                    }).done(data => {
                        console.log(data);
                    })
                    $('#create_biosphere_dialog').hide();
                },
                cancel: () => {
                    $('#create_biosphere_dialog').hide();
                }
            }
        });
    });
});
