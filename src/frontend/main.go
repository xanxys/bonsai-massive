package main

import (
	"fmt"
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

func main() {
	const n = 128
	const t = 500
	fmt.Printf("Testing lattice N=%d T<=%d\n", n, t)
	ref := NewGasLattice(n)
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

	fmt.Println("Starting frontend server")
	/*server := FrontendServer{
		text: "Bonsai frontend server 2",
	} */
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "/root/bonsai/static/index.html")
	})
	http.Handle("/static/",
		http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))
	// http.Handle("/", server)
	http.ListenAndServe(":8000", nil)
}
