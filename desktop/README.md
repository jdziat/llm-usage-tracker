# llmut Desktop

This is a Wails v2 desktop shell for the core `llmut` engine. It is a separate Go module so Wails dependencies stay isolated from the stdlib-only root module.

## Run Locally

```sh
cd desktop
wails dev
```

Live updates are delivered through the Wails runtime event bus. They only run
inside `wails dev` or a built desktop app; the browser-only mock frontend
intentionally skips the subscription because `window.runtime` is absent.

## Build Locally

```sh
cd desktop
wails build
```

For plain Go checks, build the frontend first because `main.go` embeds `frontend/dist`:

```sh
cd desktop/frontend
npm install
npm run build

cd ..
go build ./...
```

The desktop module resolves the root package through the committed `replace github.com/jdziat/llm-usage-tracker => ../` directive in `desktop/go.mod`. Do not create a `go.work`; it would defeat the dependency isolation.

## Releases & code signing

Tagging a `v*` release builds desktop bundles for Linux, macOS, and Windows via
`.github/workflows/desktop-release.yml` (separate from the CLI's release track;
assets are named `llmut-desktop-<os>-<arch>` so they never collide with the CLI
archives).

**The macOS and Windows bundles ship unsigned for now.** Code signing is the one
deferred item that depends on external procurement, not engineering:

- **macOS** needs an Apple Developer ID certificate plus notarization
  (`codesign` → `notarytool` → `xcrun stapler`), driven from CI secrets
  (`APPLE_DEVELOPER_ID`, an app-specific password or App Store Connect API key).
- **Windows** needs an Authenticode (OV/EV) certificate and its CI secret.

Until those certs exist, first-run on macOS/Windows hits Gatekeeper/SmartScreen:

- **macOS:** right-click the app → **Open** (or `xattr -d com.apple.quarantine /path/to/llmut.app`).
- **Windows:** **More info → Run anyway** on the SmartScreen prompt.
- **Linux:** the AppImage/`.deb`/binary need no signing and are the recommended
  first-class download until the macOS/Windows certs land.

When the certificates are available, add the signing/notarization steps to
`desktop-release.yml` (after `wails build`) and flip the bundles to signed — no
application code changes are required.
