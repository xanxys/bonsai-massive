package main

import (
	"./api"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"log"
)

func (fe *FeServiceImpl) DeleteBiosphere(ctx context.Context, q *api.DeleteBiosphereQ) (*api.DeleteBiosphereS, error) {
	ctx = TraceStart(ctx, "/frontend.DeleteBiosphere")
	defer TraceEnd(ctx, fe.ServerCred)

	if q.Auth == nil {
		return nil, errors.New("DeleteBiosphere requires auth")
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

	chunkIdStart := fmt.Sprintf("%d", q.BiosphereId)
	chunkIdEnd := fmt.Sprintf("%d", q.BiosphereId+1)

	// Delete snapshots.
	snapshotQ := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId>=", chunkIdStart).Filter("ChunkId<", chunkIdEnd).KeysOnly()
	snapshotKeys, err := client.GetAll(ctx, snapshotQ, nil)
	log.Printf("Delete snapshots: [%s, %s): %d PersistentChunkSnapshot keys found", chunkIdStart, chunkIdEnd, len(snapshotKeys))
	if err != nil {
		return nil, err
	}
	err = client.DeleteMulti(ctx, snapshotKeys)
	if err != nil {
		return nil, err
	}
	// Delete metadata.
	key := datastore.NewKey(ctx, "BiosphereMeta", "", int64(q.BiosphereId), nil)
	err = client.Delete(ctx, key)
	if err != nil {
		return nil, err
	}

	return &api.DeleteBiosphereS{
		Deleted: true,
	}, nil
}
