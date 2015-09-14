// ECMAscript 6

$(document).ready(() => {
    console.log("Hoge");

    var bs = new Vue({
        el: '#biospheres',
        data: {
            biospheres: [
                {
                    name: "Test World",
                    numCores: 123,
                    numTicks: 23456
                },
                {
                    name: "Big World 2",
                    numCores: 45,
                    numTicks: 3232132
                }
            ]
        },
        methods: {
            update: function() {
                var $data = this.$data;
                $.ajax('/api/worlds').done(data => {
                    $data.results = data;
                });
            }
        }
    });
});
