package main

import ()

type CkServiceImpl struct {
	*ChunkRouter
}

func NewCkService() *CkServiceImpl {
	ck := &CkServiceImpl{
		ChunkRouter: StartNewRouter(),
	}
	return ck
}
