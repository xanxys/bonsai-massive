package main

import (
	"encoding/json"
	"fmt"
	"math"
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

func SerializeLattice(lattice *HashGasLattice) TestResponse {
	m := make(map[string]int)
	for ix := 0; ix < lattice.N; ix++ {
		for iy := 0; iy < lattice.N; iy++ {
			m[fmt.Sprintf("%d:%d", ix, iy)] = int(lattice.At(ix, iy))
		}
	}
	return TestResponse{
		N:     lattice.N,
		State: m,
	}
}

func main() {
	go testTemperatureProperty()

	fmt.Println("Starting frontend server http://localhost:8000")
	/*server := FrontendServer{
		text: "Bonsai frontend server 2",
	} */

	n := 64
	hash := NewHashGasLattice(NewGasLattice(n, 0.005))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "/root/bonsai/static/index.html")
	})
	http.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		resp := SerializeLattice(hash)
		b, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(b)

		hash.StepN()
		fmt.Printf("T=%d\n", hash.Timestep)
	})

	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))
	// http.Handle("/", server)
	http.ListenAndServe(":8000", nil)
}
