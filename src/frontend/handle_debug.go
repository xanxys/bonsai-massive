package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"time"
)

// Debug translate as much errors into human-readable errors instead
// of logging, unlike other handles.
func (fe *FeServiceImpl) Debug(ctx context.Context, q *api.DebugQ) (*api.DebugS, error) {
	chunkInstances, err := fe.GetChunkServerInstances(ctx)
	if err != nil {
		return &api.DebugS{
			ChunkServersError: fmt.Sprintf("Failed to list chunk servers: %v", err),
		}, nil
	}

	var chunkServers []*api.DebugS_ChunkServerState
	for _, instance := range chunkInstances {
		ip := instance.NetworkInterfaces[0].NetworkIP
		serverState := api.DebugS_ChunkServerState{
			IpAddress: ip,
			Health:    api.DebugS_ALLOCATED,
		}
		conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
			grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
		if conn != nil {
			defer conn.Close()
			serverState.Health = api.DebugS_GRPC_OK
			chunkService := api.NewChunkServiceClient(conn)
			rpcStart := time.Now()
			resp, err := chunkService.Status(ctx, &api.StatusQ{})
			rpcRtt := time.Since(rpcStart)
			if err != nil {
				serverState.State = fmt.Sprintf("%v", err)
			} else {
				serverState.Health = api.DebugS_STATUS_OK
				serverState.Rtt = float32(rpcRtt) * 1e-9
				serverState.State = fmt.Sprintf("%v", resp)
			}
		} else {
			serverState.State = fmt.Sprintf("%v", err)
		}
		chunkServers = append(chunkServers, &serverState)
	}
	return &api.DebugS{
		ChunkServers: chunkServers,
	}, nil
}
