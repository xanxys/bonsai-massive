package main

import (
	"./api"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"mime"
	"net/http"
	"time"
)

// MaybeExtractQ extracts proto from HTTP request and returns it.
// Nil indicates failure, and appropriate status / description is written
// to response.
func MaybeExtractQ(w http.ResponseWriter, r *http.Request, defaultQ proto.Message) *proto.Message {
	q := proto.Clone(defaultQ)
	err := r.ParseForm()
	if err != nil {
		http.NotFound(w, r)
		fmt.Fprintf(w, "strange query %v", err)
		return nil
	}
	pb := r.Form.Get("pb")
	if pb == "" {
		http.NotFound(w, r)
		fmt.Fprintf(w, "Non-empty pb param required")
		return nil
	}
	err = jsonpb.UnmarshalString(pb, q)
	if err != nil {
		fmt.Fprintf(w, "Failed to parse pb param %v", err)
		return nil
	}
	return &q
}

func WriteS(
	w http.ResponseWriter, r *http.Request, s proto.Message, e error) {
	if e != nil {
		http.NotFound(w, r)
		fmt.Fprintf(w, "internal error: %v", e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	marshaler := jsonpb.Marshaler{
		EnumsAsString: true,
		Indent:        "",
	}
	err := marshaler.Marshal(w, s)
	if err != nil {
		fmt.Fprintf(w, "{\"error\": \"Internal error: failed to encode return pb\"}")
	}
}

func main() {
	const port = 8000
	fmt.Printf("Starting frontend server http://localhost:%d\n", port)
	mime.AddExtensionType(".svg", "image/svg+xml")

	fe := NewFeService()

	// Periodic service.
	go func() {
		for {
			fe.HandleApplyChunks()
			time.Sleep(10 * time.Second)
		}
	}()

	// Dispatchers.
	http.HandleFunc("/api/debug", func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, &api.DebugQ{})
		if q != nil {
			s, e := fe.HandleDebug((*q).(*api.DebugQ))
			WriteS(w, r, s, e)
		}
	})
	http.HandleFunc("/api/biospheres", func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, &api.BiospheresQ{})
		if q != nil {
			s, e := fe.HandleBiospheres((*q).(*api.BiospheresQ))
			WriteS(w, r, s, e)
		}
	})
	http.HandleFunc("/api/biosphere_delta", func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, &api.BiosphereDeltaQ{})
		if q != nil {
			s, e := fe.HandleBiosphereDelta((*q).(*api.BiosphereDeltaQ))
			WriteS(w, r, s, e)
		}
	})
	http.HandleFunc("/api/biosphere_frames", func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, &api.BiosphereFramesQ{})
		if q != nil {
			s, e := fe.HandleBiosphereFrames((*q).(*api.BiosphereFramesQ))
			WriteS(w, r, s, e)
		}
	})
	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))

	// Start FE server and block on it forever.
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
