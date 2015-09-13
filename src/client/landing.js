// ECMAscript 6

$(document).ready(() => {
    console.log("Hoge");

    var bs = new Vue({
        el: '#biospheres',
        data: {
            biospheres: [
                {
                    name: "Test World",
                    num_cores: 123,
                    num_ticks: 23456
                },
                {
                    name: "Big World 2",
                    num_cores: 45,
                    num_ticks: 3232132
                }
            ]
        }
    });
});
