package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"math"
	"reflect"
	"sort"
	"time"
)

const cooldownPeriodSecond = 120.0

type PoolController struct {
	fe                *FeServiceImpl
	configHandler     ConfigHandler
	lastGrpcOkIp      []string
	targetNum         int
	lastNonZeroTarget time.Time
}

type ConfigHandler interface {
	PostChange()
}

func NewPoolController(fe *FeServiceImpl, handler ConfigHandler) *PoolController {
	ctrl := &PoolController{
		fe:            fe,
		configHandler: handler,
	}
	go func() {
		for loopIter := 0; true; loopIter++ {
			time.Sleep(10 * time.Second)
			ctrl.LoopIter(loopIter)
		}
	}()
	return ctrl
}

func (ctrl *PoolController) GetDebug() *api.PoolDebug {
	now := time.Now()
	debug := &api.PoolDebug{
		GrpcOkIp:    ctrl.lastGrpcOkIp,
		TargetNum:   int32(ctrl.targetNum),
		LastNonZero: ctrl.lastNonZeroTarget.Format(time.RFC3339),
		CurrentTime: now.Format(time.RFC3339),
	}
	if ctrl.targetNum == 0 && now.Sub(ctrl.lastNonZeroTarget).Seconds() < cooldownPeriodSecond {
		debug.IsCooldown = true
		debug.CooldownRemaining = fmt.Sprintf("%.1f sec", now.Sub(ctrl.lastNonZeroTarget).Seconds())
	}
	return debug
}

func (ctrl *PoolController) GetUsableIp() []string {
	return ctrl.lastGrpcOkIp
}

func (ctrl *PoolController) LoopIter(loopIter int) {
	ctx := context.Background()
	chunkInstances, err := ctrl.fe.GetChunkServerInstances(ctx)
	if err != nil {
		log.Printf("loop(%d): Error while fetching instance list %v", loopIter, err)
		return
	}
	var grpcOkIp []string
	for _, chunkInstance := range chunkInstances {
		ip := chunkInstance.NetworkInterfaces[0].NetworkIP
		if isGrpcOk(ip) {
			grpcOkIp = append(grpcOkIp, ip)
		}
	}
	sort.Strings(grpcOkIp)
	ipListChanged := !reflect.DeepEqual(ctrl.lastGrpcOkIp, grpcOkIp)
	ctrl.lastGrpcOkIp = grpcOkIp
	if ipListChanged {
		log.Printf("Notifying ip change (lastGrpcOkIp=%v)", ctrl.lastGrpcOkIp)
		ctrl.configHandler.PostChange()
	}

	// Check gRPC status and notify if delta is detected.
	targetNum := ctrl.GetTargetNum()
	if targetNum > len(chunkInstances) {
		// Spawn chunk servers.
		deltaNum := targetNum - len(chunkInstances)
		clientCompute, err := ctrl.fe.AuthCompute(ctx)
		if err != nil {
			log.Printf("loop(%d): Error in compute API auth: %v", loopIter, err)
			return
		}
		for ix := 0; ix < deltaNum; ix++ {
			ctrl.fe.prepare(clientCompute)
		}
	} else if targetNum < len(chunkInstances) {
		numToKill := len(chunkInstances) - targetNum
		clientCompute, err := ctrl.fe.AuthCompute(ctx)
		if err != nil {
			log.Printf("loop(%d): Error in compute auth: %v", loopIter, err)
			return
		}
		// TODO: prefer to kill non-ready instances.
		namesToKill := make([]string, numToKill)
		for ix, chunkInstance := range chunkInstances {
			if ix < numToKill {
				namesToKill[ix] = chunkInstance.Name
			} else {
				break
			}
		}
		ctrl.fe.deleteInstances(clientCompute, namesToKill)
	}
	if ctrl.targetNum > 0 {
		ctrl.lastNonZeroTarget = time.Now()
	}
}

func isGrpcOk(ip string) bool {
	conn, err := grpc.Dial(fmt.Sprintf("%s:9000", ip),
		grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(100*time.Millisecond))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Return current target number of instances. Note that this can change over
// time because of cooldown strategy.
func (ctrl *PoolController) GetTargetNum() int {
	if ctrl.targetNum > 0 {
		return ctrl.targetNum
	} else if time.Now().Sub(ctrl.lastNonZeroTarget).Seconds() < cooldownPeriodSecond {
		return 1
	} else {
		return 0
	}
}

func (ctrl *PoolController) SetTargetCores(cores float64) {
	ctrl.targetNum = int(math.Ceil(cores / CorePerMachine))
}
