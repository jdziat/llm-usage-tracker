# llmut Desktop

This is a Wails v3 desktop shell for the core `llmut` engine. It is a separate Go module so Wails dependencies stay isolated from the stdlib-only root module.

## Run Locally

```sh
cd desktop
task dev
```

If `task` is not installed, the bundled Wails task runner works too:

```sh
cd desktop
wails3 task dev
```

Live updates are delivered through the Wails v3 runtime event bus. They only run inside `wails3 dev` or a built desktop app; the browser-only mock frontend intentionally skips the subscription because the Wails environment is absent.

## Build Locally

```sh
cd desktop
task build
```

Equivalent manual commands:

```sh
cd desktop/frontend
npm install
npm run build

cd ..
wails3 build
```

For plain Go checks, build the frontend first because `main.go` embeds `frontend/dist`:

```sh
cd desktop/frontend
npm install
npm run build

cd ..
PKG_CONFIG=/usr/bin/pkg-config go build ./...
PKG_CONFIG=/usr/bin/pkg-config go test ./...
```

`PKG_CONFIG=/usr/bin/pkg-config` is only needed when a conda environment shadows `pkg-config`; conda's shim may not search the system GTK4/WebKitGTK 6 directories.

The desktop module resolves the root package through the committed `replace github.com/jdziat/llm-usage-tracker => ../` directive in `desktop/go.mod`. Do not create a `go.work`; it would defeat the dependency isolation.

## Releases & code signing

Tagging a `v*` release builds desktop binaries for Linux, macOS, and Windows via `.github/workflows/desktop-release.yml` (separate from the CLI's release track; assets are named `llmut-desktop-<os>-<arch>` so they never collide with the CLI archives).

**The macOS and Windows builds ship unsigned for now.** Code signing is deferred until certificates are available:

- **macOS** needs an Apple Developer ID certificate plus notarization.
- **Windows** needs an Authenticode certificate and its CI secret.

Until those certs exist, first-run on macOS/Windows may hit Gatekeeper/SmartScreen. Linux needs no signing.
