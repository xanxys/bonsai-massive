package main

import (
	"./api"
	"fmt"
	"math/rand"
	"time"
)

func InitTrace(name string) *api.TimingTrace {
	return &api.TimingTrace{
		Name:  name,
		Start: time.Now().UnixNano(),
	}
}

func FinishTrace(child, parent *api.TimingTrace) {
	if child.End == 0 {
		child.End = time.Now().UnixNano()
	}
	if parent != nil {
		parent.Children = append(parent.Children, child)
	}
}

func ConvertToCloudTrace(trace *api.TimingTrace) *CTPatchRequest {
	tf := &TraceFlattener{}
	tf.Flatten("", trace)
	ctTrace := &CTTrace{
		ProjectId: ProjectId,
		TraceId:   random128bitHex(),
		Spans:     tf.spans,
	}
	return &CTPatchRequest{
		Traces: []*CTTrace{ctTrace},
	}
}

type TraceFlattener struct {
	spans []*CTSpan
}

func (tf *TraceFlattener) Flatten(parentSpanId string, tr *api.TimingTrace) {
	// Although undocumented, spanId must be uint64 >= 1.
	// (otherwise it fails with "INVALID_ARGUMENT" error)
	spanId := fmt.Sprintf("%d", len(tf.spans)+1)
	span := &CTSpan{
		SpanId:       spanId,
		Name:         tr.Name,
		StartTime:    time.Unix(0, tr.Start).Format(time.RFC3339Nano),
		EndTime:      time.Unix(0, tr.End).Format(time.RFC3339Nano),
		ParentSpanId: parentSpanId,
		Labels:       make(map[string]string),
	}
	tf.spans = append(tf.spans, span)
	for _, childSpan := range tr.Children {
		tf.Flatten(spanId, childSpan)
	}
}

func random128bitHex() string {
	return fmt.Sprintf("%08x%08x%08x%08x", rand.Uint32(), rand.Uint32(), rand.Uint32(), rand.Uint32())
}

type CTPatchRequest struct {
	Traces []*CTTrace `json:"traces"`
}

// See https://cloud.google.com/trace/api/reference/rest/v1/projects.traces
type CTTrace struct {
	ProjectId string    `json:"projectId"`
	TraceId   string    `json:"traceId"`
	Spans     []*CTSpan `json:"spans"`
}

type CTSpan struct {
	SpanId string `json:"spanId"`
	// "SPAN_KIND_UNSPECIFIED" "RPC_SERVER" "RPC_CLIENT"
	Kind         string            `json:"kind,omitempty"`
	Name         string            `json:"name"`
	StartTime    string            `json:"startTime"`
	EndTime      string            `json:"endTime"`
	ParentSpanId string            `json:"parentSpanId,omitempty"`
	Labels       map[string]string `json:"labels"`
}
