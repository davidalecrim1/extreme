// Package proxy implements a high-performance HTTP reverse proxy using fasthttp.
// It provides efficient request forwarding with minimal overhead, connection pooling,
// and round-robin load balancing across multiple backend servers.
package proxy

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	backendURLs    map[string]*url.URL
	currentBackend uint32
	server         *fasthttp.Server
	logger         *slog.Logger
}

// New creates and returns a new Proxy instance configured with the provided
// configuration and logger. It sets up the fasthttp client and server with
// optimized settings for high performance and minimal latency.
func New(cfg *config.Config, logger *slog.Logger) (*Proxy, error) {
	clients := make(map[string]*fasthttp.HostClient, len(cfg.Backends))
	backendURLs := make(map[string]*url.URL, len(cfg.Backends))

	for _, backend := range cfg.Backends {
		if !strings.HasPrefix(backend, "http://") && !strings.HasPrefix(backend, "https://") {
			return nil, fmt.Errorf("backend URL must start with http:// or https://: %s", backend)
		}

		u, err := url.Parse(backend)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL %s: %v", backend, err)
		}

		backendURLs[backend] = u

		client := &fasthttp.HostClient{
			Addr:                u.Host,
			IsTLS:               u.Scheme == "https",
			MaxConns:            cfg.ConnectionPool.MaxConnsPerHost,
			MaxIdleConnDuration: cfg.KeepAlive.BackendTimeout,
			ReadTimeout:         cfg.Server.ReadTimeout,
			WriteTimeout:        cfg.Server.WriteTimeout,

			NoDefaultUserAgentHeader:      true,
			DisablePathNormalizing:        true,
			DisableHeaderNamesNormalizing: true,

			MaxConnDuration: 90 * time.Second,
			ReadBufferSize:  4 * 1024,
			WriteBufferSize: 4 * 1024,

			Dial: (&fasthttp.TCPDialer{
				Concurrency:      cfg.Server.Concurrency,
				DNSCacheDuration: 1 * time.Hour,
			}).Dial,
		}

		if cfg.PreWarm.Enabled {
			preWarmCount := cfg.PreWarm.RequestsPerBackend
			var wg sync.WaitGroup
			wg.Add(preWarmCount)

			for range preWarmCount {
				go func() {
					defer wg.Done()
					// Create a dummy request to establish connection and keep it alive
					req := fasthttp.AcquireRequest()
					resp := fasthttp.AcquireResponse()

					req.SetHost(u.Host)
					req.Header.SetMethod(fasthttp.MethodHead)

					if err := client.Do(req, resp); err != nil {
						logger.Warn("failed to pre-warm connection",
							"backend", backend,
							"error", err,
							"request", req.String(),
						)
					}

					fasthttp.ReleaseRequest(req)
					fasthttp.ReleaseResponse(resp)
				}()
			}

			wg.Wait()

			if cfg.Logging.Enabled {
				logger.Info("pre-warmed connections",
					"backend", backend,
					"count", preWarmCount,
				)
			}
		}

		clients[backend] = client
	}

	p := &Proxy{
		config:      cfg,
		backends:    cfg.Backends,
		clients:     clients,
		backendURLs: backendURLs,
		logger:      logger,
	}

	p.server = &fasthttp.Server{
		Handler:                       p.handleRequest,
		ReadTimeout:                   cfg.Server.ReadTimeout,
		WriteTimeout:                  cfg.Server.WriteTimeout,
		MaxConnsPerIP:                 cfg.Server.MaxConnectionsPerIP,
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

		Concurrency:        cfg.Server.Concurrency,
		ReadBufferSize:     32 * 1024,
		WriteBufferSize:    32 * 1024,
		MaxRequestBodySize: 512 * 1024,
		DisableKeepalive:   false,
		IdleTimeout:        10 * time.Second,

		CloseOnShutdown:       true,
		SecureErrorLogMessage: true,
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

	backendURL := p.backendURLs[backend]
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
		ctx.SetStatusCode(fasthttp.StatusBadGateway)
		ctx.SetBodyString("Gateway Error")
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

	err := p.server.Shutdown()

	for backend, client := range p.clients {
		if p.config.Logging.Enabled {
			p.logger.Debug("closing idle connections for backend", "backend", backend)
		}
		client.CloseIdleConnections()
	}

	return err
}
