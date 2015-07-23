#!/bin/python3
import os
import shutil
import subprocess

if __name__ == '__main__':
    print("Creating container")
    subprocess.call(["bazel", "build", "frontend"], cwd="./src")
    subprocess.call(["bazel", "build", "client"], cwd="./src")
    shutil.copyfile("src/bazel-bin/frontend/server.bin", "docker/frontend-server.bin")
    shutil.copyfile("src/bazel-bin/client/client.js", "docker/client.js")
    subprocess.call(["sudo", "docker", "build", "-t", "docker.io/xanxys/bonsai-frontend", "-f", "docker/frontend", "./docker"])
