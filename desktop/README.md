# llmut Desktop

This is a Wails v2 desktop shell for the core `llmut` engine. It is a separate Go module so Wails dependencies stay isolated from the stdlib-only root module.

## Run Locally

```sh
cd desktop
wails dev
```

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
