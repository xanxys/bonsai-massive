package main

import (
	"./api"
	"golang.org/x/net/context"
)

// Debug translate as much errors into human-readable errors instead
// of logging, unlike other handles.
func (fe *FeServiceImpl) Debug(ctx context.Context, q *api.DebugQ) (*api.DebugS, error) {
	return &api.DebugS{
		ControllerDebug: fe.controller.GetDebug(),
		PoolDebug:       fe.controller.pool.GetDebug(),
	}, nil
}
