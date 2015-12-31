package main

import (
	"./api"
	"golang.org/x/net/context"
	"log"
)

func (fe *FeServiceImpl) ChangeExec(ctx context.Context, q *api.ChangeExecQ) (*api.ChangeExecS, error) {
	if q.TargetState == api.ChangeExecQ_STOPPED {
		fe.cmdQueue <- nil
	} else if q.TargetState == api.ChangeExecQ_RUNNING {
		log.Printf("RUN not implemented yet")
	}
	return &api.ChangeExecS{}, nil
}
