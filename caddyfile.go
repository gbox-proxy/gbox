package gbox

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
)

func init() { // nolint:gochecknoinits
	httpcaddyfile.RegisterHandlerDirective("gbox", parseCaddyfile)
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) { // nolint:ireturn
	m := new(Handler).CaddyModule().New().(*Handler)

	if err := m.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}

	if m.Upstream == "" {
		return nil, errors.New("upstream url must be set")
	}

	return m, nil
}

// nolint:funlen,gocyclo
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) (err error) {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "upstream":
				if h.Upstream != "" {
					return d.Err("upstream already specified")
				}

				if !d.NextArg() {
					return d.ArgErr()
				}

				val := d.Val()
				var u *url.URL
				var tokens []caddyfile.Token
				u, err = url.Parse(val)

				if err != nil {
					return err
				}

				r := &rewrite.Rewrite{URI: u.RequestURI()}
				rp := &reverseproxy.Handler{}
				rpPattern := `
reverse_proxy {
	to %s://%s 
	header_up Host {upstream_hostport}
}
`
				rpConfig := fmt.Sprintf(rpPattern, u.Scheme, u.Host)
				tokens, err = caddyfile.Tokenize([]byte(rpConfig), "")

				if err != nil {
					return err
				}

				err = rp.UnmarshalCaddyfile(caddyfile.NewDispenser(tokens))

				if err != nil {
					return err
				}

				// unmarshal again to add extra reverse proxy config
				err = rp.UnmarshalCaddyfile(d.NewFromNextSegment())

				if err != nil {
					return err
				}

				h.Upstream = val
				h.RewriteRaw = caddyconfig.JSONModuleObject(r, "rewrite", "rewrite", nil)
				h.ReverseProxyRaw = caddyconfig.JSONModuleObject(rp, "reverse_proxy", "reverse_proxy", nil)
			case "disabled_introspection":
				if !d.NextArg() {
					return d.ArgErr()
				}

				var disabled bool
				disabled, err = strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				h.DisabledIntrospection = disabled
			case "fetch_schema_interval":
				if !d.NextArg() {
					return d.ArgErr()
				}

				var dt time.Duration
				dt, err = caddy.ParseDuration(d.Val())

				if err != nil {
					return err
				}

				duration := caddy.Duration(dt)
				h.FetchSchemaInterval = &duration
			case "fetch_schema_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}

				var dt time.Duration
				dt, err = caddy.ParseDuration(d.Val())

				if err != nil {
					return err
				}

				duration := caddy.Duration(dt)
				h.FetchSchemaTimeout = &duration
			case "fetch_schema_header":
				if !d.NextArg() {
					return d.ArgErr()
				}

				name := d.Val()

				if !d.NextArg() {
					return d.ArgErr()
				}

				h.FetchSchemaHeader.Add(name, d.Val())
			case "complexity":
				if h.Complexity != nil {
					return d.Err("complexity already specified")
				}

				if err = h.unmarshalCaddyfileComplexity(d.NewFromNextSegment()); err != nil {
					return err
				}
			case "caching":
				if h.Caching != nil {
					return d.Err("caching already specified")
				}

				if err = h.unmarshalCaddyfileCaching(d.NewFromNextSegment()); err != nil {
					return err
				}
			case "disabled_playgrounds":
				if !d.NextArg() {
					return d.ArgErr()
				}

				var disabled bool
				disabled, err = strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				h.DisabledPlaygrounds = disabled
			case "cors_origins":
				origins := d.RemainingArgs()

				if len(origins) == 0 {
					return d.ArgErr()
				}

				h.CORSOrigins = origins
			case "cors_allowed_headers":
				headers := d.RemainingArgs()

				if len(headers) == 0 {
					return d.ArgErr()
				}

				h.CORSAllowedHeaders = headers
			default:
				return d.Errf("unrecognized subdirective %s", d.Val())
			}
		}
	}

	return err
}

// Interface guards.
var (
	_ caddyfile.Unmarshaler = (*Handler)(nil)
)
