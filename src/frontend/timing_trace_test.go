package main

import (
	"./api"
	"encoding/json"
	"testing"
)

func TestConvertToCloudTraceSanity(t *testing.T) {
	trace := &api.TimingTrace{
		Name:     "Root",
		Start:    1234000000000, // Thu, 01 Jan 1970 00:20:34 GMT
		End:      5678000000000, //  Thu, 01 Jan 1970 01:34:38 GMT
		Children: nil,
	}

	ctReq := ConvertToCloudTrace(trace)
	if len(ctReq.Traces) != 1 {
		t.Errorf("Expecting 1 trace, found %#v", ctReq.Traces)
	}
	if len(ctReq.Traces[0].TraceId) != 32 {
		t.Errorf("Malformed trace id: %s", ctReq.Traces[0].TraceId)
	}
	if ctReq.Traces[0].ProjectId != ProjectId {
		t.Error("Wrong project id")
	}
	if len(ctReq.Traces[0].Spans) != 1 {
		t.Errorf("Wrong number of spans: %#v", ctReq.Traces[0].Spans)
	}
}

func TestCtRequestJsonSanity(t *testing.T) {
	trace := &api.TimingTrace{
		Name:     "Root",
		Start:    1234000000000, // Thu, 01 Jan 1970 00:20:34 GMT
		End:      5678000000000, //  Thu, 01 Jan 1970 01:34:38 GMT
		Children: nil,
	}
	ctReq := ConvertToCloudTrace(trace)

	// JSON encoding must have "substantial" length, not null or {}
	ctRequestJson, _ := json.Marshal(ctReq)
	if len(ctRequestJson) < 20 {
		t.Errorf("Strange encoded request: %s", string(ctRequestJson[:]))
	}
}
