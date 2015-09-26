go_binary(
    name="server_not_working_now_do_not_use",
    srcs=[
        "main.go",
        "//proto:proto_go_src",
    ],
)

# HACK: go complains if all files are not in a same directory.
genrule(
    name="server",
    srcs=[
        "main.go",
        "service.go",
        "//proto:proto_go_src",
    ],
    outs=[
        "server.bin",
    ],
    local=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env GOPATH=/home/xyx/go go build -o server.bin",
)
