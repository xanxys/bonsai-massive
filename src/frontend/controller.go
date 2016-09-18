package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"google.golang.org/grpc"
	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"log"
	"math"
	"sync"
	"time"
)

func NewController(fe *FeServiceImpl) *Controller {
	ctrl := &Controller{
		fe:          fe,
		targetState: make(map[uint64]TargetState),
		execution:   make(map[uint64]chan bool),
		latestBs:    make(map[uint64]*biosphereState),
		connCache:   make(map[string]*GrpcConnUsage),
	}
	return ctrl
}

// All methods are thread-safe, and are guranteed to return within 150ms.
// (one RPC w/ chunk servers or nothing).
type Controller struct {
	fe *FeServiceImpl
	// BiosphereId -> TargetState
	targetState map[uint64]TargetState
	// BiosphereId -> control channel
	execution map[uint64]chan bool

	latestBsLock sync.Mutex
	latestBs     map[uint64]*biosphereState

	connCacheLock sync.Mutex
	connCache     map[string]*GrpcConnUsage // ip -> usage
}

type GrpcConnUsage struct {
	conn   *grpc.ClientConn
	numRef int
}

/*
BiosphereState
   = Stopped
   // Waiting for resource allocation
   | Waiting
   // ChunkId -> IP address
   | Running map[string]string
*/
type BiosphereStateFlag int

const (
	Stopped BiosphereStateFlag = iota
	Waiting
	Running
)

type TargetState struct {
	BsTopo BiosphereTopology
	Env    *api.BiosphereEnvConfig
	Slow   bool
}

type BiosphereState struct {
	flag BiosphereStateFlag
	// ChunkId -> IP address (only available when flag == Running)
	chunks map[string]string
}

func (ctrl *Controller) GetDebug() *api.ControllerDebug {
	m := make(map[uint64]*api.ControllerDebug_BiosphereState)
	for bsId, state := range ctrl.GetCurrentState() {
		m[bsId] = &api.ControllerDebug_BiosphereState{
			Flag: api.ControllerDebug_BiosphereFlag(state.flag),
		}
	}
	return &api.ControllerDebug{
		Biospheres: m,
	}
}

func (ctrl *Controller) GetBiosphereState(biosphereId uint64) BiosphereState {
	state, ok := ctrl.GetCurrentState()[biosphereId]
	if ok {
		return state
	} else {
		return BiosphereState{flag: Stopped}
	}
}

// TargetState != nil: Make it Running with specified parameters.
// TargetState == nil: Stop it.
//
// retval: target state is already achieved.
func (ctrl *Controller) SetBiosphereState(biosphereId uint64, targetState *TargetState) bool {
	if targetState != nil {
		ctrl.targetState[biosphereId] = *targetState
		execCh := make(chan bool, 10)
		ctrl.execution[biosphereId] = execCh
		go ctrl.runBiosphere(biosphereId, targetState, execCh)
	} else {
		delete(ctrl.targetState, biosphereId)
		ctrl.execution[biosphereId] <- true
		delete(ctrl.execution, biosphereId)
	}
	return true
}

func (ctrl *Controller) runBiosphere(pubTargetId uint64, target *TargetState, execCh chan bool) {
	conns := ctrl.acquireUsableChunkConn()
	ips := make([]string, 0, len(conns))
	for ip, _ := range conns {
		ips = append(ips, ip)
	}
	chunkAlloc := assignChunks(target.BsTopo, ips)

	// Prepare initial chunk data locator.
	initTimestamp := uint64(0) // TODO: Use maxPersistedTimestamp instead
	chunks := ctrl.getInitialDataLocators(initTimestamp, target.BsTopo)
	if chunks == nil {
		log.Printf("ERROR biosphere(%d): failed to get initial data locators. Aborting.", pubTargetId)
		return
	}

	// Keep running forever until terminated.
	bsState := &biosphereState{
		bsTopo:    target.BsTopo,
		timestamp: initTimestamp,
		chunks:    chunks,
	}
	for {
		select {
		case <-execCh:
			fmt.Printf("INFO biosphere(%d): terminated signal received (T=%d)",
				pubTargetId, bsState.timestamp)
			ctrl.releaseChunkConn(ips)
			return
		default:
		}
		prevBsState := bsState
		tBegin := time.Now()
		bsState = stepBiosphere(conns, chunkAlloc, bsState)
		if bsState == nil {
			log.Printf("ERROR biosphere(%d): stepBiosphere failed (T:%d->%d). Aborting.",
				pubTargetId, prevBsState.timestamp, prevBsState.timestamp+1)
			return
		}
		log.Printf("INFO %.3f sec (T:%d->%d)", time.Since(tBegin).Seconds(), prevBsState.timestamp, bsState.timestamp)
		ctrl.publishLatest(pubTargetId, bsState)
		if target.Slow {
			time.Sleep(time.Second)
		}
	}
}

