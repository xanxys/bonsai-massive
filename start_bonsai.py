#!/bin/python3
import os
import shutil
import subprocess

if __name__ == '__main__':
    #requires docker 1.5+
    project_name = "bonsai-genesis"
    # You need to be in docker group to run this script

    def get_container_tag(container_name):
        return "gcr.io/%s/%s" % (project_name, container_name)

    print("Creating container")
    subprocess.call(["bazel", "build", "frontend:server"], cwd="./src")
    subprocess.call(["bazel", "build", "client"], cwd="./src")
    shutil.copyfile("src/bazel-bin/frontend/server.bin", "docker/frontend-server.bin")
    shutil.copyfile("src/bazel-bin/client/client.js", "docker/client.js")
    subprocess.call(["sudo", "docker", "build", "-t", get_container_tag('bonsai_frontend'), "-f", "docker/frontend", "./docker"])

    print("Pushing containers")
    subprocess.call(['gcloud', 'docker', 'push', get_container_tag('bonsai_frontend')])
