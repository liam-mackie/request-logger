# request-logger

A tiny Go service that accepts **any** HTTP request on **any** route and writes the
whole thing — method, path, query, every header, and the body — to stdout as JSON
via `log/slog`.

```json
{"time":"2026-09-16T12:20:20Z","level":"INFO","msg":"request","method":"POST",
 "host":"webhooks.gke.beans.sh","path":"/hook/test","proto":"HTTP/1.1",
 "remote_addr":"10.0.0.1:51139","query":"a=1","headers":{"Content-Type":"application/json",
 "X-Custom":["one","two"]},"body_bytes":17,"body":"{\"hello\":\"world\"}","duration_ns":19500}
```

Every request returns `200` with `{"logged":true}`.

## Behaviour

- **All routes, all methods.** There is no mux; one handler serves everything.
- **All headers**, with repeated headers preserved as an array.
- **Full body, never truncated.** Valid UTF-8 is logged as `body`; binary payloads
  are logged as base64 in `body_b64` with `body_encoding: "base64"`.
- Nothing is redacted — `Authorization` and `Cookie` headers are logged verbatim.
  Treat the log sink as sensitive.

## Config

| Env | Default | Purpose |
| --- | --- | --- |
| `ADDR` | `:8080` | Listen address |
| `RESPONSE_STATUS` | `200` | Status code to return |

## Run locally

```sh
go run .
curl -X POST localhost:8080/anything -d 'hi'
```

## Image

Built by GitHub Actions on every push to `main` and published multi-arch
(`linux/amd64`, `linux/arm64`) to `ghcr.io/liam-mackie/request-logger`.

## Deploy

```sh
kubectl apply -f deploy/
```

Exposed at `webhooks.gke.beans.sh`:

- **`:80`** via a Gateway API `HTTPRoute` attached to the shared
  `projectcontour/contour` Gateway.
- **`:443`** via a Contour `HTTPProxy` plus a cert-manager `Certificate`
  (`letsencrypt-prod`).

Two routing objects are needed because that Gateway's `:443` listener uses
Contour's mixed-mode `projectcontour.io/https` protocol, where TLS is configured
on the HTTPProxy/Ingress rather than on the listener. It reports
`supportedKinds: []`, so no `HTTPRoute` can attach to it for TLS.