// Returns (snapshot, timestamp) if available, otherwise (nil, undefined).
// WARNING: In-transit grains (grains that escaped a chunk at a previous timestamp)
//  is included in the SOURCE (not destination) chunk, w/ out-of-bound coordinates.
// e.g. A grain exited chunk "cs-A" to "cs-B" X+ direction. The grain is returned
// as ("cs-A", (1.2, 0)), not ("cs-B", (0.2, 0)).
// This is because this method does not know about chunk topology.
func (ctrl *Controller) GetLatestSnapshot(bsId uint64) (map[string]*api.ChunkSnapshot, uint64) {
	ctrl.latestBsLock.Lock()
	bsState := ctrl.latestBs[bsId]
	ctrl.latestBsLock.Unlock()
	if bsState == nil {
		return nil, 0
	}

	var wg sync.WaitGroup
	var ssLock sync.Mutex
	snapshots := make(map[string]*api.ChunkSnapshot)
	for chunkId, dataLocator := range bsState.chunks {
		wg.Add(1)
		go func(chunkId string, remoteKey *api.RemoteChunkCache) {
			defer wg.Done()
			conn := ctrl.getChunkConn(remoteKey.Ip)
			if conn == nil {
				log.Printf("ERROR Chunk server %s is not connected", remoteKey.Ip)
				return
			}
			service := api.NewChunkServiceClient(conn)
			strictCtx, _ := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			s, err := service.GetChunk(strictCtx, &api.GetChunkQ{CacheKey: remoteKey.CacheKey})
			if err != nil || !s.Success {
				log.Printf("ERROR GetChunk(%#v) failed", remoteKey)
				return
			}

			// Flatten shards.
			var grains []*api.Grain
			for _, shard := range s.Content.Shards {
				for _, grain := range shard.Grains {
					grainMoved := *grain
					grainMoved.Pos.X += float32(shard.Dp.Dx)
					grainMoved.Pos.Y += float32(shard.Dp.Dy)
					grains = append(grains, &grainMoved)
				}
			}
			ssLock.Lock()
			defer ssLock.Unlock()
			snapshots[chunkId] = &api.ChunkSnapshot{Grains: grains}
		}(chunkId, dataLocator.GetRemoteCacheKey())
	}
	wg.Wait()
	return snapshots, bsState.timestamp
}

func (ctrl *Controller) publishLatest(bsId uint64, bsState *biosphereState) {
	ctrl.latestBsLock.Lock()
	defer ctrl.latestBsLock.Unlock()
	ctrl.latestBs[bsId] = bsState
}

type biosphereState struct {
	// Immutable biosphere topology.
	bsTopo BiosphereTopology

	timestamp uint64

	// chunk id -> chunk data.
	chunks map[string]*api.ChunkDataLocator
}

