package gbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"go.uber.org/zap"
)

const (
	errorReporterCtxKey caddy.CtxKey = "gbox_error_wrapper"
	nextHandlerCtxKey   caddy.CtxKey = "gbox_caddy_handler"
)

func init() { // nolint:gochecknoinits
	caddy.RegisterModule(Handler{})
}

// Handler implements an HTTP handler as a GraphQL reverse proxy server for caching, securing, and monitoring.
type Handler struct {
	// Rewrite
	RewriteRaw json.RawMessage `json:"rewrite_raw,omitempty" caddy:"namespace=http.handlers inline_key=rewrite"`

	// Reverse proxy
	ReverseProxyRaw json.RawMessage `json:"reverse_proxy,omitempty" caddy:"namespace=http.handlers inline_key=reverse_proxy"`

	// Upstream graphql server url
	Upstream string `json:"upstream,omitempty"`

	// Fetch schema interval, disabled by default.
	FetchSchemaInterval caddy.Duration `json:"fetch_schema_interval,omitempty"`

	// Fetch schema request timeout, "30s" by default
	FetchSchemaTimeout caddy.Duration `json:"fetch_schema_timeout,omitempty"`

	// Fetch schema headers
	FetchSchemaHeader http.Header `json:"fetch_schema_headers,omitempty"`

	// Whether to disable introspection request of downstream.
	DisabledIntrospection bool `json:"disabled_introspection,omitempty"`

	// Whether to disable playground paths.
	DisabledPlaygrounds bool `json:"disabled_playgrounds,omitempty"`

	// Request complexity settings, disabled by default.
	Complexity *Complexity `json:"complexity,omitempty"`

	// Caching queries result settings, disabled by default.
	Caching *Caching `json:"caching,omitempty"`

	// Cors origins
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Cors allowed headers
	CORSAllowedHeaders []string `json:"cors_allowed_headers,omitempty"`

	ReverseProxy        *reverseproxy.Handler `json:"-"`
	Rewrite             *rewrite.Rewrite      `json:"-"`
	ctxBackground       context.Context
	ctxBackgroundCancel func()
	logger              *zap.Logger
	schema              *graphql.Schema
	schemaDocument      *ast.Document
	router              http.Handler
	metrics             *Metrics
}

type errorReporter struct {
	error
}

func (h Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.gbox",
		New: func() caddy.Module {
			mh := new(Handler)
			mh.FetchSchemaHeader = make(http.Header)
			mh.ctxBackground, mh.ctxBackgroundCancel = context.WithCancel(context.Background())
			mh.schema = new(graphql.Schema)

			return mh
		},
	}
}

func (h *Handler) Provision(ctx caddy.Context) (err error) {
	h.metrics = metrics
	h.logger = ctx.Logger(h)
	h.initRouter()

	var m interface{}
	m, err = ctx.LoadModule(h, "ReverseProxyRaw")

	if err != nil {
		return fmt.Errorf("fail to load reverse proxy module: %w", err)
	}

	h.ReverseProxy = m.(*reverseproxy.Handler)
	m, err = ctx.LoadModule(h, "RewriteRaw")

	if err != nil {
		return fmt.Errorf("fail to load rewrite module: %w", err)
	}

	h.Rewrite = m.(*rewrite.Rewrite)

	if h.Caching != nil {
		if err = h.Caching.Provision(ctx); err != nil {
			return err
		}

		h.Caching.withLogger(h.logger)
		h.Caching.withMetrics(h)
	}

	if h.FetchSchemaTimeout == 0 {
		timeout, _ := caddy.ParseDuration("30s")
		h.FetchSchemaTimeout = caddy.Duration(timeout)
	}

	sf := &schemaFetcher{
		upstream:        h.Upstream,
		header:          h.FetchSchemaHeader,
		timeout:         h.FetchSchemaTimeout,
		interval:        h.FetchSchemaInterval,
		logger:          h.logger,
		context:         h.ctxBackground,
		onSchemaChanged: h.onSchemaChanged,
		caching:         h.Caching,
	}

	if err = sf.Provision(ctx); err != nil {
		h.logger.Error("fail to fetch upstream schema", zap.Error(err))
	}

	return err
}

func (h *Handler) Validate() error {
	if h.Caching != nil {
		if err := h.Caching.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) onSchemaChanged(oldSchemaDocument, newSchemaDocument *ast.Document, oldSchema, newSchema *graphql.Schema) {
	h.schema = newSchema
	h.schemaDocument = newSchemaDocument

	if h.Caching != nil && oldSchema != nil {
		h.logger.Info("schema changed: purge all query result cached of old schema")

		if err := h.Caching.PurgeQueryResultBySchema(h.ctxBackground, oldSchema); err != nil {
			h.logger.Error("purge all query result failed", zap.Error(err))
		}
	}
}

func (h *Handler) Cleanup() error {
	h.ctxBackgroundCancel()

	if h.Caching != nil {
		return h.Caching.Cleanup()
	}

	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, n caddyhttp.Handler) error {
	reporter := new(errorReporter)
	ctx := context.WithValue(r.Context(), nextHandlerCtxKey, n)
	ctx = context.WithValue(ctx, errorReporterCtxKey, reporter)

	h.router.ServeHTTP(w, r.WithContext(ctx))

	return reporter.error
}

// Interface guards.
var (
	_ caddy.Module                = (*Handler)(nil)
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddy.Validator             = (*Handler)(nil)
	_ caddy.CleanerUpper          = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)
