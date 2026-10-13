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

The server listens on UDP `127.0.0.1:8503`. Query it with:

```text
dig @127.0.0.1 -p 8503 go.dev A
```

On Windows, run the included smoke test with:

```text
python run.py
```

Run the Go test suite:

```text
go test ./...
```