package main

import (
	"./api"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/cloud/datastore"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Just before registering chunk
	STEPPING_EVENT_REGISTER = "register"

	// Just before starting to resuming from snapshots in datastore.
	STEPPING_EVENT_RESUME = "resume"

	// When chunk.Step is called
	STEPPING_EVENT_STEP = "step"

	// When waiting for neighbor packets.
	STEPPING_EVENT_WAIT_NEIGHBOR = "wait"

	STEPPING_EVENT_MULTICAST = "multicast"

	STEPPING_EVENT_QUIT = "quit"
)

func (ck *CkServiceImpl) SpawnChunk(ctx context.Context, q *api.SpawnChunkQ) (*api.SpawnChunkS, error) {
	proc := ChunkProcess{
		cred:           ck.ServerCred,
		router:         ck.ChunkRouter,
		topo:           q.Topology,
		snapshotModulo: q.SnapshotModulo,
		recordModulo:   q.RecordModulo,
		frameWait:      time.Duration(q.FrameWaitNs),
	}
	go proc.RunChunk(q)
	return &api.SpawnChunkS{}, nil
}

type ChunkProcess struct {
	cred           *ServerCred
	router         *ChunkRouter
	topo           *api.ChunkTopology
	snapshotModulo int32
	recordModulo   int32
	frameWait      time.Duration

	// Derived in RunChunk
	relToId map[ChunkRel]string
	idToRel map[string]ChunkRel
	wall    *ChunkWall
	chunk   *GrainChunk
}

type SteppingEvent struct {
	ChunkId        string
	EventType      string
	ChunkTimestamp uint64
	StartAtNano    int64
}

func NewEvent(eventType string, chunkId string, chunkTimestamp uint64) *SteppingEvent {
	return &SteppingEvent{
		ChunkId:        chunkId,
		EventType:      eventType,
		ChunkTimestamp: chunkTimestamp,
		StartAtNano:    time.Now().UnixNano(),
	}
}

// You should call this before checking errors, so that event will be logged to BQ.
func (ev *SteppingEvent) End(cred *ServerCred) {
	ctx := context.Background()
	EndAtNano := time.Now().UnixNano()

	bqSvc, err := cred.AuthBigquery(ctx)
	if err != nil {
		log.Printf("WARNING: Failed to log stepping event with %#v", err)
		return
	}
	row := &bigquery.TableDataInsertAllRequestRows{
		Json: map[string]bigquery.JsonValue{
			"start_at":        float64(ev.StartAtNano) * 1e-9,
			"end_at":          float64(EndAtNano) * 1e-9,
			"machine_ip":      os.Getenv("HOSTNAME"),
			"chunk_id":        ev.ChunkId,
			"chunk_timestamp": ev.ChunkTimestamp,
			"event_type":      ev.EventType,
		},
	}
	tableSvc := bigquery.NewTabledataService(bqSvc)
	q := &bigquery.TableDataInsertAllRequest{
		Rows: []*bigquery.TableDataInsertAllRequestRows{row},
	}
	_, err = tableSvc.InsertAll(ProjectId, BigqueryPlatformDatasetId, BigquerySteppingTableId, q).Do()
	if err != nil {
		log.Printf("WARNING: Failed to log stepping event with %#v", err)
		return
	}
}

