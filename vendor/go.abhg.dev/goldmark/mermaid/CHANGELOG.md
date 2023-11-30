# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html),
and is generated by [Changie](https://github.com/miniscruff/changie).

## v0.5.0 - 2023-11-03

This release brings support for rendering diagrams server-side
without the need for the MermaidJS CLI.

You can use this functionality by installing a `mermaidcdp.Compiler`
into your `mermaid.Extender` or `mermaid.ServerRenderer`.
For example:

```go
import "go.abhg.dev/goldmark/mermaid/mermaidcdp"

compiler, err := mermaidcdp.New(&mermaidcdp.Config{
  JSSource: mermaidJSSource, // contents of mermaid.min.js
})
if err != nil {
  return err
}
defer compiler.Close()

md := goldmark.New(
  goldmark.WithExtensions(
    // ...
    &mermaid.Extender{
      Compiler: compiler,
    },
  ),
  // ...
)
```

Use of mermaidcdp is highly recommended for server-side rendering
if you have lots of diagrams or documents to render.
This should be substantially faster than invoking the `mmdc` CLI.

### Breaking changes
- ServerRenderer: Delete `MMDC` and `Theme` fields.
  If you need these, you can provide them with the `CLICompiler` instead.
- `CLI` and `MMDC` were flipped.
  The old `MMDC` interface is now named `CLI`, and it now accepts a context.
  You can use the new `MMDC` function to build an instance of it.
- ClientRenderer, Extender: Rename `MermaidJS` to `MermaidURL`.
- Rename `DefaultMMDC` to `DefaultCLI`.
- Extender: Replace `MMDC` field with the `CLI` field.

### Added
- ServerRenderer now supports pluggable `Compiler`s.
- Add `CLICompiler` to render diagrams by invoking MermaidJS CLI. Plugs into ServerRenderer.
- Add mermaidcdp subpackage to render diagrams with a long-running Chromium-based process.
  Plugs into ServerRenderer.

## v0.4.0 - 2023-03-24
### Changed
- ClientRenderer: Use `<pre>` instead of `<div>` for diagram containers.

### Added
- Support changing the container tag with the `ContainerTag` option.
  This option is available on ClientRenderer, ServerRenderer, and Extender.

## v0.3.0 - 2022-12-19
### Changed
- Change the module path to `go.abhg.dev/goldmark/mermaid`.

### Removed
- Delete previously deprecated Renderer type.
  Please use the ClientRenderer instead.

## v0.2.0 - 2022-11-04
### Added
- ServerRenderer with support for rendering Mermaid diagrams
  into inline SVGs server-side.
  This is picked automatically if an 'mmdc' executable is found on PATH.
- Support opting out of the MermaidJS `<script>` tag.
  To use, set `Extender.NoScript` or `Transformer.NoScript` to true.
  Use this if the page you're rendering into already includes the tag
  elsewhere.

### Changed
- Deprecate Renderer in favor of ClientRenderer.
  Rendere has been aliased to the new type
  so existing code should continue to work unchanged.

## v0.1.1 - 2021-11-03
### Fixed

- Fix handling of multiple mermaid blocks.

## v0.1.0 - 2021-04-12
- Initial release.