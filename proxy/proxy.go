// Package proxy implements a high-performance HTTP reverse proxy using fasthttp.
// It provides efficient request forwarding with minimal overhead, connection pooling,
// and round-robin load balancing across multiple backend servers.
package proxy

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/davidalecrim/extreme/config"
	"github.com/valyala/fasthttp"
)

// Proxy represents a high-performance reverse proxy server.
// It handles incoming HTTP requests and forwards them to configured backend servers
// using connection pooling and load balancing for optimal performance.
type Proxy struct {
	config         *config.Config
	clients        map[string]*fasthttp.HostClient
	backends       []string
	currentBackend uint32
	server         *fasthttp.Server
	logger         *slog.Logger
}

// New creates and returns a new Proxy instance configured with the provided
// configuration and logger. It sets up the fasthttp client and server with
// optimized settings for high performance and minimal latency.
func New(cfg *config.Config, logger *slog.Logger) (*Proxy, error) {
	clients := make(map[string]*fasthttp.HostClient, len(cfg.Backends))

	for _, backend := range cfg.Backends {
		if !strings.HasPrefix(backend, "http://") && !strings.HasPrefix(backend, "https://") {
			return nil, fmt.Errorf("backend URL must start with http:// or https://: %s", backend)
		}

		u, err := url.Parse(backend)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL %s: %v", backend, err)
		}

		clients[backend] = &fasthttp.HostClient{
			Addr:                u.Host,
			IsTLS:               u.Scheme == "https",
			MaxConns:            cfg.ConnectionPool.MaxConnsPerHost,
			MaxIdleConnDuration: cfg.KeepAlive.BackendTimeout,
			ReadTimeout:         cfg.Server.ReadTimeout,
			WriteTimeout:        cfg.Server.WriteTimeout,

			NoDefaultUserAgentHeader:      true,
			DisablePathNormalizing:        true,
			DisableHeaderNamesNormalizing: true,
		}
	}

	p := &Proxy{
		config:   cfg,
		backends: cfg.Backends,
		clients:  clients,
		logger:   logger,
	}

	p.server = &fasthttp.Server{
		Handler:                       p.handleRequest,
		ReadTimeout:                   cfg.Server.ReadTimeout,
		WriteTimeout:                  cfg.Server.WriteTimeout,
		MaxConnsPerIP:                 cfg.Server.MaxConnections,
		MaxRequestsPerConn:            cfg.KeepAlive.MaxRequestsPerConn,
		DisableHeaderNamesNormalizing: true,
		ReduceMemoryUsage:             true,
		TCPKeepalive:                  cfg.KeepAlive.Enabled,
		TCPKeepalivePeriod:            cfg.KeepAlive.ClientTimeout,
		NoDefaultServerHeader:         true,
		NoDefaultContentType:          true,
		NoDefaultDate:                 true,
		DisablePreParseMultipartForm:  true,
		StreamRequestBody:             true,
		GetOnly:                       false,
	}

	return p, nil
}

func (p *Proxy) handleRequest(ctx *fasthttp.RequestCtx) {
	backend := p.getNextBackend()
	client := p.clients[backend]

	if p.config.Logging.Enabled {
		p.logger.Debug("forwarding request",
			"backend", backend,
			"method", string(ctx.Method()),
			"path", string(ctx.Path()),
		)
	}

	req := &ctx.Request
	resp := &ctx.Response

	backendURL, _ := url.Parse(backend)
	req.SetHost(backendURL.Host)

	if err := client.Do(req, resp); err != nil {
		if p.config.Logging.Enabled {
			p.logger.Error("error forwarding request",
				"error", err,
				"backend", backend,
				"request", map[string]any{
					"method": string(ctx.Method()),
					"path":   string(ctx.Path()),
				},
			)
		}
		ctx.Error("Proxy error", fasthttp.StatusBadGateway)
		return
	}
}

// getNextBackend uses an atomic counter for lock-free round-robin selection
func (p *Proxy) getNextBackend() string {
	// Fast modulo operation using bitwise AND when len is power of 2
	next := atomic.AddUint32(&p.currentBackend, 1)
	idx := int(next % uint32(len(p.backends)))
	return p.backends[idx]
}

// Start begins accepting incoming connections and forwarding requests
// to backend servers. It blocks until the server is shut down or encounters
// an error.
func (p *Proxy) Start() error {
	if p.config.Logging.Enabled {
		p.logger.Info("starting proxy server",
			"address", p.config.Server.ListenAddress,
			"backends", p.backends,
		)
	}
	return p.server.ListenAndServe(p.config.Server.ListenAddress)
}

// Shutdown gracefully stops the proxy server, allowing in-flight requests
// to complete before shutting down. It returns any error encountered during
// the shutdown process.
func (p *Proxy) Shutdown() error {
	if p.config.Logging.Enabled {
		p.logger.Info("shutting down proxy server")
	}
	return p.server.Shutdown()
}
