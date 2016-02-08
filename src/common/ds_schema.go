package main

type PersistentChunkSnapshot struct {
	ChunkId   string
	Timestamp int64
	Snapshot  []byte `datastore:",noindex"`
}

type BiosphereMeta struct {
	Name string
	Nx   int32
	Ny   int32
	// Serialized api.BiosphereEnvConfig.
	Env []byte
}
