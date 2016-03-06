package main

import (
	"./api"
	"errors"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
)

func (fe *FeServiceImpl) AddBiosphere(ctx context.Context, q *api.AddBiosphereQ) (*api.AddBiosphereS, error) {
	ctx = TraceStart(ctx, "/frontend.AddBiosphere")
	defer TraceEnd(ctx, fe.ServerCred)

	if q.Auth == nil {
		return nil, errors.New("AddBiosphere requires auth")
	}
	canWrite, err := fe.isWriteAuthorized(ctx, q.Auth)
	if err != nil {
		return nil, err
	}
	if !canWrite {
		return nil, errors.New("UI must disallow unauthorized actions")
	}

	client, err := fe.AuthDatastore(ctx)
	if err != nil {
		return nil, err
	}

	valid, err := fe.isValidNewConfig(ctx, client, q.Config)
	if err != nil {
		return nil, err
	}
	if !valid || q.TestOnly {
		return &api.AddBiosphereS{Success: valid}, nil
	}

	envBlob, err := proto.Marshal(q.Config.Env)
	if err != nil {
		return nil, err
	}
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	meta := &BiosphereMeta{
		Name: q.Config.Name,
		Nx:   q.Config.Nx,
		Ny:   q.Config.Ny,
		Env:  envBlob,
	}
	key, err = client.Put(ctx, key, meta)
	if err != nil {
		return nil, err
	}

	if q.Config.Env.StorageFileId != "" {
		bsTopo := NewCylinderTopology(uint64(key.ID()), int(meta.Nx), int(meta.Ny))
		err = fe.expandStorageToSnapshot(ctx, bsTopo, q.Config.Env.StorageFileId)
		if err != nil {
			log.Printf("ERROR: Failed to initialize with snapshot %v", err)
			log.Printf("Deleting biosphere entry %v", key)
			dsErr := client.Delete(ctx, key)
			if dsErr != nil {
				log.Printf("ERROR: Failed to delete BiosphereMeta(%v); datastore might have become inconsistent", key)
			}
			return nil, err
		}
	}

	return &api.AddBiosphereS{
		Success: true,
		BiosphereDesc: &api.BiosphereDesc{
			BiosphereId: uint64(key.ID()),
			Name:        meta.Name,
			NumCores:    uint32(meta.Nx*meta.Ny/5) + 1,
		},
	}, nil
}

// Reads storage file (binary proto of ChunkSnapshot) and copies it as snapshots
// of multiple chunks.
func (fe *FeServiceImpl) expandStorageToSnapshot(ctx context.Context, bsTopo BiosphereTopology, objectName string) error {
	ctx = TraceStart(ctx, "/frontend._.getStorage")
	defer TraceEnd(ctx, fe.ServerCred)

	service, err := fe.AuthStorage(ctx)

	res, err := service.Objects.Get(InitialEnvBucket, objectName).Download()
	if err != nil {
		log.Printf("WARNING: Failed to retrieve cloud storage object containing initial env, %v", err)
		return err
	}
	defer res.Body.Close()
	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("ERROR: Failed to download storage %v", err)
		return err
	}
	snapshot := &api.ChunkSnapshot{}
	err = proto.Unmarshal(blob, snapshot)
	if err != nil {
		log.Printf("ERROR: Failed to unmarshal snapshot proto %v", err)
		return err
	}

	// Bin grains.
	type ChunkKey struct {
		ix, iy int
	}
	bins := make(map[ChunkKey][]*api.Grain)
	for _, grain := range snapshot.Grains {
		key := ChunkKey{int(grain.Pos.X), int(grain.Pos.Y)}
		localGrain := proto.Clone(grain).(*api.Grain)
		localGrain.Pos.X -= float32(key.ix)
		localGrain.Pos.Y -= float32(key.iy)
		bins[key] = append(bins[key], localGrain)
	}

	dsClient, err := fe.AuthDatastore(ctx)
	if err != nil {
		return err
	}

	// Convert to chunk snapshots and write them.
	offsets := bsTopo.GetGlobalOffsets()
	for _, chunk := range bsTopo.GetChunkTopos() {
		offset := offsets[chunk.ChunkId]
		key := ChunkKey{int(offset.X), int(offset.Y)}
		grains, ok := bins[key]
		if !ok {
			continue
		}

		chunkSnapshot := &api.ChunkSnapshot{
			Grains: grains,
		}
		chunkBlob, err := proto.Marshal(chunkSnapshot)
		if err != nil {
			log.Printf("ERROR: Failed to marshal chunk snapshot proto %v", err)
			return err
		}

		// TODO: use transaction.
		dsKey := datastore.NewIncompleteKey(ctx, "PersistentChunkSnapshot", nil)
		_, err = func(ctx context.Context) (*datastore.Key, error) {
			ctx = TraceStart(ctx, "/google/datastore.Put")
			defer TraceEnd(ctx, fe.ServerCred)
			return dsClient.Put(ctx, dsKey, &PersistentChunkSnapshot{
				ChunkId:   chunk.ChunkId,
				Timestamp: 0,
				Snapshot:  chunkBlob,
			})
		}(ctx)
		if err != nil {
			log.Printf("ERROR: Failed to write chunk snapshot %v; snapshot became inconsistent", err)
			return err
		}
	}
	return nil
}

func (fe *FeServiceImpl) isValidNewConfig(ctx context.Context, dsClient *datastore.Client, config *api.BiosphereCreationConfig) (bool, error) {
	if config == nil {
		return false, nil
	}
	if config.Name == "" || config.Nx <= 0 || config.Ny <= 0 {
		return false, nil
	}
	// Name must be unique.
	qSameName := datastore.NewQuery("BiosphereMeta").Filter("Name =", config.Name)
	numSameName, err := dsClient.Count(ctx, qSameName)
	if err != nil {
		return false, err
	}
	if numSameName > 0 {
		return false, nil
	}
	return true, nil
}
