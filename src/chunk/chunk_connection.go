package main

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"sync"
	"time"
)

const EXPIRATION = 10 * time.Minute

// TTL-based chunk server connection.
// All methods are thread-safe.
type ChunkConnections struct {
	lock  sync.Mutex
	cache map[string]*GrpcConnUsage // ip -> usage
}

func NewChunkConnections() *ChunkConnections {
	cc := &ChunkConnections{
		cache: make(map[string]*GrpcConnUsage),
	}
	go func() {
		time.Sleep(10 * time.Second)
		cc.lock.Lock()
		for ip, connUsage := range cc.cache {
			if time.Now().Sub(connUsage.lastUsed) >= EXPIRATION {
				log.Printf("INFO connection to %s expired. closing", ip)
				connUsage.conn.Close()
				delete(cc.cache, ip)
			}
		}
		cc.lock.Unlock()
	}()
	return cc
}

type GrpcConnUsage struct {
	conn     *grpc.ClientConn
	lastUsed time.Time
}

// Returns connection. Renews expiration time.
func (cc *ChunkConnections) GetChunkConn(ip string) *grpc.ClientConn {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	connUsage := cc.cache[ip]
	if connUsage == nil {
		log.Printf("INFO opening connection to non-cached server %s", ip)
		svcIpPort := fmt.Sprintf("%s:9000", ip)
		conn, err := grpc.Dial(svcIpPort, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(500*time.Millisecond))
		if err != nil {
			log.Printf("ERROR Failed to connect to %s", svcIpPort)
			return nil
		}
		connUsage = &GrpcConnUsage{conn: conn}
		cc.cache[ip] = connUsage
	}
	connUsage.lastUsed = time.Now()
	return connUsage.conn
}
