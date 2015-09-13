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
        "frontend.proto",
    ],
    outs = ["frontend.pb.go"],
    cmd = "protoc --go_out=$$(dirname $(@D)) $(SRCS)",
)
