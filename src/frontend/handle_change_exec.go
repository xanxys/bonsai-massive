package main

import (
	"./api"
	"errors"
	"golang.org/x/net/context"
)

func (fe *FeServiceImpl) ChangeExec(ctx context.Context, q *api.ChangeExecQ) (*api.ChangeExecS, error) {
	if q.TargetState == api.ChangeExecQ_STOPPED {
		fe.cmdQueue <- nil
	} else if q.TargetState == api.ChangeExecQ_RUNNING {
		canWrite, err := fe.isWriteAuthorized(q.Auth)
		if err != nil {
			return nil, err
		}
		if !canWrite {
			return nil, errors.New("UI must disallow unauthorized START action")
		}
		bsTopo, err := fe.getBiosphereTopo(ctx, q.BiosphereId)
		if err != nil {
			return nil, err
		}
		fe.cmdQueue <- &ControllerCommand{bsTopo}
	}
	return &api.ChangeExecS{}, nil
}
