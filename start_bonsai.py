#!/bin/python3
"""
Build and/or deploy containers for bonsai.

           HTTP              gRPC
| Client | ---- | Frontend | ----- | Chunk |


frontend can be 2, 3, 1+2, or 1+3:
1. fake server (which proxies all API requests to more serious frontend)
2. local server (directly running docker container)
3. remote service (on GKE, managed by kubernetes under replication controller / load balancer)

chunk always runs on GCE (not GKE), launched by frontend.

Requires docker 1.5+
You need to be in docker group to run this script.
This script is not supposed to be run by root.
"""
import argparse
import datetime
import json
import http.client
import http.server
import os
import random
import shutil
import subprocess

# Google Cloud Platform project id
PROJECT_NAME = "bonsai-genesis"
FAKE_PORT = 7000
LOCAL_PORT = 8000


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
        chunk_container_name = self.get_container_path('bonsai_chunk')
        shutil.copyfile(self.path_key, "docker/key.json")
        shutil.copyfile("/etc/ssl/certs/ca-bundle.crt", "docker/ca-bundle.crt")
        open('docker/config.chunk-container', 'w').write(chunk_container_name)

    def _create_container(self, dockerfile_path, container_path, internal_f):
        print("Creating container %s" % container_path)
        internal_f()
        if subprocess.call(["docker", "build", "-t", container_path, "-f", dockerfile_path, "./docker"]) != 0:
            raise RuntimeError("Container build failed")
        return container_path

    def create_fe_container(self):
        """
        Create a docker container using docker/frontend Dockerfile.
        """
        def internal():
            if subprocess.call(["bazel", "build", "frontend:server"], cwd="./src") != 0:
                raise RuntimeError("Frontend build failed")
            if subprocess.call(["bazel", "build", "client:static"], cwd="./src") != 0:
                raise RuntimeError("Client build failed")
            shutil.copyfile("src/bazel-out/local_linux-fastbuild/genfiles/frontend/server.bin", "docker/frontend-server.bin")
            shutil.rmtree("docker/static", ignore_errors=True)
            os.mkdir("docker/static")
            subprocess.call(["tar", "-xf", "src/bazel-out/local_linux-fastbuild/bin/client/static.tar", "-C", "docker/static"])

        return self._create_container(
            'docker/frontend', self.get_container_path('bonsai_frontend'), internal)

    def create_chunk_container(self):
        """
        Create a docker container of the given name with docker/chunk file.
        """
        def internal():
            if subprocess.call(["bazel", "build", "chunk:server"], cwd="./src") != 0:
                raise RuntimeError("Chunk build failed")
            shutil.copyfile("src/bazel-out/local_linux-fastbuild/genfiles/chunk/server.bin", "docker/chunk-server.bin")

        return self._create_container(
            'docker/chunk', self.get_container_path('bonsai_chunk'), internal)


def deploy_containers_local(container_name):
    print("Running containers locally")
    name = "bonsai_fe-%d" % random.randint(0, 100000)
    subprocess.call([
        "docker", "run",
        "--tty",
        "--interactive",
        "--name", name,
        "--publish", "%d:8000" % LOCAL_PORT,
        container_name])

def deploy_containers_gke(container_name, rollout=True):
    print("Pushing containers to google container repository")
    subprocess.call(['gcloud', 'docker', 'push', container_name])

    if rollout:
        print("Rolling out new image %s" % container_name)
        subprocess.call(['kubectl', 'rolling-update',
            'dev-fe-rc',
            '--update-period=10s',
            '--image=%s' % container_name])

def get_local_url():
    return "http://localhost:%d/" % LOCAL_PORT

class FakeServerHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        base_dir = './src/client'
        path_components = self.path[1:].split('/')
        if path_components[0] == 'static':
            print('STATIC')
            # Convert to real file path
            if len(path_components) == 1:
                file_path_cs = ["index.html"]
            else:
                file_path_cs = path_components[1:]
            file_path = os.path.join(base_dir, *file_path_cs)
            self.do_get_static_with_fallback(file_path)
        elif path_components[0] == 'biosphere':
            print('STATIC (/biosphere -> /static/biosphere.html)')
            file_path = os.path.join(base_dir, 'biosphere.html')
            self.do_get_static_with_fallback(file_path)
        else:
            print('PROXY')
            self.do_get_proxy()

    def do_get_static_with_fallback(self, file_path):
        ctype = self.guess_type(file_path)
        try:
            f = open(file_path, 'rb')
            self.send_response(200)
            self.send_header('Content-Type', ctype)
            self.end_headers()
            shutil.copyfileobj(f, self.wfile)
            f.close()
        except IOError:
            print('->PROXY')
            self.do_get_proxy()

    def do_get_proxy(self):
        conn = http.client.HTTPConnection('localhost', LOCAL_PORT)
        conn.request('GET', 'http://localhost:%d%s' % (
            LOCAL_PORT, self.path))
        resp = conn.getresponse()

        self.send_response(resp.status)
        self.send_header('Content-Type', resp.getheader('Content-Type'))
        self.end_headers()
        shutil.copyfileobj(resp, self.wfile)
        conn.close()

def run_fake_server():
    host = '0.0.0.0'
    print("Running fake server at http://%s:%d/ with fallback %s" % (
        host, FAKE_PORT, get_local_url()))

    httpd = http.server.HTTPServer((host, FAKE_PORT), FakeServerHandler)
    httpd.serve_forever()

if __name__ == '__main__':
    parser = argparse.ArgumentParser("Launch bonsai service or fake server")
    # Modes
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
    # Key
    parser.add_argument("--key",
        help="""
        Path of JSON private key of Google Cloud Platform. This will be copied
        to the created container, so don't upload them to public repository.
        """)
    args = parser.parse_args()

    if args.fake:
        run_fake_server()

    if args.local or args.remote:
        factory = ContainerFactory(args.key)
        fe_container_name = factory.create_fe_container()
        chunk_container_name = factory.create_chunk_container()
        if args.local:
            deploy_containers_gke(chunk_container_name, rollout=False)
            deploy_containers_local(fe_container_name)
        elif args.remote:
            deploy_containers_gke(chunk_container_name, rollout=False)
            deploy_containers_gke(fe_container_name)
