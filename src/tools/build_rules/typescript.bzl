# Copied from https://github.com/zmxv/bazel-custom-rules
ts_filetype = FileType([".ts"])

def get_transitive_files(ctx):
  s = set()
  for dep in ctx.attr.deps:
    s += dep.transitive_files
  s += ctx.files.srcs
  return s

def ts_library_impl(ctx):
  return struct(
      files=set(),
      transitive_files=get_transitive_files(ctx))

def ts_binary_impl(ctx):
  files = list(get_transitive_files(ctx))
  output = ctx.outputs.out
  flags = ' '.join(ctx.attr.flags)
  ctx.action(
      inputs=files,
      outputs=[output],
      command="tsc %s --out %s %s" % (
          flags, output.path, ' '.join([f.path for f in files])))

ts_library = rule(
  implementation = ts_library_impl,
  attrs = {
      "srcs": attr.label_list(allow_files=ts_filetype),
      "deps": attr.label_list(allow_files=False),
  },
)

ts_binary = rule(
  implementation = ts_binary_impl,
  attrs = {
      "srcs": attr.label_list(allow_files=ts_filetype),
      "deps": attr.label_list(allow_files=False),
      "flags": attr.string_list(),
  },
  outputs = {"out": "%{name}.js"},
)
