package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/compute/v1"
	"google.golang.org/cloud/datastore"
	"log"
	"math/rand"
	"strings"
)

func (fe *FeServiceImpl) HandleBiosphereDelta(q *api.BiosphereDeltaQ) (*api.BiospheresS, error) {
	ctx := context.Background()

	name := "FugaFuga"
	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	client, err := fe.authDatastore(ctx)
	if err != nil {
		return nil, err
	}
	key := datastore.NewIncompleteKey(ctx, "BiosphereMeta", nil)
	// TODO: check collision with existing name / empty names etc.
	_, err = client.Put(ctx, key, &BiosphereMeta{
		Name: q.GetDesc().Name,
	})
	if err != nil {
		return nil, err
	}

	clientCompute, err := fe.authCompute(ctx)
	if err != nil {
		return nil, err
	}
	fe.prepare(clientCompute)

	return &api.BiospheresS{
		Biospheres: []*api.BiosphereDesc{
			&api.BiosphereDesc{
				Name:     name,
				NumCores: nCores,
				NumTicks: nTicks,
			},
		},
	}, nil
}

func (fe *FeServiceImpl) prepare(service *compute.Service) {
	name := fmt.Sprintf("chunk-server-%d", rand.Int63n(1000000000))
	const machineType = "n1-standard-4"

	prefix := "https://www.googleapis.com/compute/v1/projects/" + ProjectId
	// Run `gcloud compute images list --project google-containers`
	// to see list of container names.
	imageURL := "https://www.googleapis.com/compute/v1/projects/google-containers/global/images/container-vm-v20151103"

	// TODO: migrate to kubelet manifest file.
	startupScript := strings.Join(
		[]string{
			`#!/bin/bash`,
			`METADATA=http://metadata.google.internal./computeMetadata/v1`,
			`SVC_ACCT=$METADATA/instance/service-accounts/default`,
			`ACCESS_TOKEN=$(curl -H 'Metadata-Flavor: Google' $SVC_ACCT/token | cut -d'"' -f 4)`,
			`docker login -e dummy@example.com -u _token -p $ACCESS_TOKEN https://gcr.io`,
			fmt.Sprintf(`docker pull %s`, fe.chunkContainerName),
			fmt.Sprintf(`docker run --publish 9000:9000 %s`, fe.chunkContainerName),
		}, "\n")

	bonsaiType := "chunk"

	instance := &compute.Instance{
		Name:        name,
		Description: "Exposes a set of chunks as gRPC service.",
		MachineType: fmt.Sprintf("%s/zones/%s/machineTypes/%s", prefix, zone, machineType),
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "root-pd-" + name,
					SourceImage: imageURL,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: prefix + "/global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.DevstorageFullControlScope,
					compute.ComputeScope,
				},
			},
		},
		Scheduling: &compute.Scheduling{
			Preemptible:       true,
			OnHostMaintenance: "TERMINATE",
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "startup-script",
					Value: &startupScript,
				},
				{
					Key:   "bonsai-type",
					Value: &bonsaiType,
				},
			},
		},
	}

	op, err := service.Instances.Insert(ProjectId, zone, instance).Do()
	log.Printf("Op: %#v   Err:%#v\n", op, err)
	if op != nil {
		if op.Error != nil {
			log.Printf("Error while booting: %#v", op.Error)
		}
	}

	// Wait for all instances in parallel.
	// We can return immediately, because calling Discard() before instances become ready
	// will be ok because instances are already in PENDING state.
	/*
		services := make(chan Rpc, provider.instanceNum)
		go func(name string) {
			for {
				log.Printf("Pinging status for %s\n", name)
				resp, _ := service.Instances.Get(provider.projectId, provider.zone, name).Do()
				if resp != nil && resp.Status == "RUNNING" && len(resp.NetworkInterfaces) > 0 {
					ip := resp.NetworkInterfaces[0].AccessConfigs[0].NatIP
					url := fmt.Sprintf("http://%s:8000", ip)
					BlockUntilAvailable(url, 5*time.Second)
					services <- NewHttpRpc(url)
					return
				}
				time.Sleep(5 * time.Second)
			}
		}(name)
		return services
	*/
}