func (proc *ChunkProcess) RunChunk(q *api.SpawnChunkQ) {
	ctx := context.Background()

	// Register chunk earliest to get ready for reciving packets from neighbors.
	regEv := NewEvent(STEPPING_EVENT_REGISTER, proc.topo.ChunkId, q.StartTimestamp)
	chunkMeta := proc.router.RegisterNewChunk(proc.topo)
	go regEv.End(proc.cred)
	if chunkMeta == nil {
		log.Printf("RunChunk(%s) exiting because it's already running", proc.topo.ChunkId)
		return
	}

	proc.relToId, proc.idToRel, proc.wall = decodeTopo(proc.topo)
	resumeEv := NewEvent(STEPPING_EVENT_RESUME, proc.topo.ChunkId, q.StartTimestamp)
	loadedChunk, err := resumeFromSnapshot(ctx, proc.topo.ChunkId, q.StartTimestamp, proc.cred)
	go resumeEv.End(proc.cred)
	if err != nil {
		log.Printf("Resuming failed with %#v, not starting %s", err, proc.topo.ChunkId)
		return
	}
	proc.chunk = loadedChunk

	// Post initial empty state to unblock other chunks.
	mcEv := NewEvent(STEPPING_EVENT_MULTICAST, proc.topo.ChunkId, q.StartTimestamp)
	grains := make([]*api.Grain, len(proc.chunk.Grains))
	for ix, grain := range proc.chunk.Grains {
		grains[ix] = ser(grain)
	}
	proc.router.MulticastToNeighbors(proc.topo.Neighbors, &NeighborExport{
		OriginChunkId: proc.topo.ChunkId,
		Timestamp:     proc.chunk.Timestamp,
		ChunkGrains:   grains,
		EscapedGrains: make(map[string][]*api.Grain),
	})
	go mcEv.End(proc.cred)

	waitEv := NewEvent(STEPPING_EVENT_WAIT_NEIGHBOR, proc.topo.ChunkId, q.StartTimestamp)
	currentNeighbors := make(map[string]*NeighborExport)
	var futurePackets []*NeighborExport
	for {
		select {
		case <-chunkMeta.quitCh:
			go waitEv.End(proc.cred)

			quitEv := NewEvent(STEPPING_EVENT_QUIT, proc.topo.ChunkId, proc.chunk.Timestamp)
			log.Printf("INFO: chunkId=%s: Quit signal received", proc.topo.ChunkId)
			go quitEv.End(proc.cred)
			break
		case packet := <-chunkMeta.recvCh:
			// Packet filtering.
			if packet.Timestamp < proc.chunk.Timestamp {
				log.Printf("ERROR: chunkId=%s (t=%d): Dropped too old incoming neighbor packet (packet chunk=%s, timestamp=%d)",
					proc.topo.ChunkId, proc.chunk.Timestamp, packet.OriginChunkId, packet.Timestamp)
				continue
			} else if packet.Timestamp > proc.chunk.Timestamp {
				futurePackets = append(futurePackets, packet)
				continue
			} else {
				currentNeighbors[packet.OriginChunkId] = packet
				if len(currentNeighbors) < len(proc.topo.Neighbors) {
					// Not enough packets collected.
					continue
				}
			}

			// All neighor packets are available.
			go waitEv.End(proc.cred)

			proc.assembleAndStep(ctx, currentNeighbors)

			waitEv = NewEvent(STEPPING_EVENT_WAIT_NEIGHBOR, proc.topo.ChunkId, proc.chunk.Timestamp)
			currentNeighbors = make(map[string]*NeighborExport)
			for _, fp := range futurePackets {
				chunkMeta.recvCh <- fp
			}
			futurePackets = nil
		}
	}
}

func (proc *ChunkProcess) assembleAndStep(ctx context.Context, neighbors map[string]*NeighborExport) {
	var incomingGrains []*Grain
	var envGrains []*Grain
	for _, packet := range neighbors {
		rel := proc.idToRel[packet.OriginChunkId]
		deltaPos := Vec3f{float32(rel.Dx), float32(rel.Dy), 0}

		// Convert and append to incomingGrains
		for _, grainProto := range packet.EscapedGrains[proc.topo.ChunkId] {
			grain := deser(grainProto)
			incomingGrains = append(incomingGrains, grain)
		}

		// Create envgrains
		for _, grainProto := range packet.ChunkGrains {
			grain := deser(grainProto)
			grain.Position = grain.Position.Add(deltaPos)
			envGrains = append(envGrains)
		}
	}
	proc.chunk.IncorporateAddition(incomingGrains)

	// Persist when requested.
	if proc.snapshotModulo > 0 && proc.chunk.Timestamp%uint64(proc.snapshotModulo) == 0 {
		key, err := takeSnapshot(ctx, proc.topo.ChunkId, proc.cred, proc.chunk)
		if err != nil {
			log.Printf("Error: Failed to take snapshot with %#v", err)
		}
		log.Printf("Snapshot key=%v", key)
	}
	// Stream to bigquery when requested.
	if proc.recordModulo > 0 && proc.chunk.Timestamp%uint64(proc.recordModulo) == 0 {
		err := takeRecord(ctx, proc.topo.ChunkId, proc.cred, proc.chunk)
		if err != nil {
			log.Printf("Error: Failed to record with %# v", pretty.Formatter(err))
		}
	}

	// Actual simulation.
	stepEv := NewEvent(STEPPING_EVENT_STEP, proc.topo.ChunkId, proc.chunk.Timestamp)
	escapedGrains := proc.chunk.Step(envGrains, proc.wall)
	go stepEv.End(proc.cred)
	if proc.frameWait > 0 {
		time.Sleep(proc.frameWait)
	}

	// Pack exported things.
	mcEv := NewEvent(STEPPING_EVENT_MULTICAST, proc.topo.ChunkId, proc.chunk.Timestamp)
	grains := make([]*api.Grain, len(proc.chunk.Grains))
	for ix, grain := range proc.chunk.Grains {
		grains[ix] = ser(grain)
	}
	bins := make(map[string][]*api.Grain)
	for _, escapedGrain := range escapedGrains {
		coord := binExternal(proc.relToId, escapedGrain.Position)
		if coord == nil {
			continue
		}
		sGrain := ser(escapedGrain)
		sGrain.Pos = &api.CkPosition{
			coord.Pos.X, coord.Pos.Y, coord.Pos.Z,
		}
		bins[coord.Key] = append(bins[coord.Key], sGrain)
	}
	packet := &NeighborExport{
		OriginChunkId: proc.topo.ChunkId,
		Timestamp:     proc.chunk.Timestamp,
		ChunkGrains:   grains,
		EscapedGrains: bins,
	}
	proc.router.MulticastToNeighbors(proc.topo.Neighbors, packet)
	go mcEv.End(proc.cred)
}

