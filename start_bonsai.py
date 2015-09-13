#!/bin/python3
#
# requires docker 1.5+
# You need to be in docker group to run this script.
# This script is not supposed to be run by root.
import argparse
import datetime
import http.client
import http.server
import os
import random
import shutil
import subprocess

PROJECT_NAME = "bonsai-genesis"
FAKE_PORT = 7000
LOCAL_PORT = 8000

def get_container_tag(container_name):
    label = datetime.datetime.now().strftime('%Y%m%d-%H%M')
    return "gcr.io/%s/%s:%s" % (PROJECT_NAME, container_name, label)

def create_containers(container_name):
    print("Creating container %s" % container_name)
    # Without clean, bazel somehow won't update
    subprocess.call(["bazel", "clean"], cwd="./src")
    subprocess.call(["bazel", "build", "frontend:server"], cwd="./src")
    subprocess.call(["bazel", "build", "client:static"], cwd="./src")
    shutil.copyfile("src/bazel-out/local_linux-fastbuild/genfiles/frontend/server.bin", "docker/frontend-server.bin")
    shutil.rmtree("docker/static", ignore_errors=True)
    os.mkdir("docker/static")
    subprocess.call(["tar", "-xf", "src/bazel-out/local_linux-fastbuild/bin/client/static.tar", "-C", "docker/static"])
    subprocess.call(["docker", "build", "-t", container_name, "-f", "docker/frontend", "./docker"])

def deploy_containers_local(container_name):
    print("Running containers locally")
    name = "bonsai_fe-%d" % random.randint(0, 1000)
    subprocess.call([
        "docker", "run",
        "--tty",
        "--interactive",
        "--name", name,
        "--publish", "%d:8000" % LOCAL_PORT,
        container_name])

def deploy_containers_gke(container_name):
    print("Pushing containers to GC repository")
    subprocess.call(['gcloud', 'docker', 'push', container_name])

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
        else:
            print('PROXY')
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
    print("Running fake server at http://localhost:%d/ with fallback %s" % (
        FAKE_PORT, get_local_url()))

    httpd = http.server.HTTPServer(('localhost', FAKE_PORT), FakeServerHandler)
    httpd.serve_forever()

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
