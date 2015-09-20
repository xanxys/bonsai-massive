// ECMAscript 6

$(document).ready(() => {
    var bs = new Vue({
        el: '#biospheres',
        data: {
            biospheres: []
        },
        methods: {
            update: () => {
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

    })

    $('#create_biosphere_yes').click(() => {
        console.log('CREATE');
        $('#create_biosphere_dialog').hide();
    });
    $('#create_biosphere_no').click(() => {
        $('#create_biosphere_dialog').hide();
    });
});
