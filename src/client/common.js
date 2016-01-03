"use strict";
// ECMAscript 6

var ProtoBuf = dcodeIO.ProtoBuf;

$.ajax('/static/frontend.proto').done(data => {
    console.log('Creating proto builder');
    document.proto_builder = ProtoBuf.loadProto(data, 'frontend.proto');
});

// Return ajax future (that is returned by $.ajax) for calling jsonpb RPC.
function call_fe(rpc_name, data, needs_auth) {
    if (document.proto_builder === undefined) {
        return {
            "done": handler => {
                setTimeout(() => {
                    call_fe(rpc_name, data, needs_auth).done(handler);
                }, 250);
            }
        };
    }
    var resp_message = document.proto_builder.build('api.' + convert_rpc_name_to_proto(rpc_name) + 'S');

    if (needs_auth) {
        if (document.googleUser) {
            data["auth"] = {
                id_token: document.googleUser.getAuthResponse().id_token,
            };
        } else {
            console.warn("Not adding auth token despite requested, because sign in widget failed to load.");
        }
    }

    // jquery <-> raw XHR compatibility layer.
    var xhr = ProtoBuf.Util.XHR();
    xhr.open(
        /* method */ "GET",
        /* file */ '/api/' + rpc_name + '?pb=' + JSON.stringify(data),
        /* async */ true
    );
    xhr.responseType = "arraybuffer";
    var jq_promise = {
        receiver: null,
        message: null,
    };
    xhr.onload = function(evt) {
        var msg = resp_message.decode(xhr.response);
        console.log(msg);

        if (jq_promise.receiver !== null) {
            jq_promise.receiver(msg);
        } else {
            jq_promise.message = msg;
        }
    }
    xhr.send(null);

    return {
        "done": handler => {
            if (jq_promise.message !== null) {
                // Message received before .done(...) is called.
                handler(jq_promise.message);
            } else {
                // Message not yet available -> set receiver.
                jq_promise.receiver = handler;
            }
        }
    };
}

function convert_rpc_name_to_proto(rpc_name) {
    var result = '';
    _.each(rpc_name.split('_'), word => {
        result += word[0].toUpperCase() + word.slice(1);
    })
    return result;
}
