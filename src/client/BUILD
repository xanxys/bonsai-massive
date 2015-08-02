# Flatten all files at top level of the tar archive.
# https://stackoverflow.com/questions/4898056/how-to-create-flat-tar-archive
genrule(
    name = "static",
    srcs = [
        ":client",
        "index.html",
    ],
    outs = ["static.tar"],
    cmd = "tar -c --xform s:^.*/:: $(SRCS) > $@",
    output_to_bindir = 1,
)

ts_binary(
    name = "client",
    srcs = [
        "main.ts",
    ],
)
