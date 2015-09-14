package main

import (
	"./api"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"mime"
	"net/http"
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
		fmt.Fprintf(w, "pb param required")
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
	w http.ResponseWriter, r *http.Request, s proto.Message) {

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

	fe := &FeServiceImpl{}

	// Dispatchers.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "World List\n")
	})
	http.HandleFunc("/api/worlds", func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, &api.WorldsQ{})
		if q != nil {
			s := fe.HandleWorlds((*q).(*api.WorldsQ))
			WriteS(w, r, s)
		}
	})
	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))

	// Start FE server and block on it forever.
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
