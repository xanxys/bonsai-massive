package main

import (
	"./api"
	"golang.org/x/net/context"
	"time"
)

func (ck *CkServiceImpl) Snapshot(ctx context.Context, q *api.SnapshotQ) (*api.SnapshotS, error) {
	s := <-ck.ChunkRouter.RequestSnapshot(q.ChunkId, time.Second*5)
	return s, nil
}
