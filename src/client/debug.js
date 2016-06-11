"use strict";
// ECMAscript 6

let api_key = "AIzaSyDV2xeiMq0PAUNTE-fSIm_np8lojyzhONE";
let scopes = 'https://www.googleapis.com/auth/bigquery https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/cloud-platform.read-only';

function authAndCallBq(query) {
    if (document.googleUser.hasGrantedScopes(scopes)) {
        return callBq(query);
    }
    return document.googleUser.grant({
        'fetch_basic_profile': false,
        'scope': scopes
    }).then(
        (success) => {
            console.log('Scopes', success.getGrantedScopes());
            return callBq(query);
        },
        (fail) => {
            console.log('Permission request failed', fail);
        }
    );
}

function callBq(query) {
    gapi.client.setApiKey(api_key);
    return gapi.client.load('bigquery', 'v2').then(function() {
        return gapi.client.bigquery.jobs.query({
            'projectId': 'bonsai-genesis',
            'useLegacySql': false,
            'query': query,
        });
    });
}

$(document).ready(() => {
    google.charts.load('current', {
        'packages': ['timeline']
    });

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
            }
        }
    });
    bs.update();

    let bs_stepping = new Vue({
        el: '#card_stepping',
        data: {
            response: "",
            stepping_min_date_str: "",
            stepping_max_date_str: "",
            st_min_ratio: 0,
            st_max_ratio: 1,
        },
        methods: {
            // For some reason, () => doesn't work.
            update: function() {
                let query = `select
                  machine_ip,
                  chunk_id,
                  array(
                    select event
                    from unnest(events) as event
                    order by event.start_ms) as events
                from (
                  select
                    machine_ip,
                    chunk_id,
                    array_agg(
                      struct(
                        unix_millis(start_at) as start_ms,
                        unix_millis(end_at) as end_ms,
                        event_type,
                        chunk_timestamp
                      )
                    ) as events
                  from
                    \`platform.stepping\`
                  group by
                    machine_ip,
                    chunk_id
                )`;
                let vm = this;

                authAndCallBq(query).then((resp) => {
                    let container = document.getElementById('stepping_timeline');
                    bs_stepping.chart = new google.visualization.Timeline(container);
                    vm.stepping_rows = resp.result.rows;

                    var min_time_ms = 1e20;
                    var max_time_ms = 0;
                    _.each(resp.result.rows, (row_location) => {
                        _.each(row_location.f[2].v, (row_ev) => {
                            let timestamp_start = parseInt(row_ev.v.f[0].v);
                            let timestamp_end = parseInt(row_ev.v.f[1].v);
                            min_time_ms = Math.min(min_time_ms, timestamp_start);
                            max_time_ms = Math.max(max_time_ms, timestamp_end);
                        });
                    });
                    vm.stepping_min_date_str = new Date(min_time_ms).toISOString();
                    vm.stepping_max_date_str = new Date(max_time_ms).toISOString();
                }, (fail) => {
                    console.log('Failed to do BQ');
                });
            }
        }
    });

    let maybe_update_range = () => {
        let min_d = new Date(bs_stepping.stepping_min_date_str);
        let max_d = new Date(bs_stepping.stepping_max_date_str);
        if (isNaN(min_d) || isNaN(max_d)) {
            return;
        }

        let dataTable = new google.visualization.DataTable();
        dataTable.addColumn({
            type: 'string',
            id: 'Location'
        });
        dataTable.addColumn({
            type: 'string',
            id: 'Event'
        });
        dataTable.addColumn({
            type: 'string',
            id: 'EventFull',
            role: 'tooltip'
        });
        dataTable.addColumn({
            type: 'date',
            id: 'Start'
        });
        dataTable.addColumn({
            type: 'date',
            id: 'End'
        });
        _.each(bs_stepping.stepping_rows, (row_location) => {
            let location = row_location.f[0].v + "/" + row_location.f[1].v;
            _.each(row_location.f[2].v, (row_ev) => {
                let timestamp_start = parseInt(row_ev.v.f[0].v);
                let timestamp_end = parseInt(row_ev.v.f[1].v);
                // Don't show if there's no overlap between event span & current view.
                if (new Date(timestamp_end) < min_d || max_d < new Date(timestamp_start)) {
                    return;
                }
                timestamp_start = Math.max(timestamp_start, min_d);
                timestamp_end = Math.min(timestamp_end, max_d);

                let ev_type = row_ev.v.f[2].v;
                let ev_label = `${ev_type} (${row_ev.v.f[3].v})`;
                dataTable.addRow([location, ev_type, ev_label, new Date(timestamp_start), new Date(timestamp_end)]);
            });
        });
        // Setting viewWindow doesn't work in timeline chart.
        bs_stepping.chart.draw(dataTable, {
            height: 600,
            hAxis: {minValue: min_d, maxValue: max_d}
        });
    };
    bs_stepping.$watch('stepping_min_date_str', maybe_update_range);
    bs_stepping.$watch('stepping_max_date_str', maybe_update_range);
});
