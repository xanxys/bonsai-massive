"use strict";
// ECMAscript 6

let api_key = "AIzaSyDV2xeiMq0PAUNTE-fSIm_np8lojyzhONE";
let scopes = 'https://www.googleapis.com/auth/bigquery https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/cloud-platform.read-only';

function authAndCallBq() {
    return document.googleUser.grant({'fetch_basic_profile': false, 'scope': scopes}).then(
        (success) => {
            console.log('Scopes', success.getGrantedScopes());
            return callBq();
        },
        (fail) => {
            console.log('Permission request failed', fail);
        }
    );
}

function callBq() {
    gapi.client.setApiKey(api_key);
    return gapi.client.load('bigquery', 'v2').then(function() {
        return gapi.client.bigquery.jobs.query({
            'projectId': 'bonsai-genesis',
            'query': 'select count(*) from [platform.stepping]'
        });
    });
}

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
                authAndCallBq().then((succ)=>{
                    bs_stepping.$set('response', JSON.stringify(succ, null, 2));
                }, (fail) => {
                    console.log('Failed to do BQ');
                });
            }
        }
    });
});
