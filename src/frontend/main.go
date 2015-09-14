package main

import (
	"./api"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"mime"
	"net/http"
)

type FeServiceImpl struct {
}

func (fe *FeServiceImpl) HandleWorlds(q *api.WorldsQ) *api.WorldsS {
	name := "Hogehoge"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38
	return &api.WorldsS{
		Worlds: []*api.WorldDescription{
			&api.WorldDescription{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}

func main() {
	const port = 8000
	fmt.Printf("Starting frontend server http://localhost:%d\n", port)
	mime.AddExtensionType(".svg", "image/svg+xml")

	http.HandleFunc("/prototype", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prototype" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "/root/bonsai/static/index.html")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "World List\n")
	})

	fe := &FeServiceImpl{}

	http.HandleFunc("/api/worlds", func(w http.ResponseWriter, r *http.Request) {
		q := api.WorldsQ{}
		err := r.ParseForm()
		if err != nil {
			http.NotFound(w, r)
			fmt.Fprintf(w, "strange query %v", err)
			return
		}
		pb := r.Form.Get("pb")
		if pb == "" {
			http.NotFound(w, r)
			fmt.Fprintf(w, "pb param required")
			return
		}
		err = jsonpb.UnmarshalString(pb, &q)
		if err != nil {
			fmt.Fprintf(w, "Failed to parse pb param %v", err)
			return
		}
		s := fe.HandleWorlds(&q)

		w.Header().Set("Content-Type", "application/json")
		marshaler := jsonpb.Marshaler{
			EnumsAsString: true,
			Indent:        "",
		}
		err = marshaler.Marshal(w, s)
		if err != nil {
			fmt.Fprintf(w, "{\"error\": \"Internal error: failed to encode return pb\"}")
		}
	})

	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