// Calculate stepped biosphere state using workers, in a blocking way.
// If failed, return nil.
func stepBiosphere(connCache map[string]*grpc.ClientConn, workers map[string]string, st *biosphereState) *biosphereState {
	var wg sync.WaitGroup
	newChunks := make(map[string]*api.ChunkDataLocator)
	for _, cTopo := range st.bsTopo.GetChunkTopos() {
		ip := workers[cTopo.ChunkId]
		// Convert neighbor topo into data locators, with self/remote optimization.
		inputs := make([]*api.StepChunkQ_Input, len(cTopo.Neighbors))
		for ix, neighborTopo := range cTopo.Neighbors {
			nLocOptimized := st.chunks[neighborTopo.ChunkId]
			maybeRemoteCK := nLocOptimized.GetRemoteCacheKey()
			if maybeRemoteCK != nil && ip == maybeRemoteCK.Ip {
				nLocOptimized = &api.ChunkDataLocator{
					Location: &api.ChunkDataLocator_SelfCacheKey{maybeRemoteCK.CacheKey},
				}
			}
			inputs[ix] = &api.StepChunkQ_Input{
				Dp:   &api.ChunkRel{neighborTopo.Dx, neighborTopo.Dy},
				Data: nLocOptimized,
			}
		}
		stepChunkQ := &api.StepChunkQ{
			ChunkInput: append(inputs, &api.StepChunkQ_Input{
				Dp:   &api.ChunkRel{Dx: 0, Dy: 0},
				Data: st.chunks[cTopo.ChunkId],
			}),
		}

		wg.Add(1)
		go func(chunkId string) {
			defer wg.Done()
			service := api.NewChunkServiceClient(connCache[ip])
			strictCtx, _ := context.WithTimeout(context.Background(), 100000*time.Millisecond)
			s, err := service.StepChunk(strictCtx, stepChunkQ)
			if err != nil {
				log.Printf("ERROR: StepChunk@%s failed with %v", ip, err)
				return
			}
			if !s.Success {
				log.Printf("ERROR StepChunk failed")
				return
			}
			newChunks[chunkId] = &api.ChunkDataLocator{
				Location: &api.ChunkDataLocator_RemoteCacheKey{
					RemoteCacheKey: &api.RemoteChunkCache{
						Ip:       ip,
						CacheKey: s.CacheKey,
					},
				},
			}
		}(cTopo.ChunkId)
	}
	wg.Wait()
	if len(newChunks) != len(st.bsTopo.GetChunkTopos()) {
		log.Printf("ERROR some of chunk stepping failed (%d success/%d) prev:%#v new:%#v",
			len(newChunks), len(st.bsTopo.GetChunkTopos()),
			st, newChunks)
		return nil
	}
	return &biosphereState{
		bsTopo:    st.bsTopo,
		timestamp: st.timestamp + 1,
		chunks:    newChunks,
	}
}

// Assign chunks to given nodes with affinity consideration, and then
// returns chunk id -> node mapping.
func assignChunks(bsTopo BiosphereTopology, nodes []string) map[string]string {
	chunks := make(map[string]string)
	for chunkId, lsh := range bsTopo.GetLSHs() {
		workerIx := int(math.Floor(lsh * float64(len(nodes))))
		if workerIx >= len(nodes) {
			workerIx = len(nodes) - 1
		}
		chunks[chunkId] = nodes[workerIx]
	}
	return chunks
}

func (ctrl *Controller) GetCurrentState() map[uint64]BiosphereState {
	biospheres := make(map[uint64]BiosphereState)
	for bsId, _ := range ctrl.execution {
		biospheres[bsId] = BiosphereState{
			flag: Running,
		}
	}
	return biospheres
}

