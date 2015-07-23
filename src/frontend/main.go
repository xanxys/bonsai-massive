package main

import (
	"fmt"
	"net/http"
)

type String string

func (s String) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, s)
}

func main() {
	fmt.Println("Hello World")
	http.Handle("/", String("Bonsai frontend server"))
	http.ListenAndServe("localhost:8000", nil)
}
