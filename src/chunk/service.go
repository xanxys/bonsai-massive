package main

import ()

type CkServiceImpl struct {
	*ChunkRouter
	*ServerCred
}

func NewCkService() *CkServiceImpl {
	ck := &CkServiceImpl{
		ChunkRouter: StartNewRouter(),
		ServerCred:  NewServerCred(),
	}
	return ck
}
