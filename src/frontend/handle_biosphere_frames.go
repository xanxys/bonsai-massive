package main

import (
	"./api"
	"errors"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/cloud/datastore"
	"log"
	"math"
	"math/rand"
)

func (fe *FeServiceImpl) BiosphereFrames(ctx context.Context, q *api.BiosphereFramesQ) (*api.BiosphereFramesS, error) {
	ctx = TraceStart(ctx, "/frontend.BiosphereFrames")
	defer TraceEnd(ctx, fe.ServerCred)

	type snapshotFetchResult struct {
		ChunkId  string
		Snapshot *api.ChunkSnapshot
		Trace    *api.TimingTrace
	}

	bsTopo, _, err, topoTrace := fe.getBiosphereTopo(ctx, q.BiosphereId)
	if err != nil {
		return nil, err
	}
	FinishTrace(topoTrace, GetCurrentTrace(ctx))

	snapshots := make(map[string]*api.ChunkSnapshot)
	var timestamp uint64
	fetchTrace := InitTrace("/frontend._.FetchSnapshot")
	if q.FetchSnapshot {
		client, err := fe.AuthDatastore(ctx)
		if err != nil {
			return nil, err
		}

		snapshotCh := make(chan *snapshotFetchResult, 5)
		for _, chunkTopo := range bsTopo.GetChunkTopos() {
			go func(chunkId string) {
				fetchChunkTrace := InitTrace("/frontend._.FetchChunkSnapshot")
				query := datastore.NewQuery("PersistentChunkSnapshot").Filter("ChunkId=", chunkId).Filter("Timestamp=", int64(q.SnapshotTimestamp)).Limit(1)
				var ss []*PersistentChunkSnapshot
				_, err := client.GetAll(ctx, query, &ss)
				if err != nil {
					log.Printf("ERROR: %#v", err)
					snapshotCh <- nil
					return
				}
				if len(ss) < 1 {
					log.Printf("ERROR: snapshot %s not found", chunkId)
					snapshotCh <- nil
					return
				}
				snapshotProto := &api.ChunkSnapshot{}
				err = proto.Unmarshal(ss[0].Snapshot, snapshotProto)
				if err != nil {
					log.Printf("ERROR: parse failed %#v", err)
					snapshotCh <- nil
					return
				}
				FinishTrace(fetchChunkTrace, nil)
				snapshotCh <- &snapshotFetchResult{chunkId, snapshotProto, fetchChunkTrace}
			}(chunkTopo.ChunkId)
		}
		// Wait for completion or failure.
		for range bsTopo.GetChunkTopos() {
			select {
			case s := <-snapshotCh:
				if s == nil {
					return nil, errors.New("Some of snapshot fethches failed")
				}
				snapshots[s.ChunkId] = s.Snapshot
				FinishTrace(s.Trace, fetchTrace)
			}
		}
		timestamp = q.SnapshotTimestamp
	} else {
		snapshots = fe.controller.GetLatestSnapshot(q.BiosphereId)
		if snapshots == nil {
			log.Printf("WARNING snapshot for bsId=%d failed", q.BiosphereId)
		}
	}
	FinishTrace(fetchTrace, GetCurrentTrace(ctx))

	var maybeCone *OrientedCone
	if q.VisibleRegion != nil {
		maybeCone = NewCone(q.VisibleRegion)
	}
	return &api.BiosphereFramesS{
		ContentTimestamp: timestamp,
		Points:           snapshotToPointCloud(maybeCone, bsTopo, snapshots),
		Stat:             snapshotToStat(snapshots),
		Cells:            snapshotToCellStats(bsTopo, snapshots),
	}, nil
}

func snapshotToPointCloud(maybeCone *OrientedCone, bsTopo BiosphereTopology, snapshot map[string]*api.ChunkSnapshot) *api.PointCloud {
	var points []*api.PointCloud_Point
	offsets := bsTopo.GetGlobalOffsets()
	countTotal := 0
	countAdded := 0
	for chunkId, chunkSnapshot := range snapshot {
		offset := offsets[chunkId]
		for _, grain := range chunkSnapshot.Grains {
			pos := Vec3f{grain.Pos.X, grain.Pos.Y, grain.Pos.Z}.Add(offset)
			if maybeCone != nil && !maybeCone.Contains(pos) {
				continue
			}
			baseColor := Vec3f{0, 0, 0}
			if grain.Kind == api.Grain_WATER {
				baseColor = Vec3f{0.4, 0.4, 1}
			} else if grain.Kind == api.Grain_SOIL {
				baseColor = Vec3f{0.8, 0.4, 0.3}
			} else if grain.Kind == api.Grain_CELL {
				baseColor = Vec3f{0.8, 0.8, 0.8}
			}
			color := baseColor.Add(Vec3f{float32(random1(grain.Id, 1416028811)), float32(random1(grain.Id, 456307397)), float32(random1(grain.Id, 386052383))}.MultS(0.2))
			points = append(points, &api.PointCloud_Point{
				Px: round1024(pos.X),
				Py: round1024(pos.Y),
				Pz: round1024(pos.Z),
				R:  round256(color.X),
				G:  round256(color.Y),
				B:  round256(color.Z),
			})
			countAdded++
		}
		countTotal += len(chunkSnapshot.Grains)
	}
	countDropped := countTotal - countAdded
	log.Printf("Mesh serializer: %d grains dropped (%f %%)", countDropped, float32(countDropped)/float32(countTotal)*100)
	return &api.PointCloud{
		Points: points,
	}
}

