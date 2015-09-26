package main

import (
	"./api"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/cloud"
	"google.golang.org/cloud/datastore"
	"io/ioutil"
	"log"
	"strings"
)

const (
	WorkerContainerName   = "gcr.io/bonsai-genesis/bonsai_chunk:20150926-1932"
	WorkerPathInContainer = "/root/pentatope/worker"
)

type FeServiceImpl struct {
	cred *jwt.Config
}

func NewFeService() *FeServiceImpl {
	jsonKey, err := ioutil.ReadFile("/root/bonsai/key.json")
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.JWTConfigFromJSON(
		jsonKey,
		datastore.ScopeDatastore,
		datastore.ScopeUserEmail,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/compute")
	if err != nil {
		log.Fatal(err)
	}
	return &FeServiceImpl{
		cred: conf,
	}
}

func (fe *FeServiceImpl) authDatastore(ctx context.Context) (*datastore.Client, error) {
	client, err := datastore.NewClient(
		ctx, "bonsai-genesis", cloud.WithTokenSource(fe.cred.TokenSource(ctx)))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (fe *FeServiceImpl) authCompute(ctx context.Context) (*compute.Service, error) {
	client := fe.cred.Client(oauth2.NoContext)
	service, err := compute.New(client)
	return service, err
}

type BiosphereMeta struct {
	Name string
}

// HandleApplyChunks checks chunk servers and commit latest status to datastore
// when new state is detected. Note that this function can be called on multiple
// nodes when multiple FeServer are running.
// Do not mess up datastore.
func (fe *FeServiceImpl) HandleApplyChunks() error {
	return nil
}

func (fe *FeServiceImpl) HandleBiospheres(q *api.BiospheresQ) (*api.BiospheresS, error) {
	ctx := context.Background()

	var nCores uint32
	var nTicks uint64
	nCores = 42
	nTicks = 38

	client, err := fe.authDatastore(ctx)
	if err != nil {
		return nil, err
	}
	dq := datastore.NewQuery("BiosphereMeta")

	var metas []*BiosphereMeta
	_, err = client.GetAll(ctx, dq, &metas)
	if err != nil {
		return nil, err
	}
	var bios []*api.BiosphereDesc
	for _, meta := range metas {
		bios = append(bios, &api.BiosphereDesc{
			Name:     &meta.Name,
			NumCores: &nCores,
			NumTicks: &nTicks,
		})
	}
	return &api.BiospheresS{
		Biospheres: bios,
	}, nil
}

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
	_, err = client.Put(ctx, key, &BiosphereMeta{
		Name: q.GetDesc().GetName(),
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
				Name:     &name,
				NumCores: &nCores,
				NumTicks: &nTicks,
			},
		},
	}, nil
}

func (fe *FeServiceImpl) prepare(service *compute.Service) {
	const name = "chunk-server-0"
	const zone = "us-central1-b"
	const machineType = "n1-standard-4"
	const projectId = "bonsai-genesis"

	prefix := "https://www.googleapis.com/compute/v1/projects/" + projectId
	imageURL := "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-1504-vivid-v20150422"

	startupScript := strings.Join(
		[]string{
			`#!/bin/bash`,
			`apt-get update`,
			`apt-get -y install docker.io`,
			`service docker start`,
			`METADATA=http://metadata.google.internal./computeMetadata/v1`,
			`SVC_ACCT=$METADATA/instance/service-accounts/default`,
			`ACCESS_TOKEN=$(curl -H 'Metadata-Flavor: Google' $SVC_ACCT/token | cut -d'"' -f 4)`,
			`docker login -e dummy@example.com -u _token -p $ACCESS_TOKEN https://gcr.io`,
			fmt.Sprintf(`docker pull %s`, WorkerContainerName),
			fmt.Sprintf(`docker run --publish 8000:8000 %s`, WorkerContainerName),
		}, "\n")

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
			},
		},
	}

	op, err := service.Instances.Insert(projectId, zone, instance).Do()
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
