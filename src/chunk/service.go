package main

type CkServiceImpl struct {
	*ServerCred
	*SnapshotCache
	*ChunkConnections
}

func NewCkService() *CkServiceImpl {
	ck := &CkServiceImpl{
		ServerCred:       NewServerCred(),
		SnapshotCache:    NewSnapshotCache(),
		ChunkConnections: NewChunkConnections(),
	}
	return ck
}