func snapshotToMesh(maybeCone *OrientedCone, bsTopo BiosphereTopology, snapshot map[string]*api.ChunkSnapshot) *Mesh {
	offsets := bsTopo.GetGlobalOffsets()
	mesh := NewMesh()
	countTotal := 0
	countAdded := 0
	for chunkId, chunkSnapshot := range snapshot {
		offset := offsets[chunkId]
		for _, grain := range chunkSnapshot.Grains {
			pos := Vec3f{grain.Pos.X, grain.Pos.Y, grain.Pos.Z}.Add(offset)
			if maybeCone != nil && !maybeCone.Contains(pos) {
				continue
			}
			interleave := int(math.Ceil(float64(pos.Sub(maybeCone.Pos).Length())))
			if interleave <= 0 {
				interleave = 1
			}
			if grain.Id%uint64(interleave) != 0 {
				continue
			}
			grainMesh := Icosahedron(pos, 0.06)
			baseColor := Vec3f{0, 0, 0}
			if grain.Kind == api.Grain_WATER {
				baseColor = Vec3f{0.4, 0.4, 1}
			} else if grain.Kind == api.Grain_SOIL {
				baseColor = Vec3f{0.8, 0.4, 0.3}
			} else if grain.Kind == api.Grain_CELL {
				baseColor = Vec3f{0.8, 0.8, 0.8}
			}
			grainMesh.SetColor(baseColor.Add(Vec3f{float32(random1(grain.Id, 1416028811)), float32(random1(grain.Id, 456307397)), float32(random1(grain.Id, 386052383))}.MultS(0.2)))
			countAdded++
			mesh.Merge(grainMesh)
		}
		countTotal += len(chunkSnapshot.Grains)
	}
	countDropped := countTotal - countAdded
	log.Printf("Mesh serializer: %d grains dropped (%f %%)", countDropped, float32(countDropped)/float32(countTotal)*100)
	return mesh
}

func snapshotToStat(snapshot map[string]*api.ChunkSnapshot) *api.BiosphereStat {
	stat := &api.BiosphereStat{}
	for _, chunkSnapshot := range snapshot {
		for _, grain := range chunkSnapshot.Grains {
			switch grain.Kind {
			case api.Grain_WATER:
				stat.NumWater++
			case api.Grain_SOIL:
				stat.NumSoil++
			case api.Grain_CELL:
				stat.NumCell++
			}
		}
	}
	return stat
}

func snapshotToCellStats(bsTopo BiosphereTopology, snapshot map[string]*api.ChunkSnapshot) []*api.CellStat {
	offsets := bsTopo.GetGlobalOffsets()
	var cells []*api.CellStat
	for chunkId, chunkSnapshot := range snapshot {
		offset := offsets[chunkId]
		for _, grain := range chunkSnapshot.Grains {
			if grain.Kind != api.Grain_CELL {
				continue
			}
			p := Vec3f{grain.Pos.X, grain.Pos.Y, grain.Pos.Z}.Add(offset)
			cells = append(cells, &api.CellStat{
				Prop: grain.CellProp,
				Pos:  &api.Vec3F{p.X, p.Y, p.Z},
			})
		}
	}
	return cells
}

// Wrapper of api.OrientedCone.
type OrientedCone struct {
	Pos, Dir     Vec3f
	HalfAngle    float32
	cosHalfAngle float32
}

func NewCone(cone *api.OrientedCone) *OrientedCone {
	return &OrientedCone{
		Pos:          Vec3f{cone.Px, cone.Py, cone.Pz},
		Dir:          Vec3f{cone.Dx, cone.Dy, cone.Dz},
		HalfAngle:    cone.HalfAngle,
		cosHalfAngle: float32(math.Cos(float64(cone.HalfAngle))),
	}
}

func (cone *OrientedCone) Contains(pt Vec3f) bool {
	delta := pt.Sub(cone.Pos)
	cosine := delta.Dot(cone.Dir) / delta.Length()
	return cosine >= cone.cosHalfAngle
}

func createRandomReponseForBenchmark() *api.BiosphereFramesS {
	var mesh Mesh
	for ix := 0; ix < 10000; ix++ {
		pos := Vec3f{rand.Float32(), rand.Float32(), rand.Float32()}
		grainMesh := Icosahedron(pos, 0.06)
		baseColor := Vec3f{0.5, 0.5, 0.5}
		grainMesh.SetColor(baseColor.Add(Vec3f{rand.Float32(), rand.Float32(), rand.Float32()}.MultS(0.2)))
		mesh.Merge(grainMesh)
	}
	return &api.BiosphereFramesS{
		Content:          mesh.Serialize(),
		ContentTimestamp: 0,
	}
}

func fallbackContent() *api.PolySoup {
	return Icosahedron(NewVec3f0(), 0.06).Serialize()
}
