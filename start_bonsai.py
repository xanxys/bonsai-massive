#!/bin/python3
#
# requires docker 1.5+
# You need to be in docker group to run this script.
# This script is not supposed to be run by root.
import argparse
import datetime
import os
import random
import shutil
import subprocess

project_name = "bonsai-genesis"

def get_container_tag(container_name):
    label = datetime.datetime.now().strftime('%Y%m%d-%H%M')
    return "gcr.io/%s/%s:%s" % (project_name, container_name, label)

def create_containers(container_name):
    print("Creating container %s" % container_name)
    # Without clean, bazel somehow won't update
    subprocess.call(["bazel", "clean"], cwd="./src")
    subprocess.call(["bazel", "build", "frontend:server"], cwd="./src")
    subprocess.call(["bazel", "build", "client:static"], cwd="./src")
    shutil.copyfile("src/bazel-bin/frontend/server.bin", "docker/frontend-server.bin")
    shutil.rmtree("docker/static", ignore_errors=True)
    os.mkdir("docker/static")
    subprocess.call(["tar", "-xf", "src/bazel-bin/client/static.tar", "-C", "docker/static"])
    subprocess.call(["docker", "build", "-t", container_name, "-f", "docker/frontend", "./docker"])

def deploy_containers_local(container_name):
    print("Running containers locally")
    name = "bonsai_fe-%d" % random.randint(0, 1000)
    subprocess.call([
        "docker", "run",
        "--tty",
        "--interactive",
        "--name", name,
        "--publish", "8000:8000",
        container_name])

def deploy_containers_gke(container_name):
    print("Pushing containers to GC repository")
    subprocess.call(['gcloud', 'docker', 'push', container_name])

    print("Rolling out new image %s" % container_name)
    subprocess.call(['kubectl', 'rolling-update',
        'dev-fe-rc',
        '--update-period=10s',
        '--image=%s' % container_name])

def run_fake_server():
    fake_port = 7000
    fallback_url = "http://localhost:8000/"
    print("Running fake server at http://localhost:%d/ with fallback %s" % (
        fake_port, fallback_url))

if __name__ == '__main__':
    parser = argparse.ArgumentParser("Launch bonsai service or fake server")
    parser.add_argument("--fake",
        default=False, action='store_const', const=True,
        help="""
        Run very lightweight fake python server, which is useful for client dev.
        Files are served directly (no container) from working tree, and all
        unknown requests are redirected to localhost:8000, which can be launched
        by --local.
        """)
    parser.add_argument('--local',
        default=False, action='store_const', const=True,
        help="""
        Create and run a container locally. This is equivalent to running it on
        GCE (--remote)
        """)
    parser.add_argument('--remote',
        default=False, action='store_const', const=True,
        help="""
        Create and run a container remotely on Google Container Engine.
        This option starts rolling-update immediately after uploading container.
        """)
    args = parser.parse_args()

    container_name = get_container_tag('bonsai_frontend')
    if args.fake:
        run_fake_server()
    elif args.local:
        create_containers(container_name)
        deploy_containers_local(container_name)
    elif args.remote:
        create_containers(container_name)
        deploy_containers_gke(container_name)
    else:
        print("One of {--local, --remote, --fake} required. See --help for details.")
