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

	sPacker := NewGrainPacker(api.Grain_SOIL)
	sPacker.latticeSize = 0.07
	sPacker.org = Vec3f{0.1, 0.5, 0}
	sPacker.nx = 10
	sPacker.ny = 10
	sPacker.nz = 40
	sPacker.packType = CI
	grains = append(grains, sPacker.Generate()...)

	wPacker := NewGrainPacker(api.Grain_WATER)
	wPacker.org = Vec3f{1.7 + rand.Float32()*0.5, rand.Float32() + 1.2, 0.1}
	wPacker.nx = 7
	wPacker.ny = 7
	wPacker.nz = 20
	wPacker.natural = true
	grains = append(grains, wPacker.Generate()...)

	for i := 0; i < 10; i++ {
		pos := Vec3f{rand.Float32(), rand.Float32(), 2}
		grains = append(grains, &api.Grain{
			Id:   uint64(rand.Uint32()),
			Pos:  &api.CkPosition{pos.X, pos.Y, pos.Z},
			Vel:  &api.CkVelocity{0, 0, 0},
			Kind: api.Grain_CELL,
			CellProp: &api.CellProp{
				Quals: make(map[string]int32),
				Cycle: &api.CellProp_Cycle{
					IsDividing: false,
				},
			},
		})
	}

	return &api.ChunkSnapshot{
		Grains: grains,
	}
}

// Pearson symbols to describe crystal packing structures.
type PackingType int

const (
	// primitive cubic
	CP = iota

	// body centered
	CI
)

type GrainPacker struct {
	grainType api.Grain_Kind

	// finite latice description
	latticeSize float32
	packType    PackingType
	org         Vec3f
	nx, ny, nz  int

	// If true, add small random noise to break grain symmetry.
	natural bool
}

func NewGrainPacker(grainType api.Grain_Kind) *GrainPacker {
	return &GrainPacker{
		grainType:   grainType,
		latticeSize: 0.1,
		packType:    CP,
	}
}

func (packer *GrainPacker) Generate() []*api.Grain {
	const groundOffset = 0.1
	const noiseAmplitude = 0.001

	offset := packer.org.Add(Vec3f{0, 0, groundOffset})

	var grains []*api.Grain
	for iz := 0; iz < packer.nz; iz++ {
		for ix := 0; ix < packer.nx; ix++ {
			for iy := 0; iy < packer.ny; iy++ {
				for _, unitOffset := range packer.generateUnitOffsets() {
					baseIndex := Vec3f{float32(ix), float32(iy), float32(iz)}
					pos := baseIndex.Add(unitOffset).MultS(packer.latticeSize).Add(offset)
					if packer.natural {
						noiseBase := Vec3f{rand.Float32(), rand.Float32(), rand.Float32()}.MultS(2).SubS(1)
						pos = pos.Add(noiseBase.MultS(noiseAmplitude))
					}
					grains = append(grains, &api.Grain{
						Id:   uint64(ix + 1),
						Pos:  &api.CkPosition{pos.X, pos.Y, pos.Z},
						Vel:  &api.CkVelocity{0, 0, 0},
						Kind: packer.grainType,
					})
				}

			}
		}
	}
	return grains
}

func (packer *GrainPacker) generateUnitOffsets() []Vec3f {
	switch packer.packType {
	case CP:
		return []Vec3f{Vec3f{0, 0, 0}}
	case CI:
		return []Vec3f{Vec3f{0, 0, 0}, Vec3f{0.5, 0.5, 0.5}}
	}
	return nil
}
