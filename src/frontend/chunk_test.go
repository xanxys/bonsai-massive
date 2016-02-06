package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestIsValidJson(t *testing.T) {
	manifest := getChunkServerManifest("hogehoge:12345")
	var data interface{}
	err := json.NewDecoder(strings.NewReader(manifest)).Decode(&data)
	if err != nil {
		t.Errorf("Chunk server manifest is not valid json: %v\nGenerated manifest:\n%s", err, manifest)
	}
}
