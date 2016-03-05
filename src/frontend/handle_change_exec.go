package main

import (
	"./api"
	"errors"
	"golang.org/x/net/context"
)

func (fe *FeServiceImpl) ChangeExec(ctx context.Context, q *api.ChangeExecQ) (*api.ChangeExecS, error) {
	ctx = TraceStart(ctx, "/frontend.ChangeExec")
	defer TraceEnd(ctx, fe.ServerCred)

	if q.TargetState == api.ChangeExecQ_STOPPED {
		fe.controller.SetBiosphereState(q.BiosphereId, nil)
	} else if q.TargetState == api.ChangeExecQ_RUNNING {
		canWrite, err := fe.isWriteAuthorized(ctx, q.Auth)
		if err != nil {
			return nil, err
		}
		if !canWrite {
			return nil, errors.New("UI must disallow unauthorized START action")
		}
		bsTopo, envConfig, err, _ := fe.getBiosphereTopo(ctx, q.BiosphereId)
		if err != nil {
			return nil, err
		}
		fe.controller.SetBiosphereState(q.BiosphereId, &TargetState{bsTopo, envConfig})
	}
	return &api.ChangeExecS{}, nil
}
