package main

import (
	"fmt"
	"net/http"
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
