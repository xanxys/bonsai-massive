#!/bin/python3
#
# requires docker 1.5+
# You need to be in docker group to run this script.
# This script is not supposed to be run by root.
import argparse
import os
import random
import shutil
import subprocess

project_name = "bonsai-genesis"

def get_container_tag(container_name):
    return "gcr.io/%s/%s" % (project_name, container_name)

def create_containers():
    print("Creating container")
    subprocess.call(["bazel", "build", "frontend:server"], cwd="./src")
    subprocess.call(["bazel", "build", "client"], cwd="./src")
    shutil.copyfile("src/bazel-bin/frontend/server.bin", "docker/frontend-server.bin")
    shutil.copyfile("src/bazel-bin/client/client.js", "docker/client.js")
    subprocess.call(["docker", "build", "-t", get_container_tag('bonsai_frontend'), "-f", "docker/frontend", "./docker"])

def deploy_containers_local():
    print("Running containers locally")
    name = "bonsai_fe-%d" % random.randint(0, 1000)
    subprocess.call([
        "docker", "run",
        "--tty",
        "--interactive",
        "--name", name,
        "--publish", "8000:8000",
        get_container_tag('bonsai_frontend')])

def deploy_containers_gke():
    print("Pushing containers")
    subprocess.call(['gcloud', 'docker', 'push', get_container_tag('bonsai_frontend')])

if __name__ == '__main__':
    parser = argparse.ArgumentParser("Launch bonsai service")
    parser.add_argument('--local', default=False, action='store_const',
        const=True, help="Run locally (default: run remotely)")
    args = parser.parse_args()

    create_containers()
    if args.local:
        deploy_containers_local()
    else:
        deploy_containers_gke()
