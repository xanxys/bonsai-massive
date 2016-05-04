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

	rand.Seed(time.Now().UnixNano())
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
	//grains = append(grains, GeneratePackedSoilCP(Vec3f{0.5, 0.5, 0}, 5, 10, 10)...)
	//grains = append(grains, GeneratePackedSoilCP(Vec3f{0.5 + rand.Float32(), 0.5 + rand.Float32(), 0.6}, 10, 10, 5)...)
	grains = append(grains, GeneratePackedSoilCP(Vec3f{0.1, 0.5, 0}, 10, 10, 100, 0.1)...)
	// grains = append(grains, GeneratePackedSoilCP(Vec3f{1.1, 0.5, 0}, 7, 7, 10, 0.21)...)
	//grains = append(grains, GeneratePackedSoilCP(Vec3f{2.1, 0.5, 0}, 7, 7, 10, 0.3)...)
	grains = append(grains, GeneratePackedWaterCP(Vec3f{1.7 + rand.Float32()*0.5, rand.Float32() + 1.2, 0.1}, 7, 7, 20, true)...)

	return &api.ChunkSnapshot{
		Grains: grains,
	}
}

// Genrate soil with cP (primitive cubic) lattice.
func GeneratePackedSoilCP(org Vec3f, nx, ny, nz int, latticeSize float32) []*api.Grain {
	const groundOffset = 0.1

	var grains []*api.Grain
	for iz := 0; iz < nz; iz++ {
		for ix := 0; ix < nx; ix++ {
			for iy := 0; iy < ny; iy++ {
				grains = append(grains, &api.Grain{
					Id: uint64(ix + 1),
					Pos: &api.CkPosition{
						X: float32(ix)*latticeSize + org.X,
						Y: float32(iy)*latticeSize + org.Y,
						Z: float32(iz)*latticeSize + groundOffset + org.Z,
					},
					Vel:  &api.CkVelocity{0, 0, 0},
					Kind: api.Grain_SOIL,
				})
			}
		}
	}
	return grains
}

// Genrate water with cP (primitive cubic) lattice.
// natural==true: Add small random noise to make grains behave like fluid
// natural==false: Put grains at perfect lattice, making them non-fluid (when rest on simple ground)
func GeneratePackedWaterCP(org Vec3f, nx, ny, nz int, natural bool) []*api.Grain {
	const latticeSize = 0.1
	const groundOffset = 0.1
	const noiseAmplitude = 0.001

	var grains []*api.Grain
	for iz := 0; iz < nz; iz++ {
		for ix := 0; ix < nx; ix++ {
			for iy := 0; iy < ny; iy++ {
				pos := &api.CkPosition{
					X: float32(ix)*latticeSize + org.X,
					Y: float32(iy)*latticeSize + org.Y,
					Z: float32(iz)*latticeSize + groundOffset + org.Z,
				}
				if natural {
					pos.X += (rand.Float32()*2 - 1) * noiseAmplitude
					pos.Y += (rand.Float32()*2 - 1) * noiseAmplitude
					pos.Z += (rand.Float32()*2 - 1) * noiseAmplitude
				}
				grains = append(grains, &api.Grain{
					Id:   uint64(ix + 1),
					Pos:  pos,
					Vel:  &api.CkVelocity{0, 0, 0},
					Kind: api.Grain_WATER,
				})
			}
		}
	}
	return grains
}
