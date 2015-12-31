"use strict";
// ECMAscript 6

$(document).ready(() => {
    var bs = new Vue({
        el: '#debug',
        data: {
            debug: ""
        },
        methods: {
            // For some reason, () => doesn't work.
            update: function() {
                call_fe('debug', {}).done(data => {
                    bs.$set('debug', JSON.stringify(data, null, 2));
                });
            },
            enter: function(biosphere) {
                console.log('entering', biosphere.biosphere_id);
                window.location.href = '/biosphere/' + biosphere.biosphere_id;
            }
        }
    });
    bs.update();
});
