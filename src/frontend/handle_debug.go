package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"time"
)

// HandleDebug translate as much errors into human-readable errors instead
// of logging, unlike other handles.
func (fe *FeServiceImpl) HandleDebug(q *api.DebugQ) (*api.DebugS, error) {
	ctx := context.Background()

	service, err := fe.authCompute(ctx)
	if err != nil {
		return &api.DebugS{
			ChunkServersError: fmt.Sprintf("Failed to get GCE access: %v", err),
		}, nil
	}

	list, err := service.Instances.List(ProjectId, zone).Do()
	if err != nil {
		return &api.DebugS{
			ChunkServersError: fmt.Sprintf("Failed to get GCE instance list: %#v", err),
		}, nil
	}

	var chunkServers []*api.DebugS_ChunkServerState
	for _, instance := range list.Items {
		metadata := make(map[string]string)
		for _, item := range instance.Metadata.Items {
			metadata[item.Key] = *item.Value
		}
		ty, ok := metadata["bonsai-type"]
		if ok && ty == "chunk" {
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
				resp, err := chunkService.Status(ctx, &api.StatusQ{})
				if err != nil {
					serverState.State = fmt.Sprintf("%v", err)
				} else {
					serverState.Health = api.DebugS_STATUS_OK
					serverState.State = fmt.Sprintf("%v", resp)
				}
			} else {
				serverState.State = fmt.Sprintf("%v", err)
			}
			chunkServers = append(chunkServers, &serverState)
		}

	}
	return &api.DebugS{
		ChunkServers: chunkServers,
	}, nil
}
