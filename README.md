# shortlink

A small, dependency-light URL shortener HTTP API written in Go, using only
the standard library's `net/http` (Go 1.22+ method-pattern routing) plus
`google/uuid` for code generation.

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

`ttl_seconds` is optional; omit it for a link that never expires.

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

Covers the service layer (validation, expiry, hit counting) and the HTTP
handlers (status codes, redirect location, error responses).

## Design notes / trade-offs

- **Storage**: ships with an in-memory store for simplicity. The `store.Store`
  interface is the seam for a durable backend — a DynamoDB implementation
  would satisfy the same three methods (`Save`, `Get`, `IncrementHits`).
- **Code generation**: uses a UUID-derived short code with a bounded retry
  loop on collision, rather than a counter, so it works without coordination
  across multiple instances.
- **Hit counting** is best-effort and non-blocking: a failure to record a
  hit never breaks the redirect itself.
- **No auth/rate-limiting** included — out of scope for this example, but
  would sit as additional middleware in `internal/middleware` alongside the
  existing logging/recovery middleware.
