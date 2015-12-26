package main

import (
	"./api"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

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
}
