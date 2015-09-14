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
});
