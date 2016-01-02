package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) Snapshot(ctx context.Context, q *api.SnapshotQ) (*api.SnapshotS, error) {
	s := <-ck.ChunkRouter.RequestSnapshot(q.ChunkId)
	return s, nil
}