func (ctrl *Controller) getInitialDataLocators(timestamp uint64, bsTopo BiosphereTopology) map[string]*api.ChunkDataLocator {
	ctx := context.Background()
	client, err := ctrl.fe.AuthDatastore(ctx)
	if err != nil {
		log.Printf("ERROR: Datastore failed with %#v", err)
		return nil
	}

	var wg sync.WaitGroup
	var csLock sync.Mutex
	chunks := make(map[string]*api.ChunkDataLocator)
	for _, chunkTopo := range bsTopo.GetChunkTopos() {
		wg.Add(1)
		go func(chunkId string) {
			defer wg.Done()
			query := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", chunkId).Filter("Timestamp=", int64(timestamp)).KeysOnly()
			ks, err := client.GetAll(ctx, query, nil)
			if err != nil || len(ks) == 0 {
				log.Printf("ERROR Specified timestamp & chunkId was not found query=%#v err=%#v keys=%#v", query, err, ks)
				return
			}
			if len(ks) > 1 {
				log.Printf("WARNING Multiple keys found for query=%#v keys=%#v. Using the first one", query, ks)
			}
			csLock.Lock()
			defer csLock.Unlock()
			chunks[chunkId] = &api.ChunkDataLocator{
				Location: &api.ChunkDataLocator_DatastoreKey{
					DatastoreKey: ks[0].ID(),
				},
			}
		}(chunkTopo.ChunkId)
	}
	wg.Wait()
	if len(chunks) != len(bsTopo.GetChunkTopos()) {
		log.Printf("ERROR Some PersistentChunkSnapshot key was not found. Returning nil initial data.")
		return nil
	}
	return chunks
}

func (ctrl *Controller) getMaxPersistedTimestamp(ts *TargetState) uint64 {
	ctx := context.Background()
	client, err := ctrl.fe.AuthDatastore(ctx)
	if err != nil {
		log.Printf("ERROR: Datastore failed with %#v", err)
		return 0
	}

	firstChunkId := ts.BsTopo.GetChunkTopos()[0].ChunkId
	// TODO: This is slow. Do something.
	query := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", firstChunkId)
	var ss []*PersistentChunkSnapshot
	_, err = client.GetAll(ctx, query, &ss)
	if err != nil {
		log.Printf("ERROR: failed to retrieve max timestamp %v", err)
		return 0
	}
	maxTimestamp := uint64(0)
	for _, snapshot := range ss {
		if uint64(snapshot.Timestamp) > maxTimestamp {
			maxTimestamp = uint64(snapshot.Timestamp)
		}
	}
	return maxTimestamp
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
func (ctrl *Controller) acquireUsableChunkConn() map[string]*grpc.ClientConn {
	ctrl.connCacheLock.Lock()
	defer ctrl.connCacheLock.Unlock()

	ips := GetChunkIps()
	conns := make(map[string]*grpc.ClientConn)
	for _, ip := range ips {
		connUsage := ctrl.connCache[ip]
		if connUsage == nil {
			conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
				grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
			if err != nil {
				log.Printf("WARNING Couldn't connect to chunk server %s, ignoring. %v", ip, err)
				continue
			}
			connUsage = &GrpcConnUsage{conn: conn, numRef: 0}
			ctrl.connCache[ip] = connUsage
		}
		connUsage.numRef++
		conns[ip] = connUsage.conn
	}
	return conns
}

// Release ips. Internally, connection cache is referecen-counted and closed when
// all acquired connections are released.
func (ctrl *Controller) releaseChunkConn(ips []string) {
	ctrl.connCacheLock.Lock()
	defer ctrl.connCacheLock.Unlock()

	for _, ip := range ips {
		connUsage := ctrl.connCache[ip]
		if connUsage == nil || connUsage.numRef == 0 {
			log.Panicf("ERROR releaseChunkConn called for non-existing (or already closed conn) %s", ip)
		}
		connUsage.numRef--
		if connUsage.numRef == 0 {
			log.Printf("INFO Closing chunk server connection %s", ip)
			connUsage.conn.Close()
			delete(ctrl.connCache, ip)
		}
	}
}

// Get short-term connection without increasing refcount.
// Caller must make sure (last) releaseChunkConn is not yet called while the returned
// connection for the ip is in use.
// Returns nil if not possible.
func (ctrl *Controller) getChunkConn(ip string) *grpc.ClientConn {
	ctrl.connCacheLock.Lock()
	defer ctrl.connCacheLock.Unlock()
	connUsage := ctrl.connCache[ip]
	if connUsage == nil {
		return nil
	}
	return connUsage.conn
}

const chunkIdFormat = "%d-%d:%d"
