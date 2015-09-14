package main

import (
	"./api"
)

type FeServiceImpl struct {
}

func (fe *FeServiceImpl) HandleWorlds(q *api.WorldsQ) *api.WorldsS {
	name := "Hogehoge"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38
	return &api.WorldsS{
		Worlds: []*api.WorldDescription{
			&api.WorldDescription{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}
