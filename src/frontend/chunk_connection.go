package main

import (
	"fmt"
	"google.golang.org/grpc"
	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"log"
	"sync"
	"time"
)

// Reference counted connection chunk server grpc connection cache.
// All methods are thread-safe.
type ChunkConnections struct {
	connCacheLock sync.Mutex
	connCache     map[string]*GrpcConnUsage // ip -> usage
}

func NewChunkConnections() *ChunkConnections {
	return &ChunkConnections{
		connCache: make(map[string]*GrpcConnUsage),
	}
}

type GrpcConnUsage struct {
	conn   *grpc.ClientConn
	numRef int
}

// Get fresh list of chunk pod IP addresses.
// http://qiita.com/dtan4/items/f2f30207e0acec454c3d
func GetChunkIps() []string {
	kubeClient, err := client.NewInCluster()
	if err != nil {
		return nil
	}

	pods, err := kubeClient.Pods(kapi.NamespaceDefault).List(kapi.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"name": "chunk",
			"env":  "staging",
		}),
	})
	if err != nil {
		return nil
	}

	podIps := make([]string, len(pods.Items))
	for ix, pod := range pods.Items {
		podIps[ix] = pod.Status.PodIP
	}
	return podIps
}

// Returns {ip: connection}.
func (cc *ChunkConnections) AcquireUsableChunkConn() map[string]*grpc.ClientConn {
	cc.connCacheLock.Lock()
	defer cc.connCacheLock.Unlock()

	ips := GetChunkIps()
	conns := make(map[string]*grpc.ClientConn)
	for _, ip := range ips {
		connUsage := cc.connCache[ip]
		if connUsage == nil {
			conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
				grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
			if err != nil {
				log.Printf("WARNING Couldn't connect to chunk server %s, ignoring. %v", ip, err)
				continue
			}
			connUsage = &GrpcConnUsage{conn: conn, numRef: 0}
			cc.connCache[ip] = connUsage
		}
		connUsage.numRef++
		conns[ip] = connUsage.conn
	}
	return conns
}

// Release ips. Internally, connection cache is referecen-counted and closed when
// all acquired connections are released.
func (cc *ChunkConnections) ReleaseChunkConn(ips []string) {
	cc.connCacheLock.Lock()
	defer cc.connCacheLock.Unlock()

	for _, ip := range ips {
		connUsage := cc.connCache[ip]
		if connUsage == nil || connUsage.numRef == 0 {
			log.Panicf("ERROR releaseChunkConn called for non-existing (or already closed conn) %s", ip)
		}
		connUsage.numRef--
		if connUsage.numRef == 0 {
			log.Printf("INFO Closing chunk server connection %s", ip)
			connUsage.conn.Close()
			delete(cc.connCache, ip)
		}
	}
}

// Get short-term connection without increasing refcount.
// Caller must make sure (last) releaseChunkConn is not yet called while the returned
// connection for the ip is in use.
// Returns nil if not possible.
func (cc *ChunkConnections) GetChunkConnWeak(ip string) *grpc.ClientConn {
	cc.connCacheLock.Lock()
	defer cc.connCacheLock.Unlock()
	connUsage := cc.connCache[ip]
	if connUsage == nil {
		return nil
	}
	return connUsage.conn
}
