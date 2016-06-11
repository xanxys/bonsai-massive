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
            response: ""
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
                        format("%s(%d)", event_type, chunk_timestamp) as label
                      )
                    ) as events
                  from
                    \`platform.stepping\`
                  group by
                    machine_ip,
                    chunk_id
                )`;

                authAndCallBq(query).then((resp) => {
                    let container = document.getElementById('stepping_timeline');
                    let chart = new google.visualization.Timeline(container);
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
                        type: 'date',
                        id: 'Start'
                    });
                    dataTable.addColumn({
                        type: 'date',
                        id: 'End'
                    });
                    _.each(resp.result.rows, (row_location) => {
                        let location = row_location.f[0].v + "/" + row_location.f[1].v;

                        _.each(row_location.f[2].v, (row_ev) => {
                            let timestamp = parseInt(row_ev.v.f[0].v);
                            let ev = row_ev.v.f[1].v;
                            dataTable.addRow(
                                [location, ev, new Date(timestamp), new Date(timestamp + 100)]
                            );
                        });
                    })
                    chart.draw(dataTable);
                }, (fail) => {
                    console.log('Failed to do BQ');
                });
            }
        }
    });
});
