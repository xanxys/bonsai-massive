package main

import (
	"fmt"
	"google.golang.org/api/compute/v1"
	"log"
	"math/rand"
	//"strings"
)

const GoogleCloudLoggingScope = "https://www.googleapis.com/auth/logging.write"

func (fe *FeServiceImpl) prepare(service *compute.Service) {
	name := fmt.Sprintf("chunk-server-%d", rand.Int63n(1000000000))
	const machineType = "n1-standard-4"

	prefix := "https://www.googleapis.com/compute/v1/projects/" + ProjectId
	// Run `gcloud compute images list --project google-containers`
	// to see list of container names.
	imageURL := "https://www.googleapis.com/compute/v1/projects/google-containers/global/images/container-vm-v20151215"

	// TODO: migrate to kubelet manifest file.
	/*
		startupScript := strings.Join(
			[]string{
				`#!/bin/bash`,
				`METADATA=http://metadata.google.internal./computeMetadata/v1`,
				`SVC_ACCT=$METADATA/instance/service-accounts/default`,
				`ACCESS_TOKEN=$(curl -H 'Metadata-Flavor: Google' $SVC_ACCT/token | cut -d'"' -f 4)`,
				`docker login -e dummy@example.com -u _token -p $ACCESS_TOKEN https://gcr.io`,
				`curl -sSO https://dl.google.com/cloudagents/install-logging-agent.sh`,
				`sha256sum install-logging-agent.sh`,
				`bash install-logging-agent.sh`,
				`printf '<source>\ntype forward \nport 24224\n</source>\n' > /etc/google-fluentd/config.d/docker.conf`,
				`service google-fluentd restart`,
				fmt.Sprintf(`docker pull %s`, fe.chunkContainerName),
				fmt.Sprintf(`docker run -d --log-driver=fluentd --publish 9000:9000 %s`, fe.chunkContainerName),
			}, "\n")
	*/

	containerManifest := fmt.Sprintf(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"name": "bonsai-chunk"
			},
			"spec": {
				"volumes": [{
					"name": "log-storage",
					"emptyDir": {
						"medium": "Memory"
					}
				}],
				"containers": [
					{
						"name": "bonsai-chunk-server",
						"image": "%s",
						"imagePullPolicy": "Always",
						"ports": [{"containerPort": 9000, "hostPort": 9000}],
						"env": [{
							"name": "LOG_PATH",
							"value": "/mnt/log/CHUNK_LOG"
						}],
						"volumeMounts": [{
							"name": "log-storage",
							"mountPath": "/mnt/log"
						}]
					},
					{
						"name": "bonsai-chunk-log-collector",
						"image": "gcr.io/google_containers/fluentd-sidecar-gcp:1.1",
						"env": [{
							"name": "FILES_TO_COLLECT",
							"value": "/mnt/log/CHUNK_LOG"
						}],
						"volumeMounts": [{
							"name": "log-storage",
							"readOnly": true,
							"mountPath": "/mnt/log"
						}]
					}
				]
			}
		}`, fe.chunkContainerName)

	bonsaiType := "chunk-" + fe.envType

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
					GoogleCloudLoggingScope,
				},
			},
		},
		Scheduling: &compute.Scheduling{
			Preemptible:       true,
			OnHostMaintenance: "TERMINATE",
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				/*
					{
						Key:   "startup-script",
						Value: &startupScript,
					},
				*/
				{
					Key:   "bonsai-type",
					Value: &bonsaiType,
				},
				{
					Key:   "google-container-manifest",
					Value: &containerManifest,
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

func (fe *FeServiceImpl) deleteInstances(service *compute.Service, names []string) {
	for _, name := range names {
		op, err := service.Instances.Delete(ProjectId, zone, name).Do()
		log.Printf("Op: %#v Err:%#v\n", op, err)
		if op != nil {
			if op.Error != nil {
				log.Printf("Error while Instances.Delete: %#v", op.Error)
			}
		}
	}
}
