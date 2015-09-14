package main

import (
	"./api"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"math"
	"mime"
	"net/http"
	"time"
)

type FrontendServer struct {
	text string
}

func getStaticFileUrl(path string) string {
	return "/static/" + path
}

func (s FrontendServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fmt.Fprint(w, s.text)
}

func testCompareSpeed() {
	const n = 128
	const t = 500
	fmt.Printf("Testing lattice N=%d T<=%d\n", n, t)
	ref := NewGasLattice(n, 0.01)
	hash := NewHashGasLattice(ref)

	t0 := time.Now()
	for i := 0; i < t; i++ {
		ref.Step()
	}
	fmt.Printf("Naive: %v\n", time.Now().Sub(t0))

	t0 = time.Now()
	for hash.Timestep < t {
		hash.StepN()
		fmt.Printf("*T=%d\n", hash.Timestep)
	}
	fmt.Printf("Hash: %v\n", time.Now().Sub(t0))
}

func testTemperatureProperty() {
	const n = 128
	const t = 500
	fmt.Printf("= Testing lattice N=%d T<=%d\n", n, t)

	// 1..10^-6
	for i := 0; i < 30; i++ {
		temp := math.Pow(10, -float64(i)*0.2)
		ref := NewGasLattice(n, temp)
		hash := NewHashGasLattice(ref)

		t0 := time.Now()
		for hash.Timestep < t {
			hash.StepN()
		}
		fmt.Printf("== temp=%f dt=%v\n", temp, time.Now().Sub(t0))
		hash.PrintStat()
	}
}

// TODO: migrate to proto as soon as hashlife is proven to work.
type TestRequest struct {
	Timestep int
}

// TODO: migrate to proto as soon as hashlife is proven to work.
type TestResponse struct {
	N     int
	State map[string]int
}

func SerializeLattice(lattice Lattice) TestResponse {
	n := lattice.GetN()
	m := make(map[string]int)
	for ix := 0; ix < n; ix++ {
		for iy := 0; iy < n; iy++ {
			m[fmt.Sprintf("%d:%d", ix, iy)] = int(lattice.At(ix, iy))
		}
	}
	return TestResponse{
		N:     n,
		State: m,
	}
}

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
	go testTemperatureProperty()

	fmt.Print(api.WorldsQ{})

	mime.AddExtensionType(".svg", "image/svg+xml")

	fmt.Println("Starting frontend server http://localhost:8000")
	/*server := FrontendServer{
		text: "Bonsai frontend server 2",
	} */

	n := 128
	hash := NewHashGasLattice(NewGasLattice(n, 0.005)) // NewGasLattice(n, 0.005)

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

	http.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		resp := SerializeLattice(hash)
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(b)

		/*
			for i := 0; i < 10; i++ {
				hash.Step()
			}
		*/
		hash.StepN()
		fmt.Printf("T=%d\n", hash.Timestep)
	})

	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))
	// http.Handle("/", server)
	http.ListenAndServe(":8000", nil)
}
