package main

import (
	"./api"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
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

	bsTopo, err := fe.getBiosphereTopo(ctx, q.BiosphereId)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		log.Print("Active chunk server not found, returning dummy frame.")
		return &api.BiosphereFramesS{
			Content: fallbackContent(),
		}, nil
	}

	chunkInstance := chunks[0]
	ip := chunkInstance.NetworkInterfaces[0].NetworkIP

	// TODO: Reuse connection
	conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
		grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	chunkService := api.NewChunkServiceClient(conn)
	chunkIds := make([]string, len(bsTopo.GetChunkTopos()))
	for ix, chunkTopo := range bsTopo.GetChunkTopos() {
		chunkIds[ix] = chunkTopo.ChunkId
	}
	resp, err := chunkService.Snapshot(ctx, &api.SnapshotQ{
		ChunkId: chunkIds,
	})
	if err != nil {
		log.Printf("ChunkService.Snapshot failed %v", err)
		return nil, err
	}
	if resp.Snapshot == nil {
		return nil, errors.New("ChunkServer.Snapshot doesn't contain snapshot")
	}
	return &api.BiosphereFramesS{
		Content:          snapshotToMesh(bsTopo, resp.Snapshot).Serialize(),
		ContentTimestamp: resp.Timestamp,
	}, nil
}

func (fe *FeServiceImpl) getBiosphereTopo(ctx context.Context, biosphereId uint64) (BiosphereTopology, error) {
	client, err := fe.authDatastore(ctx)
	if err != nil {
		return nil, err
	}
	key := datastore.NewKey(ctx, "BiosphereMeta", "", int64(biosphereId), nil)
	var meta BiosphereMeta
	err = client.Get(ctx, key, &meta)
	if err != nil {
		return nil, err
	}
	log.Printf("Found config of %d: %d x %d", key.ID(), meta.Nx, meta.Ny)
	return NewCylinderTopology(biosphereId, int(meta.Nx), int(meta.Ny)), nil
}

func snapshotToMesh(bsTopo BiosphereTopology, snapshot map[string]*api.ChunkSnapshot) Mesh {
	offsets := bsTopo.GetGlobalOffsets()
	var mesh Mesh
	for chunkId, chunkSnapshot := range snapshot {
		offset := offsets[chunkId]
		for _, grain := range chunkSnapshot.Grains {
			pos := Vec3f{grain.Pos.X, grain.Pos.Y, grain.Pos.Z}.Add(offset)
			grainMesh := Icosahedron(pos, 0.06)
			baseColor := Vec3f{0, 0, 0}
			if grain.Kind == api.Grain_WATER {
				baseColor = Vec3f{0.4, 0.4, 1}
			} else if grain.Kind == api.Grain_SOIL {
				baseColor = Vec3f{0.8, 0.4, 0.3}
			}
			grainMesh.SetColor(baseColor.Add(Vec3f{rand.Float32(), rand.Float32(), rand.Float32()}.MultS(0.2)))
			mesh = append(mesh, grainMesh...)
		}
	}
	return mesh
}

func fallbackContent() *api.PolySoup {
	return Icosahedron(NewVec3f0(), 0.06).Serialize()
}
