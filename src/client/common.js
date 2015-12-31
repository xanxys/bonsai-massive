"use strict";
// ECMAscript 6

// Return ajax future (that is returned by $.ajax) for calling jsonpb RPC.
function call_fe(rpc_name, data, needs_auth) {
    if (needs_auth) {
        if (document.googleUser) {
            data["auth"] = {
                id_token: document.googleUser.getAuthResponse().id_token,
            };
        } else {
            console.warn("Not adding auth token despite requested, because sign in widget failed to load.");
        }
    }
    return $.ajax('/api/' + rpc_name, {
        "data": {
            "pb": JSON.stringify(data)
        }
    });
}
