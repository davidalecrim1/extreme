// Package proxy implements a high-performance HTTP reverse proxy using fasthttp.
// It provides efficient request forwarding with minimal overhead, connection pooling,
// and round-robin load balancing across multiple backend servers.
package proxy

import (
	"log/slog"
	"net"
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
	currentBackend uint32
	server         *fasthttp.Server
	logger         *slog.Logger
}

// New creates and returns a new Proxy instance configured with the provided
// configuration and logger. It sets up the fasthttp client and server with
// optimized settings for high performance and minimal latency.
func New(cfg *config.Config, logger *slog.Logger) (*Proxy, error) {
	clients := make(map[string]*fasthttp.HostClient, len(cfg.BackendSockets))

	for _, backend := range cfg.BackendSockets {
		client := &fasthttp.HostClient{
			Addr: backend,
			Dial: func(addr string) (net.Conn, error) {
				return net.DialTimeout("unix", addr, 5*time.Second)
			},
			MaxIdleConnDuration: cfg.Server.KeepAliveTimeout,
			ReadTimeout:         cfg.Server.ReadTimeout,
			WriteTimeout:        cfg.Server.WriteTimeout,

			NoDefaultUserAgentHeader:      true,
			DisablePathNormalizing:        true,
			DisableHeaderNamesNormalizing: true,
		}

		if cfg.PreWarm.Enabled {
			preWarmCount := cfg.PreWarm.RequestsPerBackend
			for range preWarmCount {
				go func() {
					// Create a dummy request to establish connection and keep it alive
					req := fasthttp.AcquireRequest()
					resp := fasthttp.AcquireResponse()

					req.SetRequestURI("/")
					req.SetHost(client.Addr) // dummy host because of unix sockets
					req.Header.SetMethod(fasthttp.MethodHead)

					if err := client.Do(req, resp); err != nil {
						logger.Warn("failed to pre-warm connection",
							"backend", backend,
							"error", err,
						)
					}

					fasthttp.ReleaseRequest(req)
					fasthttp.ReleaseResponse(resp)
				}()
			}
		}

		clients[backend] = client
	}

	p := &Proxy{
		config:  cfg,
		clients: clients,
		logger:  logger,
	}

	p.server = &fasthttp.Server{
		Handler: p.handleRequest,
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
	idx := int(next % uint32(len(p.config.BackendSockets)))
	return p.config.BackendSockets[idx]
}

// Start begins accepting incoming connections and forwarding requests
// to backend servers. It blocks until the server is shut down or encounters
// an error.
func (p *Proxy) Start() error {
	if p.config.Logging.Enabled {
		p.logger.Info("starting proxy server",
			"address", p.config.Server.ListenAddress,
			"backends", p.config.BackendSockets,
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
