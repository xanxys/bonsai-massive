package main

import (
	"./api"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

func main() {
	logPath, ok := os.LookupEnv("LOG_PATH")
	if !ok {
		log.Println("$LOG_PATH not found. Writing to stderr")
	} else {
		f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Cannot open log file, %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.Printf("Writing to $LOG_PATH=%s\n", logPath)
	}

	log.Println("Starting chunk server at :9000 (gRPC)")
	lis, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	ck := NewCkService()
	api.RegisterChunkServiceServer(server, ck)
	server.Serve(lis)
}
