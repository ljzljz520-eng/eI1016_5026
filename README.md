# Building Operations

Building Operations is a local facility console with session sign-in, account failure protection, audit history, and fixed elevator, air-conditioning, and access-control status fixtures.

## Requirements

- Go 1.23.12
- Node.js 20 for the optional frontend build
- `CGO_ENABLED=0`
- `GOTOOLCHAIN=local`

## Run

From the module root:

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 go run ./cmd/buildingops
```

Open `http://localhost:8080`. The local fixture credentials are `ops.admin` / `facility-123`. Set `PORT` to use another port.

## Verify

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 ./...
```

The business tests exercise anonymous access, sign-in, the authenticated dashboard APIs, audit records, sign-out, and account failure protection.

Build the frontend with Node.js 20:

```sh
cd web
npm install
npm run build
```

The server embeds `web/src`, so `web/dist` is a disposable build result and is not required at runtime.
