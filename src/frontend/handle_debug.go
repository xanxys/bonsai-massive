package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"strings"
)

// Debug translate as much errors into human-readable errors instead
// of logging, unlike other handles.
func (fe *FeServiceImpl) Debug(ctx context.Context, q *api.DebugQ) (*api.DebugS, error) {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		clusterInfo = fmt.Sprintf("error: %#v", err)
	}
	return &api.DebugS{
		ClusterInfo:     clusterInfo,
		ControllerDebug: fe.controller.GetDebug(),
	}, nil
}

// http://qiita.com/dtan4/items/f2f30207e0acec454c3d
func GetClusterInfo() (string, error) {
	kubeClient, err := client.NewInCluster()
	if err != nil {
		return "", err
	}

	pods, err := kubeClient.Pods(kapi.NamespaceDefault).List(kapi.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"name": "chunk",
			"env":  "staging",
		}),
	})
	if err != nil {
		return "", err
	}

	podDescs := make([]string, len(pods.Items))
	for ix, pod := range pods.Items {
		podDescs[ix] = fmt.Sprintf("%s : %s : %s", pod.Status.PodIP, pod.Status.Phase, pod.Name)
	}
	return strings.Join(podDescs, "\n"), nil
}
