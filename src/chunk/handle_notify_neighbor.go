package main

import (
	"./api"
	"golang.org/x/net/context"
)

func (ck *CkServiceImpl) NotifyNeighbor(ctx context.Context, q *api.NotifyNeighborQ) (*api.NotifyNeighborS, error) {
	ck.ChunkRouter.AcceptMulticastForExternalNodes(q.Packet)
	return &api.NotifyNeighborS{}, nil
}
