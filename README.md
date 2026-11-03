# dns-go

## Project layout

- `cmd/dns-go`: executable entrypoint
- `internal/dnsserver`: DNS handler, record definitions, and Redis storage
- `internal/admin`: authenticated record-management and health HTTP API
- `web`: React/TypeScript admin dashboard
- `tests`: external behavior and Redis-backed handler tests

## Redis record storage

DNS answers are read from Redis using keys in the form
`dns:record:<normalized-name>:<type>`, for example
`dns:record:google.com.:A`. Values are JSON-encoded `Record` objects:

```json
{"Name":"google.com.","Type":1,"TTL":60,"Value":"127.0.0.2"}
```

On startup, the built-in records are inserted only when their keys do not
already exist, so Redis remains the source of truth. Redis read failures are
returned to DNS clients as `SERVFAIL`; they do not terminate the server.

## Running

Create a `.env` file in the repository root with the admin token:

```text
DNS_ADMIN_TOKEN=change-me
```

Then start the complete stack with Docker Compose:

```powershell
docker compose up --build
```

Docker Compose automatically loads variables from `.env`. The real `.env` file
is ignored by Git; use [`.env.example`](./.env.example) as a template.

The script starts the dashboard at `http://localhost:5173`, the admin API at
`http://127.0.0.1:8080`, and DNS on `127.0.0.1:8503`. Press `Ctrl+C` to stop
the Compose stack. Redis data is persisted in the `redis-data` Docker volume.

To stop the stack without following logs:

```powershell
docker compose down
```

The Compose file is [`compose.yaml`](./compose.yaml). The frontend is served
by Nginx and proxies the admin API and health endpoints to the Go container.

The server listens on both UDP and TCP at `127.0.0.1:8503` by default. Runtime
settings can be provided through environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `DNS_LISTEN_ADDR` | `127.0.0.1:8503` | UDP and TCP listen address |
| `DNS_REDIS_ADDR` | `localhost:6379` | Redis address |
| `DNS_REDIS_PASSWORD` | empty | Redis password |
| `DNS_REDIS_DB` | `0` | Redis database number |
| `DNS_DEFAULT_TTL` | `300` | TTL used when a record has TTL `0` |
| `DNS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `DNS_RECURSION_ENABLED` | `false` | Advertise recursion availability |
| `DNS_ALLOWED_CLIENTS` | `127.0.0.0/8,::1/128` | Comma-separated client CIDRs allowed to query |
| `DNS_RATE_LIMIT` | `50` | Sustained queries per second per client |
| `DNS_RATE_BURST` | `100` | Maximum per-client burst size |
| `DNS_MAX_RESPONSE_BYTES` | `1232` | Maximum DNS response size |
| `DNS_ZONE_FILE` | empty | Optional JSON file containing a `Record` array |
| `DNS_SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown timeout |
| `DNS_ADMIN_ENABLED` | `false` | Enable the authenticated admin HTTP API |
| `DNS_ADMIN_ADDR` | `127.0.0.1:8080` | Admin HTTP listen address |
| `DNS_ADMIN_TOKEN` | empty | Bearer token required by admin API write/read endpoints |

For example:

```text
DNS_LISTEN_ADDR=0.0.0.0:8503 DNS_REDIS_ADDR=redis:6379 go run ./cmd/dns-go
```

When binding beyond loopback, set `DNS_ALLOWED_CLIENTS` explicitly, for example
`DNS_ALLOWED_CLIENTS=10.0.0.0/8,192.0.2.0/24`. Records are validated before
being seeded, including names, address values, and structured record targets.
Malformed queries are rejected, recursion is disabled by default, and responses
are truncated to the configured maximum size to reduce amplification risk.

The server handles `SIGINT` and `SIGTERM`, stops accepting UDP and TCP DNS
requests within the configured timeout, and closes the Redis client before
exiting.

### Admin API

Enable the admin API with a token. It binds to localhost by default:

```text
DNS_ADMIN_ENABLED=true DNS_ADMIN_TOKEN=change-me go run ./cmd/dns-go
```

All record endpoints require `Authorization: Bearer <token>`. Health endpoints
are unauthenticated:

```text
GET    /healthz
GET    /readyz
GET    /api/v1/records
PUT    /api/v1/records?name=example.com.&type=A
DELETE /api/v1/records?name=example.com.&type=A
```

Create or update a record:

```text
curl -X PUT \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"ttl":60,"value":"192.0.2.10"}' \
  "http://127.0.0.1:8080/api/v1/records?name=example.com.&type=A"
```

The API reuses the DNS record validation rules. Updates replace existing
records, deletes maintain the internal name marker used to distinguish
NXDOMAIN from NODATA, and Redis readiness failures return HTTP 503.

### Admin dashboard

The admin dashboard is a Vite React/TypeScript app in [`web`](./web). Start
the Go server with the admin API enabled, then run:

```text
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. The Vite development server proxies `/api`,
`/healthz`, and `/readyz` to `http://127.0.0.1:8080`. Enter the
`DNS_ADMIN_TOKEN` value on the sign-in screen. The token is kept only in
browser session storage.

To create a production bundle:

```text
npm run build
```

Query it with:

```text
dig @127.0.0.1 -p 8503 go.dev A
```
Run the Go tests:

```text
go test ./...
```