func resumeFromSnapshot(ctx context.Context, chunkId string, startTimestamp uint64, cred *ServerCred) (*GrainChunk, error) {
	client, err := cred.AuthDatastore(ctx)
	if err != nil {
		return nil, err
	}

	// Find resuming point and delete snapshots after it.
	// This is super inefficient.
	qSnapshots := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", chunkId)
	var ss []*PersistentChunkSnapshot
	keys, err := client.GetAll(ctx, qSnapshots, &ss)
	if err != nil {
		return nil, err
	}
	var keysToDelete []*datastore.Key
	var resumePoint *PersistentChunkSnapshot
	for ix, snapshot := range ss {
		if uint64(snapshot.Timestamp) == startTimestamp {
			resumePoint = snapshot
		} else if uint64(snapshot.Timestamp) > startTimestamp {
			keysToDelete = append(keysToDelete, keys[ix])
		}
	}
	if resumePoint == nil {
		return nil, errors.New(fmt.Sprintf("PersistentChunkSnapshot(id=%s, t=%d) not found", chunkId, startTimestamp))
	}

	// Initialize chunk from snapshot.
	snapshotProto := &api.ChunkSnapshot{}
	err = proto.Unmarshal(resumePoint.Snapshot, snapshotProto)
	if err != nil {
		return nil, err
	}
	chunk := NewGrainChunk(false)
	chunk.Timestamp = startTimestamp
	chunk.Grains = make([]*Grain, len(snapshotProto.Grains))
	for ix, grainProto := range snapshotProto.Grains {
		chunk.Grains[ix] = deser(grainProto)
	}

	// Only after confirming successful chunk resuming, delete snapshots after resume point.
	err = client.DeleteMulti(ctx, keysToDelete)
	if err != nil {
		log.Printf("Error: Failed to delete %d snapshots when resuming from t=%d: %#v", len(keysToDelete), startTimestamp, keysToDelete)
	}
	return chunk, nil
}

func decodeTopo(topo *api.ChunkTopology) (map[ChunkRel]string, map[string]ChunkRel, *ChunkWall) {
	relToId := make(map[ChunkRel]string)
	idToRel := make(map[string]ChunkRel)
	for _, neighbor := range topo.Neighbors {
		rel := ChunkRel{int(neighbor.Dx), int(neighbor.Dy)}
		relToId[rel] = neighbor.ChunkId
		idToRel[neighbor.ChunkId] = rel
	}
	_, canPassXm := relToId[ChunkRel{-1, 0}]
	_, canPassXp := relToId[ChunkRel{1, 0}]
	_, canPassYm := relToId[ChunkRel{0, -1}]
	_, canPassYp := relToId[ChunkRel{0, 1}]
	wall := &ChunkWall{
		Xm: !canPassXm,
		Xp: !canPassXp,
		Ym: !canPassYm,
		Yp: !canPassYp,
	}
	return relToId, idToRel, wall
}

