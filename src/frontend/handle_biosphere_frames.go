package main

import (
	"./api"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"time"
)

func (fe *FeServiceImpl) BiosphereFrames(ctx context.Context, q *api.BiosphereFramesQ) (*api.BiosphereFramesS, error) {
	chunks, err := fe.GetChunkServerInstances(ctx)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		log.Print("Active chunk server not found.")
		if q.EnsureStart {
			log.Print("Trying to start new chunk server and returning dummy frame for now")
			clientCompute, err := fe.authCompute(ctx)
			if err != nil {
				return nil, err
			}
			fe.prepare(clientCompute)
			return &api.BiosphereFramesS{
				Content: fallbackContent(),
			}, nil
		} else {
			log.Print("Returning dummy frame")
			return &api.BiosphereFramesS{
				Content: fallbackContent(),
			}, nil
		}
	}

	chunkInstance := chunks[0]
	ip := chunkInstance.NetworkInterfaces[0].NetworkIP

	conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
		grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	chunkService := api.NewChunkServiceClient(conn)
	resp, err := chunkService.Status(ctx, &api.StatusQ{})
	if err != nil {
		log.Printf("ChunkService.Status filed %v", err)
		return nil, err
	}
	if resp.Snapshot == nil {
		return nil, errors.New("ChunkServer.Status doesn't contain snapshot")
	}

	var mesh Mesh
	for _, grain := range resp.Snapshot.Grains {
		pos := Vec3f{float32(grain.Pos.X), float32(grain.Pos.Y), float32(grain.Pos.Z)}.MultS(1e-4)
		grainMesh := Icosahedron(pos, 0.1)
		if grain.Kind == api.Grain_WATER {
			grainMesh.SetColor(Vec3f{rand.Float32()*0.1 + 0.5, rand.Float32()*0.1 + 0.5, 1})
		} else if grain.Kind == api.Grain_SOIL {
			grainMesh.SetColor(Vec3f{1, rand.Float32()*0.1 + 0.5, rand.Float32()*0.1 + 0.5})
		} else {
			grainMesh.SetColor(Vec3f{rand.Float32(), rand.Float32(), rand.Float32()})
		}
		mesh = append(mesh, grainMesh...)

	}
	return &api.BiosphereFramesS{
		Content:          mesh.Serialize(),
		ContentTimestamp: resp.SnapshotTimestamp,
	}, nil

}

func fallbackContent() *api.PolySoup {
	return Icosahedron(NewVec3f0(), 0.1).Serialize()
}
