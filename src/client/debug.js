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
                    bs.$set('debug', JSON.stringify(JSON.parse(data.encodeJSON()), null, 2));
                });
            },
            enter: function(biosphere) {
                console.log('entering', biosphere.biosphere_id);
                window.location.href = '/biosphere/' + biosphere.biosphere_id;
            }
        }
    });
    bs.update();

    let bs_timing = new Vue({
        el: '#trace_visualizer',
        data: {
        },
        methods: {
            refresh: function() {
                let trace = JSON.parse($('#trace_json').val());
                $('#timing').empty();

                let t0 = trace.start;
                let px_per_ns = 500 / (trace.end - trace.start);
                let put = (tr) => {
                    let bar = $('<div/>').css('margin-bottom', '4px');
                    let time = ((tr.end - tr.start) * 1e-9).toFixed(3) + 's';
                    bar.append($('<span/>').css('display', 'inline-block').css('width', (tr.start - t0) * px_per_ns + 'px'));
                    bar.append($('<span/>').css('display', 'inline-block').css('width', (tr.end - tr.start) * px_per_ns + 'px').css('background-color', '#B2DFDB').css('height', '24px').text(tr.name + ': ' + time));
                    $('#timing').append(bar);
                    _.each(tr.children, put);
                };
                put(trace);
            },
        },
    });
});
