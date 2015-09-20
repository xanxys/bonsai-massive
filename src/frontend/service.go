package main

import (
	"./api"
)

type FeServiceImpl struct {
}

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) *api.BiospheresS {
	name := "Hogehoge"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38
	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}

func (fe *FeServiceImpl) HandleBiosphereDelta(q *api.BiosphereDeltaQ) *api.BiospheresS {
	name := "Hogehoge"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38
	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}
}
