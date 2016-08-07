package(default_visibility = ["//visibility:public"])

genrule(
    name = "proto_go_src",
    srcs = [
        "chunk.proto",
        "common.proto",
        "frontend.proto",
    ],
    outs = ["chunk.pb.go", "common.pb.go", "frontend.pb.go"],
    local=1,
    cmd = "env PATH=$$PATH:/home/xyx/go/bin:/home/xyx/protobuf/bin/ protoc --go_out=plugins=grpc:$$(dirname $(@D)) $(SRCS)",
)

# This rule is necessary to use https://github.com/dcodeIO/protobuf.js,
# which requires .proto in runtime.
filegroup(
    name = "files",
    srcs = glob([
        "*.proto",
    ]),
)
