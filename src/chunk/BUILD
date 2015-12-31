# HACK: go complains if all files are not in a same directory.
genrule(
    name="server",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
        "chunk.go",
        "handle_chunk_summary.go",
        "handle_modify_chunk.go",
        "handle_spawn_chunk.go",
        "handle_status.go",
        "main.go",
        "service.go",
    ],
    outs=[
        "server.bin",
    ],
    local=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go build -o server.bin",
)
