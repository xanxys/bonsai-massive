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
        'packages': ['corechart', 'table', 'timeline']
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

    // Time hierarchy:
    // Whole (determined by query_summary) -> Window (around center) -> View
    let bs_stepping = new Vue({
        el: '#card_stepping',
        data: {
            stepping_view_min_str: "",
            stepping_view_max_str: "",
            center_ratio: 5000,
            window_min: 0,
            window_max: 0,
            view_min_ratio: 0,
            view_max_ratio: 10000,
            loading_detail: false,
        },
        computed: {
            view_min: function() {
                let t = this.view_min_ratio * 1e-4;
                return this.window_min * (1 - t) + this.window_max * t;
            },
            view_max: function() {
                let t = this.view_max_ratio * 1e-4;
                return this.window_min * (1 - t) + this.window_max * t;
            },
            view_min_str: {
                get: function() {
                    return new Date(this.view_min).toISOString();
                },
                set: function(iso_str) {
                    let d = new Date(iso_str);
                    if (!isNaN(d)) {
                        this.view_min = d * 1.0;
                    }
                }
            },
            view_max_str: {
                get: function() {
                    return new Date(this.view_max).toISOString();
                },
                set: function(iso_str) {
                    let d = new Date(iso_str);
                    if (!isNaN(d)) {
                        this.view_max = d * 1.0;
                    }
                }
            }
        },
        methods: {
            // For some reason, () => doesn't work.
            update_whole: function() {
                let query_summary = `
                select
                  unix_millis(timestamp_trunc(start_at, hour)) as time_bucket,
                  count(*) as num_events
                from
                  \`platform.stepping\`
                group by time_bucket
                order by time_bucket
                `;

                let vm = this;

                authAndCallBq(query_summary).then((resp) => {
                    let data = new google.visualization.DataTable();
                    data.addColumn('datetime', 'bucket');
                    data.addColumn('number', '#events');
                    let ts = [];
                    _.each(resp.result.rows, (row) => {
                        let t = parseFloat(row.f[0].v);
                        data.addRow([new Date(t), parseInt(row.f[1].v)]);
                        ts.push(t);
                    });
                    let chart = new google.visualization.Table(document.getElementById('stepping_summary'));
                    chart.draw(data);
                    google.visualization.events.addListener(chart, 'select', (ev) => {
                        let selected_ts = _.map(chart.getSelection(), (sel) => ts[sel.row]);
                        vm.select_bucket(selected_ts);
                    });
                });
            },
            select_bucket: function(selection) {
                let vm = this;
                vm.view_min_ratio = 0;
                vm.view_max_ratio = 10000;
                vm.window_min = _.min(selection);
                vm.window_max = _.max(selection) + 3600e3; // one bucket + 1hr

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
                  where ${vm.window_min} <= unix_millis(start_at) and unix_millis(start_at) <= ${vm.window_max}
                  group by
                    machine_ip,
                    chunk_id
                )`;
                this.loading_detail = true;
                authAndCallBq(query).then((resp) => {
                    bs_stepping.chart = new google.visualization.Timeline(document.getElementById('stepping_timeline'));
                    vm.stepping_rows = resp.result.rows;
                    vm.loading_detail = false;
                    vm.maybe_update_range();
                });
            },
            maybe_update_range:  function() {
                if (this.chart === undefined) {
                    return;
                }
                let min_d = this.view_min;
                let max_d = this.view_max;
                if (isNaN(new Date(min_d)) || isNaN(new Date(max_d))) {
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
                let rows = [];
                _.each(this.stepping_rows, (row_location) => {
                    let location = row_location.f[0].v + "/" + row_location.f[1].v;
                    _.each(row_location.f[2].v, (row_ev) => {
                        let timestamp_start = parseInt(row_ev.v.f[0].v);
                        let timestamp_end = parseInt(row_ev.v.f[1].v);
                        // Don't show if there's no overlap between event span & current view.
                        if (timestamp_end < min_d || max_d < timestamp_start) {
                            return;
                        }
                        timestamp_start = Math.max(timestamp_start, min_d);
                        timestamp_end = Math.min(timestamp_end, max_d);

                        let ev_type = row_ev.v.f[2].v;
                        let ev_label = `${ev_type} (${row_ev.v.f[3].v})`;
                        rows.push([location, ev_type, ev_label, new Date(timestamp_start), new Date(timestamp_end)]);
                    });
                });
                const max_num_rows = 400;
                if (rows.length > max_num_rows) {
                    console.log('Warning: some rows are not shown because of threshold', max_num_rows);
                }
                dataTable.addRows(rows.slice(0, max_num_rows));
                // Setting viewWindow doesn't work in timeline chart.
                this.chart.draw(dataTable, {
                    height: 600,
                    hAxis: {
                        minValue: new Date(min_d),
                        maxValue: new Date(max_d)
                    }
                });
            },
        }
    });
    bs_stepping.$watch('view_min', () => {bs_stepping.maybe_update_range();});
    bs_stepping.$watch('view_max', () => {bs_stepping.maybe_update_range();});
});
