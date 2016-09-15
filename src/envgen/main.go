package main

import (
	"./api"
	"bytes"
	"flag"
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
	var seed_ptr = flag.Int64("seed", 12345, "Seed number of random number generator.")
	var type_ptr = flag.String("type", "terrain", "Environment type. One of {test_move, terrain}.")
	flag.Parse()

	ctx := context.Background()

	nx := 3
	ny := 3

	service, err := GetStorageService(ctx)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
		return
	}

	var snapshot *api.ChunkSnapshot
	switch *type_ptr {
	case "test_move":
		snapshot = GenerateTestMove()
	case "terrain":
		snapshot = GenerateSnapshot(*seed_ptr)
	default:
		log.Fatalf("Unknown type: %s", *type_ptr)
	}
	blob, err := proto.Marshal(snapshot)
	if err != nil {
		log.Panicf("Failed to serialize snapshot %v", err)
		return
	}

	rand.Seed(time.Now().UnixNano())
	objectName := fmt.Sprintf("envgen-%dx%d:%s:%s:%08x", nx, ny,
		time.Now().Format("2006-01-02"), *type_ptr, rand.Uint32())
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
