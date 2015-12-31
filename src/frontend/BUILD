go_binary(
    name="server_not_working_now_do_not_use",
    srcs=[
        "//proto:proto_go_src",
        "main.go",
    ],
)

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
