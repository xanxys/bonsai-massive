# Go library shared by both frontend and chunk.
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "lib",
    srcs = [
        "auth.go",
        "constants.go",
        "ds_schema.go",
        "timing_trace.go",
        "vector.go",
    ],
)

sh_test(
    name="common_test",
    size="small",
    srcs=["run_common_test.sh"],
    data=["common_test.bin"],
    deps=[":common_test_files"],
)

genrule(
    name="common_test_files",
    srcs=[
        "//common:lib",
        "//proto:proto_go_src",
    ] + glob(["*_test.go"]),
    outs=[
        "common_test.bin",
        "run_common_test.sh",
    ],
    local=1,
    testonly=1,
    cmd="mkdir -p $(@D)/api; cp -f $(SRCS) $(@D); mv $(@D)/*.pb.go $(@D)/api; cd $(@D); env CGO_ENABLED=0 GOPATH=/home/xyx/go go test -c -o common_test.bin; echo 'common/common_test.bin' > run_common_test.sh; chmod a+x run_common_test.sh",
)
