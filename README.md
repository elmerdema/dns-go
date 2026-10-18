# dns-go

## Project layout

- `cmd/dns-go`: executable entrypoint
- `internal/dnsserver`: DNS handler, record definitions, and Redis storage
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

Start Redis locally:

```text
docker run --name redis-dev -p 6379:6379 -d redis:7
```

Run the DNS server:

```text
go run ./cmd/dns-go
```

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

Query it with:

```text
dig @127.0.0.1 -p 8503 go.dev A
```
Run the Go tests:

```text
go test ./...
```