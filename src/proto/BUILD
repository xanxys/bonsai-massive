package(default_visibility = ["//visibility:public"])

genrule(
    name="proto_go_lib",
    srcs=[
        "frontend.pb.go",
    ],
    outs=[
        "proto_go.a",
    ],
    # TODO: Remove dependency to external hardcoded lib path.
    local=1,
    cmd="echo $(OUTS); env GOPATH=/home/xyx/go go build -o $(OUTS) $(SRCS)",
)

genrule(
    name = "proto_go_src",
    srcs = [
        "chunk.proto",
        "frontend.proto",
    ],
    outs = ["chunk.pb.go", "frontend.pb.go"],
    local=1,
    cmd = "protoc --go_out=plugins=grpc:$$(dirname $(@D)) $(SRCS)",
)

# This rule is necessary to use https://github.com/dcodeIO/protobuf.js,
# which requires .proto in runtime.
filegroup(
    name = "files",
    srcs = glob([
        "*.proto",
    ]),
)
