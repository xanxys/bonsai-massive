package main

import (
	"./api"
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/storage/v1"
	"log"
	"math/rand"
	"time"
)

func main() {
	ctx := context.Background()

	nx := 3
	ny := 3

	service, err := GetStorageService(ctx)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
		return
	}

	snapshot := GenerateSnapshot(12345)
	blob, err := proto.Marshal(snapshot)
	if err != nil {
		log.Panicf("Failed to serialize snapshot %v", err)
		return
	}

	objectName := fmt.Sprintf("envgen-%dx%d:%s:%08x", nx, ny, time.Now().Format("2006-01-02"), rand.Uint32())
	object := &storage.Object{Name: objectName}
	log.Printf("Uploading to gs://%s/%s", InitialEnvBucket, objectName)

	_, err = service.Objects.Insert(InitialEnvBucket, object).Media(bytes.NewReader(blob)).Do()
	if err != nil {
		log.Panicf("Failed to upload %v", err)
		return
	}
	log.Printf("Successfully uploaded! %s", objectName)
}

func GetStorageService(ctx context.Context) (*storage.Service, error) {
	client, err := google.DefaultClient(ctx, storage.DevstorageFullControlScope)
	if err != nil {
		return nil, err
	}
	return storage.New(client)
}

func GenerateSnapshot(seed int64) *api.ChunkSnapshot {
	rand.Seed(seed)
	var grains []*api.Grain

	for ix := 0; ix < 10; ix++ {
		for iy := 0; iy < 10; iy++ {
			grains = append(grains, &api.Grain{
				Id: uint64(ix + 1),
				Pos: &api.CkPosition{
					X: float32(ix) * 0.1,
					Y: float32(iy) * 0.1,
					Z: 0.1,
				},
				Vel:  &api.CkVelocity{0, 0, 0},
				Kind: api.Grain_SOIL,
			})
		}
	}

	return &api.ChunkSnapshot{
		Grains: grains,
	}
}
