package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

// Debug translate as much errors into human-readable errors instead
// of logging, unlike other handles.
func (fe *FeServiceImpl) Debug(ctx context.Context, q *api.DebugQ) (*api.DebugS, error) {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		clusterInfo = fmt.Sprintf("error: %#V", err)
	}
	return &api.DebugS{
		ClusterInfo:     clusterInfo,
		ControllerDebug: fe.controller.GetDebug(),
		PoolDebug:       fe.controller.pool.GetDebug(),
	}, nil
}

// http://qiita.com/dtan4/items/f2f30207e0acec454c3d
func GetClusterInfo() (string, error) {
	kubeClient, err := client.NewInCluster()
	if err != nil {
		return "", err
	}

	pods, err := kubeClient.Pods(kapi.NamespaceDefault).List(kapi.ListOptions{})
	if err != nil {
		return "", err
	}

	//kubeClient.
	return fmt.Sprintf("%#v", pods), nil
}
