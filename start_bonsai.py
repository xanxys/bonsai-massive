#!/bin/python3
"""
Build and/or deploy containers for bonsai.

           HTTP              gRPC
| Client | ---- | Frontend | ----- | Chunk |


frontend can be 2 or 1+2:
1. fake server (which proxies all API requests to more serious frontend)
2. remote service (on GKE, managed by kubernetes under replication controller / load balancer)

chunk always runs on GCE (not GKE), launched by frontend.

Requires docker 1.5+
You need to be in docker group to run this script.
This script is not supposed to be run by root.

It is assumed you use a single GKE cluster for all environments.
"""
import argparse
import datetime
import http.client
import io
import json
import os
import random
import shutil
import subprocess

# Google Cloud Platform project id
PROJECT_NAME = "bonsai-genesis"
FAKE_PORT = 7000
# Don't put trailing /
SERVERS = {
    'staging': ('bonsai-staging.xanxys.net', 80),
    'prod': ('bonsai.xanxys.net', 80),
}


class ContainerFactory:
    """
    A minutely tag is created on creation of ContainerFactory, and all
    containers created with the instance will carry the same tag.

    Note that this factory is tailored for containers in bosai, and not trying
    to be generic.
    """
    def __init__(self, path_key):
        try:
            json.load(open(path_key, "r"))
        except (IOError, KeyError) as exc:
            print("ERROR: JSON key at %s not found. Aborting container build. %s" %
                (path_key, exc))
            raise exc
        self.tag = datetime.datetime.now().strftime('%Y%m%d-%H%M')
        self.path_key = path_key
        self._setup_shared_context()

    def __del__(self):
        os.remove('docker/key.json')

    def get_container_path(self, container_name):
        return "gcr.io/%s/%s:%s" % (PROJECT_NAME, container_name, self.tag)

    def _setup_shared_context(self):
        chunk_container_name = self.get_container_path('bonsai_container')
        shutil.copyfile(self.path_key, "docker/key.json")
        shutil.copyfile("/etc/ssl/certs/ca-bundle.crt", "docker/ca-bundle.crt")

    def _create_container(self, dockerfile_path, container_path, internal_f):
        print("Creating container %s" % container_path)
        internal_f()
        if subprocess.call(["docker", "build", "-t", container_path, "-f", dockerfile_path, "./docker"]) != 0:
            raise RuntimeError("Container build failed")
        return container_path

    def create_container(self):
        """
        Create a docker container using docker/frontend Dockerfile.
        """
        def internal():
            if subprocess.call(["bazel", "build", "frontend:server"], cwd="./src") != 0:
                raise RuntimeError("Frontend build failed")
            if subprocess.call(["bazel", "build", "chunk:server"], cwd="./src") != 0:
                raise RuntimeError("Chunk build failed")
            if subprocess.call(["bazel", "build", "client:static"], cwd="./src") != 0:
                raise RuntimeError("Client build failed")
            shutil.copyfile("src/bazel-out/local-fastbuild/genfiles/frontend/server.bin", "docker/frontend-server.bin")
            shutil.copyfile("src/bazel-out/local-fastbuild/genfiles/chunk/server.bin", "docker/chunk-server.bin")
            shutil.rmtree("docker/static", ignore_errors=True)
            os.mkdir("docker/static")
            subprocess.call(["tar", "-xf", "src/bazel-out/local-fastbuild/bin/client/static.tar", "-C", "docker/static"])

        return self._create_container(
            'docker/container', self.get_container_path('bonsai_container'), internal)


def deploy_containers_gke(container_name):
    assert(args.env != 'prod')
    print("Pushing containers to google container repository")
    subprocess.call(['gcloud', 'docker', 'push', container_name])

    print("Rolling out new image (frontend) %s" % container_name)
    subprocess.call(['kubectl', 'rolling-update',
        'bonsai-%s-frontend-rc' % args.env,
        '--update-period=10s',
        '--image=%s' % container_name])

    print("Rolling out new image (chunk) %s" % container_name)
    subprocess.call(['kubectl', 'rolling-update',
        'bonsai-%s-chunk-rc' % args.env,
        '--update-period=10s',
        '--image=%s' % container_name])

def show_prod_deployment():
    pod_result = json.loads(subprocess.check_output(['kubectl', 'get', '-o', 'json', 'pod']).decode('utf-8'))
    def get_images_for(pod_name):
        container_statuses = [container_status for pod in pod_result['items'] for container_status in pod['status']['containerStatuses']]
        return set([cs['image'] for cs in container_statuses if cs['name'] == pod_name])

    staging_images = get_images_for('bonsai-staging-frontend')
    print("* current staging images: %s" % staging_images)
    print("* current prod images: %s" % get_images_for('bonsai-prod-frontend'))

    if len(staging_images) == 0:
        print("Staging pods not running; aborting")
    elif len(staging_images) >= 2:
        print("Multiple staging images %s exist; maybe doing rolling-update? Wait until it stabilizes and retry.")
    else:
        container_name_staging = staging_images.pop()
        print("===================================================================")
        print("kubectl rolling-update bonsai-prod-frontend-rc --update-period=30s --image=%s" % container_name_staging)
        print("===================================================================")


if __name__ == '__main__':
    parser = argparse.ArgumentParser("Launch bonsai service or fake server")
    # Modes
    parser.add_argument('--remote',
        default=False, action='store_const', const=True,
        help="""
        Create and run a container remotely on Google Container Engine.
        This option starts rolling-update immediately after uploading container.
        """)
    parser.add_argument('--env',
        default='staging',
        help="""
        Environment to deploy to. It must be either 'staging' (default) or 'prod'.
        """)
    # Key
    parser.add_argument("--key",
        help="""
        Path of JSON private key of Google Cloud Platform. This will be copied
        to the created container, so don't upload them to public repository.
        """)
    args = parser.parse_args() # this is a global variable.
    if args.env not in ['staging', 'prod']:
        raise RuntimeError("Invalid --env. It must be either staging or prod.")

    if args.remote:
        if args.env != 'prod':
            factory = ContainerFactory(args.key)
            container_name = factory.create_container()
            deploy_containers_gke(container_name)
        else:
            print("Direct deployment to prod is not supported, because it's dangerous.")
            print("Instead, here is the command to copy staging configuration to prod.")
            show_prod_deployment()
