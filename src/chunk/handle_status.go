package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) Status(ctx context.Context, q *api.StatusQ) (*api.StatusS, error) {
	ck.chunkQuery <- true
	result := <-ck.chunkResult
	return &api.StatusS{
		Snapshot:          result.Snapshot,
		SnapshotTimestamp: result.Timestamp,
	}, nil
}
