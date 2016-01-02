package main

import (
	"./api"
)

type ChunkRouter interface {
	RequestNeighbor(timestamp uint64, topo *api.ChunkTopology) chan *NeighborImport
	NotifyResult(timestamp uint64, topo *api.ChunkTopology, export *NeighborExport)
}

// Immutable structure that holds incoming grains from neighbor / env grains.
type NeighborImport struct {
	// Incoming grains.
	IncomingGrains []*api.Grain

	// Frozen neighbor chunks. (in their coordinates)
	EnvGrains map[string][]*api.Grain
}

// Immutable structure that holds outgoing grains / env grains visible from other
// chunks.
type NeighborExport struct {
	// Can be a subset of a chunk, but it's safe to pass everything.
	ChunkGrains []*api.Grain

	// Frozen escaped grains after canonicalization.
	EscapedGrains map[string][]*api.Grain
}

// Request neighbors necessary for stepping chunk from timestap to timetamp+1,
// with given topo. Returns a channel that returns that data once.
func (ck *CkServiceImpl) RequestNeighbor(timestamp uint64, topo *api.ChunkTopology) chan *NeighborImport {
	ch := make(chan *NeighborImport)
	return ch
}

func (ck *CkServiceImpl) NotifyResult(timestamp uint64, topo *api.ChunkTopology, export *NeighborExport) {
}
