package main

import (
	"golang.org/x/net/context"
	"google.golang.org/api/compute/v1"
)

// GetChunkServerInstances returns GCE instance tagged as chunk servers.
// It does not guarantee avaiability of them.
func (fe *FeServiceImpl) GetChunkServerInstances(ctx context.Context) ([]*compute.Instance, error) {
	service, err := fe.AuthCompute(ctx)
	if err != nil {
		return nil, err
	}

	list, err := service.Instances.List(ProjectId, zone).Do()
	if err != nil {
		return nil, err
	}

	var chunkServers []*compute.Instance
	for _, instance := range list.Items {
		if instance.Status == "TERMINATED" {
			continue
		}
		metadata := make(map[string]string)
		for _, item := range instance.Metadata.Items {
			metadata[item.Key] = *item.Value
		}
		ty, ok := metadata["bonsai-type"]
		if ok && ty == "chunk-"+fe.envType {
			chunkServers = append(chunkServers, instance)
		}
	}
	return chunkServers, nil
}
