def get_transitive_files(ctx):
    s = set()
    for dep in ctx.attr.deps:
        s += dep.transitive_files
    s += ctx.files.srcs
    return s

def go_binary_impl(ctx):
    files = list(get_transitive_files(ctx))
    output = ctx.outputs.out
    flags = ' '.join(ctx.attr.flags)
    ctx.action(
      inputs=files,
      outputs=[output],
      command="go build -o %s %s %s" % (
          output.path, flags, ' '.join([f.path for f in files])))

    ctx.file_action(
        output = ctx.outputs.executable,
        content = "\n".join([
          "#!/bin/bash -e",
          'set -o pipefail',
          'exec %s "$@"' % (output.short_path),
        ]),
        executable = True,
    )
    return struct(
        runfiles = ctx.runfiles(files = [ctx.outputs.executable, output])
    )

go_binary = rule(
  implementation = go_binary_impl,
  attrs = {
      "srcs": attr.label_list(allow_files=FileType([".go", ".a"])),
      "deps": attr.label_list(allow_files=False),
      "flags": attr.string_list(),
  },
  outputs = {"out": "%{name}.bin"},
  executable = True,
)
