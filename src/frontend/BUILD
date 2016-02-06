# HACK: go complains if all files are not in a same directory.
genrule(
    name="server",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
    ] + glob(["*.go"]),
    outs=[
        "server.bin",
    ],
    local=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go build -o server.bin",
)

sh_test(
    name="server_test",
    size="small",
    srcs=["run_server_test.sh"],
    data=["server_test.bin"],
    deps=[":server_test_files"],
)

genrule(
    name="server_test_files",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
    ] + glob(["*.go"]),
    outs=[
        "server_test.bin",
        "run_server_test.sh",
    ],
    local=1,
    testonly=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go test -c -o server_test.bin; echo 'frontend/server_test.bin' > run_server_test.sh; chmod a+x run_server_test.sh",
)
