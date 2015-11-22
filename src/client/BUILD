# Flatten all files at top level of the tar archive.
# https://stackoverflow.com/questions/4898056/how-to-create-flat-tar-archive
genrule(
    name = "static",
    srcs = [
        ":client",
    ],
    outs = ["static.tar"],
    cmd = "tar -c --xform s:^.*/:: $(SRCS) > $@",
    output_to_bindir = 1,
)

filegroup(
    name = "client",
    srcs = glob([
        "*.css",
        "*.html",
        "*.js",
        "card_background.png",
        "icons/*.png",
        "icons/*.svg",
        "third_party/*.js",
    ]),
)
