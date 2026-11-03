FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dns-go ./cmd/dns-go

FROM alpine:3.22
RUN adduser -D -H -u 10001 app
COPY --from=build /out/dns-go /usr/local/bin/dns-go
USER app
EXPOSE 8080 8503/tcp 8503/udp
ENTRYPOINT ["/usr/local/bin/dns-go"]
