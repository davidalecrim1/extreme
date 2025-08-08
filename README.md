# High-Performance Reverse Proxy

An ultra-high-performance reverse proxy written in Go, designed for maximum throughput and minimal latency. Uses `fasthttp` for superior performance over the standard `net/http` package.

## Features

- High-performance request forwarding using `fasthttp`
- Round-robin load balancing across multiple backends
- Connection pooling and keep-alive support
- Configurable timeouts and connection limits
- Minimal memory footprint
- Zero-copy forwarding of request/response bodies
- Graceful shutdown support
- YAML configuration with command-line override

## Configuration

Configuration is loaded from `config.yaml` by default. All settings can be customized:

```yaml
server:
  listen_address: ":8080"
  read_timeout: "30s"
  write_timeout: "30s"
  max_connections: 50000

keep_alive:
  enabled: true
  client_timeout: "60s"
  backend_timeout: "60s"
  max_requests_per_conn: 1000

backends:
  - "http://localhost:8081"
  - "http://localhost:8082"

connection_pool:
  max_idle_conns: 1000
  max_conns_per_host: 2000

logging:
  enabled: true
  level: "info"
```

## Usage

Build and run:

```bash
go build
./extreme -config config.yaml
```

The proxy will start and begin forwarding requests to the configured backends in a round-robin fashion.

## Performance Optimizations

- Uses `fasthttp` for maximum performance
- Zero-allocation request/response handling
- Efficient connection pooling
- Atomic counter for load balancing
- Minimal logging overhead (can be disabled)
- No request/response body parsing
- Direct byte forwarding

## Requirements

- Go 1.24 or later
- Backend servers must support HTTP/1.1 

## Versions

The current version that showned the best performance results for **[Rinha de Backend](https://github.com/davidalecrim1/rinha-with-go-2025)** is **v0.4.0**.

## License

[LICENSE](./LICENSE)