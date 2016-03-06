genrule(
    name="envgen",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
    ] + glob(["*.go"], exclude=["*_test.go"]),
    outs=[
        "envgen.bin",
    ],
    local=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go build -o envgen.bin",
)
