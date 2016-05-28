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

sh_test(
    name="envgen_test",
    size="small",
    srcs=["run_envgen_test.sh"],
    data=["envgen_test.bin"],
    deps=[":envgen_test_files"],
)

genrule(
    name="envgen_test_files",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
    ] + glob(["*.go"]),
    outs=[
        "envgen_test.bin",
        "run_envgen_test.sh",
    ],
    local=1,
    testonly=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go test -c -o envgen_test.bin; echo 'envgen/envgen_test.bin' > run_envgen_test.sh; chmod a+x run_envgen_test.sh",
)
