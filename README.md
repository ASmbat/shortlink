# shortlink

[![CI](https://github.com/ASmbat/shortlink/actions/workflows/ci.yml/badge.svg)](https://github.com/ASmbat/shortlink/actions/workflows/ci.yml)

A small URL shortener HTTP API written in Go with only the standard
library: `net/http` method-pattern routing, `crypto/rand` for codes, no
third-party dependencies.

## Why this exists

A compact example of how I structure a Go service: clean separation between
transport (`handler`), business logic (`service`), and persistence
(`store`), with the persistence layer defined as an interface so a
production backend (DynamoDB, Postgres, etc.) can be swapped in without
touching the handler or service code.

## Endpoints

| Method | Path       | Description                          |
|--------|------------|---------------------------------------|
| POST   | `/links`   | Create a short link                   |
| GET    | `/{code}`  | Redirect to the original URL          |
| GET    | `/links/{code}` | Link details and hit count (no redirect, not counted as a hit) |
| GET    | `/healthz` | Liveness check                        |

### Create a link

```
POST /links
Content-Type: application/json

{"long_url": "https://example.com/some/very/long/path", "ttl_seconds": 3600}
```

Response:

```json
{
  "code": "a1b2c3d",
  "long_url": "https://example.com/some/very/long/path",
  "created_at": "2026-09-14T12:00:00Z",
  "expires_at": "2026-09-14T13:00:00Z",
  "hits": 0
}
```

`ttl_seconds` is optional (0 to 10 years); omit it for a link that never expires (then
`expires_at` is left out of the response).

### Resolve a link

```
GET /a1b2c3d
```

Responds with a `302` redirect to the original URL, or `404` if the code
doesn't exist or has expired.

## Running locally

```bash
make run
# or
go run ./cmd/server
```

The server listens on `:8080` by default; override with `ADDR=:9090`.

## Running with Docker

```bash
docker build -t shortlink .
docker run -p 8080:8080 shortlink
```

## Tests

```bash
make test
```

Covers the service (URL validation, code format and uniqueness, collision
retry limit, expiry at the TTL boundary, cleanup of expired links, hit
counting), the in-memory store, the logging/recovery middleware, and the
HTTP handlers (status codes, redirect location, stats endpoint).

## Design notes / trade-offs

- **Storage**: ships with an in-memory store for simplicity. The `store.Store`
  interface is the seam for a durable backend (e.g. DynamoDB). Expiry rules
  live in the service, so every backend behaves the same.
- **Expiry and cleanup**: expired links return `404` right away, and a
  background job removes them every minute so they don't use memory forever.
- **Code generation**: 7 random base62 characters from `crypto/rand` (about
  3.5 trillion combinations), with rejection sampling so every character is
  equally likely, and a bounded retry on collision. Random codes need no
  coordination between instances, unlike a counter. Codes that would shadow
  other routes (`healthz`, `links`) are never issued.
- **Hit counting** is best-effort and non-blocking: a failure to record a
  hit never breaks the redirect itself.
- **No auth/rate-limiting** included — out of scope for this example, but
  would sit as additional middleware in `internal/middleware` alongside the
  existing logging/recovery middleware.

## License

MIT — see [LICENSE](LICENSE).