func takeSnapshot(ctx context.Context, chunkId string, cred *ServerCred, chunk *GrainChunk) (*datastore.Key, error) {
	client, err := cred.AuthDatastore(ctx)
	if err != nil {
		return nil, err
	}

	grains := make([]*api.Grain, len(chunk.Grains))
	for ix, grain := range chunk.Grains {
		grains[ix] = ser(grain)
	}
	ssBlob, err := proto.Marshal(&api.ChunkSnapshot{
		Grains: grains,
	})
	if err != nil {
		return nil, err
	}

	log.Printf("Snapshotting at t=%d size=%d", chunk.Timestamp, len(ssBlob))
	key := datastore.NewIncompleteKey(ctx, "PersistentChunkSnapshot", nil)
	key, err = client.Put(ctx, key, &PersistentChunkSnapshot{
		ChunkId:   chunkId,
		Timestamp: int64(chunk.Timestamp),
		Snapshot:  ssBlob,
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

func takeRecord(ctx context.Context, chunkId string, cred *ServerCred, chunk *GrainChunk) error {
	// Skip streaming when the chunk is empty.
	if len(chunk.Grains) == 0 {
		return nil
	}

	bqSvc, err := cred.AuthBigquery(ctx)
	if err != nil {
		return err
	}
	log.Printf("Start takeRecord serialization")
	rows := make([]*bigquery.TableDataInsertAllRequestRows, len(chunk.Grains))
	for ix, grain := range chunk.Grains {
		rows[ix] = &bigquery.TableDataInsertAllRequestRows{Json: convertGrainToRecord(chunkId, chunk.Timestamp, grain)}
	}
	log.Printf("End takeRecord serialization / Start takeRecord request")
	tableSvc := bigquery.NewTabledataService(bqSvc)
	q := &bigquery.TableDataInsertAllRequest{
		Rows: rows,
	}
	s, err := tableSvc.InsertAll(ProjectId, BigqueryDatasetId, BigqueryGrainRecordTableId, q).Do()
	log.Printf("End takeRecord request")
	if err != nil {
		return err
	}
	if len(s.InsertErrors) > 0 {
		return fmt.Errorf(
			"%d rows failed to insert; first error: %# v",
			len(s.InsertErrors), pretty.Formatter(s.InsertErrors[0].Errors))
	}
	return nil
}

func convertGrainToRecord(chunkId string, timestamp uint64, grain *Grain) map[string]bigquery.JsonValue {
	// HACK HACK. Chunk server is not supposed to know about topology of chunks nor biosphere id.
	// format: <biosphere id> "-" <dx> ":" <dy>
	chunkIdParts := strings.Split(chunkId, "-")
	bsId, _ := strconv.ParseUint(chunkIdParts[0], 10, 64)
	dx, _ := strconv.ParseInt(strings.Split(chunkIdParts[1], ":")[0], 10, 32)
	dy, _ := strconv.ParseInt(strings.Split(chunkIdParts[1], ":")[1], 10, 32)

	absPos := Vec3f{float32(dx), float32(dy), 0}.Add(grain.Position)

	type QualPair struct {
		qual string
		num  int32
	}

	var cellprop map[string]bigquery.JsonValue
	if grain.CellProp != nil {
		qualPairs := make([]*QualPair, len(grain.CellProp.Quals))
		index := 0
		for qual, num := range grain.CellProp.Quals {
			qualPairs[index] = &QualPair{qual, num}
			index++
		}
		cellprop = map[string]bigquery.JsonValue{
			"energy": grain.CellProp.Energy,
			"cycle":  grain.CellProp.Cycle,
			"genome": grain.CellProp.Genome,
			"quals":  qualPairs,
		}
	}

	return map[string]bigquery.JsonValue{
		"biosphere_id": bsId,
		"chunk_id":     chunkId,
		"grain_id":     grain.Id,
		"timestamp":    timestamp,
		"pos": map[string]float32{
			"x": absPos.X,
			"y": absPos.Y,
			"z": absPos.Z,
		},
		"vel": map[string]float32{
			"x": grain.Velocity.X,
			"y": grain.Velocity.Y,
			"z": grain.Velocity.Z,
		},
		"kind":     int32(grain.Kind),
		"cellprop": cellprop,
	}
}

type ChunkRel struct {
	Dx, Dy int
}

type WorldCoord2 struct {
	Key string
	Pos Vec3f
}

// Convert a known-to-be-outlying point to WorldCoord.
func binExternal(relToId map[ChunkRel]string, pos Vec3f) *WorldCoord2 {
	ix := ifloor(pos.X)
	iy := ifloor(pos.Y)
	if ix == 0 && iy == 0 {
		log.Printf("Pos declared ougoing, but found in-chunk: %#v", pos)
		return nil
	}

	key, ok := relToId[ChunkRel{ix, iy}]
	if ok {
		return &WorldCoord2{key, pos.Sub(Vec3f{float32(ix), float32(iy), 0})}
	} else {
		log.Printf("Grain (pos %v) escaped to walled region, returning (0.5, 0.5, 10)", pos)
		return nil
	}
}
