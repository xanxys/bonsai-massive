package main

import (
	"./api"
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
