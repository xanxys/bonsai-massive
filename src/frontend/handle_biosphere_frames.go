package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

func (fe *FeServiceImpl) BiosphereFrames(ctx context.Context, q *api.BiosphereFramesQ) (*api.BiosphereFramesS, error) {
	chunks, err := fe.GetChunkServerInstances(ctx)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		log.Print("Failed to connect to a chunk server; returning dummy frame")
		polysoup := Icosahedron(NewVec3f0(), 0.1).Serialize()
		return &api.BiosphereFramesS{
			Content: polysoup,
		}, nil
	} else {
		chunkInstance := chunks[0]
		ip := chunkInstance.NetworkInterfaces[0].NetworkIP

		conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
			grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		chunkService := api.NewChunkServiceClient(conn)
		log.Printf("Connected to chunk server %v, but returning dummy because chunk is not implemented yet", chunkService)
		polysoup := Icosahedron(NewVec3f0(), 0.1).Serialize()
		return &api.BiosphereFramesS{
			Content: polysoup,
		}, nil
	}
}
