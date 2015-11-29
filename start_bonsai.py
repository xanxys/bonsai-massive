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
import http.server
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

def deploy_containers_gke(container_name, rollout=True):
    assert(args.env != 'prod')
    print("Pushing containers to google container repository")
    subprocess.call(['gcloud', 'docker', 'push', container_name])

    if rollout:
        print("Rolling out new image %s" % container_name)
        subprocess.call(['kubectl', 'rolling-update',
            'bonsai-%s-frontend-rc' % args.env,
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


class FakeServerHandler(http.server.SimpleHTTPRequestHandler):
    """
    Fake server that serves static files (including special URLs such as "/debug")
    identically from working directory files as the real server, but proxies
    everything to a real server determined by args.env.
    """

    def do_GET(self):
        base_dir = './src/client'
        maybe_fn = self.get_filename_from_url()
        mapping = self.emulate_build()
        if maybe_fn == True:
            print('PROXYING')
            self.do_get_proxy()
        elif maybe_fn == False or maybe_fn not in mapping:
            print('STATIC:NOT_FOUND_LOCALLY %s -> %s' % (self.path, maybe_fn))
            self.send_error(404,
                message="""File not found (FakeServer). You actually don't have the file, or FakeServer is behaving differently from real servers.""")
        else:
            print('STATIC %s -> %s' % (self.path, maybe_fn))
            self.do_get_static_file(mapping[maybe_fn])

    def do_get_static_file(self, file_path):
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

    def get_filename_from_url(self):
        """
        Converts URL to filename.
        Returns either:
        1. filename (str)
        2. False (when path is definitely not found)
        3. True (when path might be found on real server)
        path needs to start with "/"
        """
        path_components = [c for c in self.path.split('/') if c]
        if len(path_components) == 0:
            return 'landing.html'
        elif len(path_components) >= 2 and path_components[0] == 'static':
            if len(path_components) == 2:
                return path_components[1]
            else:
                return False
        elif path_components == ['debug']:
            return 'debug.html'
        elif len(path_components) == 2 and path_components[0] == 'biosphere':
            return 'biosphere.html'
        else:
            return True

    def emulate_build(self):
        """
        Create mapping from filename to file path,
        simulating BUILD file.
        """
        filename_blacklist = set(['BUILD', '.gitignore'])
        mapping = {}
        for (dir_path, dir_names, file_names) in os.walk('src/client'):
            for file_name in file_names:
                if file_name in filename_blacklist:
                    continue
                mapping[file_name] = os.path.join(dir_path, file_name)
        return mapping

    def do_get_proxy(self):
        host, port = SERVERS[args.env]
        conn = http.client.HTTPConnection(host, port)
        conn.request('GET', 'http://%s:%d%s' % (
            host, port, self.path), None, {
                'Accept-Encoding': self.parse_headers().get('accept-encoding', '')
            })
        resp = conn.getresponse()

        self.send_response(resp.status)
        self.send_header('Content-Encoding', resp.getheader('Content-Encoding'))
        self.send_header('Content-Type', resp.getheader('Content-Type'))
        self.end_headers()
        shutil.copyfileobj(resp, self.wfile)
        conn.close()

    def parse_headers(self):
        """
        Return dict (lowercase header name -> header content string)
        """
        headers = {}
        for line in self.headers.as_string().split('\n'):
            line = line.strip()
            ix = line.find(':')
            if ix < 0:
                continue
            headers[line[:ix].lower()] = line[ix+1:].strip()
        return headers


def run_fake_server():
    fake_server_config = ('0.0.0.0', FAKE_PORT)
    fake_server_url = 'http://%s:%d/' % fake_server_config
    real_server_url = 'http://%s:%d/' % SERVERS[args.env]
    print("Running fake server at %s with fallback %s" % (fake_server_url, real_server_url))
    httpd = http.server.HTTPServer(fake_server_config, FakeServerHandler)
    httpd.serve_forever()

if __name__ == '__main__':
    parser = argparse.ArgumentParser("Launch bonsai service or fake server")
    # Modes
    parser.add_argument("--fake",
        default=False, action='store_const', const=True,
        help="""
        Run very lightweight fake python server which serves static files from
        working directory, and proxies other requests to real server on GKE.
        """)
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

    if args.fake:
        run_fake_server()

    if args.remote:
        if args.env != 'prod':
            factory = ContainerFactory(args.key)
            fe_container_name = factory.create_fe_container()
            chunk_container_name = factory.create_chunk_container()
            deploy_containers_gke(chunk_container_name, rollout=False)
            deploy_containers_gke(fe_container_name)
        else:
            print("Direct deployment to prod is not supported, because it's dangerous.")
            print("Instead, here is the command to copy staging configuration to prod.")
            show_prod_deployment()
