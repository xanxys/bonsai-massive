package main

import (
	"./api"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	log.Println("Starting chunk server at :8000 (gRPC)")
	lis, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	ck := NewCkService()
	api.RegisterChunkServiceServer(server, ck)
	server.Serve(lis)
}
