package main

type CkServiceImpl struct {
	*ServerCred
	*SnapshotCache
}

func NewCkService() *CkServiceImpl {
	ck := &CkServiceImpl{
		ServerCred:    NewServerCred(),
		SnapshotCache: NewSnapshotCache(),
	}
	return ck
}
