package gbox

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"net/url"
	"strconv"
)

func (h *Handler) unmarshalCaddyfileCaching(d *caddyfile.Dispenser) error {
	var disabled bool
	caching := new(Caching)

	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "enabled":
				if !d.NextArg() {
					return d.ArgErr()
				}

				val, err := strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				disabled = !val
			case "store_dsn":
				if !d.NextArg() {
					return d.ArgErr()
				}

				_, err := url.Parse(d.Val())

				if err != nil {
					return err
				}

				caching.StoreDsn = d.Val()
			case "rules":
				if err := caching.unmarshalCaddyfileRules(d.NewFromNextSegment()); err != nil {
					return err
				}
			case "varies":
				if err := caching.unmarshalCaddyfileVaries(d.NewFromNextSegment()); err != nil {
					return err
				}
			case "type_keys":
				if err := caching.unmarshalCaddyfileTypeKeys(d.NewFromNextSegment()); err != nil {
					return err
				}
			case "auto_invalidate_cache":
				if !d.NextArg() {
					return d.ArgErr()
				}

				val, err := strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				caching.AutoInvalidate = val
			case "debug_headers":
				if !d.NextArg() {
					return d.ArgErr()
				}

				val, err := strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				caching.DebugHeaders = val
			default:
				return d.Errf("unrecognized subdirective %s", d.Val())
			}
		}
	}

	if !disabled {
		h.Caching = caching
	}

	return nil
}

func (c *Caching) unmarshalCaddyfileRules(d *caddyfile.Dispenser) error {
	rules := make(CachingRules)

	for d.Next() {
		for d.NextBlock(0) {
			rule := new(CachingRule)
			desc := d.Val()

			for subNesting := d.Nesting(); d.NextBlock(subNesting); {
				switch d.Val() {
				case "types":
					if err := rule.unmarshalCaddyfileTypes(d.NewFromNextSegment(), desc); err != nil {
						return err
					}
				case "max_age":
					if !d.NextArg() {
						return d.ArgErr()
					}

					v, err := caddy.ParseDuration(d.Val())

					if err != nil {
						return err
					}

					rule.MaxAge = caddy.Duration(v)
				case "swr":
					if !d.NextArg() {
						return d.ArgErr()
					}

					v, err := caddy.ParseDuration(d.Val())

					if err != nil {
						return err
					}

					rule.Swr = caddy.Duration(v)
				case "varies":
					args := d.RemainingArgs()

					if len(args) == 0 {
						return d.ArgErr()
					}

					varies := make(map[string]struct{})

					for _, arg := range args {
						if _, exists := varies[arg]; exists {
							return d.Errf("duplicate vary: %s", arg)
						}

						varies[arg] = struct{}{}
					}

					rule.Varies = varies
				default:
					return d.Errf("unrecognized subdirective %s", d.Val())
				}
			}

			rules[desc] = rule
		}
	}

	c.Rules = rules

	return nil
}

func (r *CachingRule) unmarshalCaddyfileTypes(d *caddyfile.Dispenser, desc string) error {
	types := make(graphql.RequestTypes)

	for d.Next() {
		for d.NextBlock(0) {
			val := d.Val()

			if _, ok := types[val]; ok {
				return fmt.Errorf("%s already specific", d.Val())
			}

			fields := map[string]struct{}{}
			args := d.RemainingArgs()

			for _, arg := range args {
				fields[arg] = struct{}{}
			}

			types[val] = fields
		}
	}

	r.Types = types

	return nil
}

func (c *Caching) unmarshalCaddyfileVaries(d *caddyfile.Dispenser) error {
	varies := make(CachingVaries)

	for d.Next() {
		for d.NextBlock(0) {
			name := d.Val()
			vary := &CachingVary{
				Headers: make(map[string]struct{}),
				Cookies: make(map[string]struct{}),
			}

			for subNesting := d.Nesting(); d.NextBlock(subNesting); {
				switch d.Val() {
				case "headers":
					args := d.RemainingArgs()

					if len(args) == 0 {
						return d.ArgErr()
					}

					for _, arg := range args {
						if _, exists := vary.Headers[arg]; exists {
							return d.Errf("duplicate header: %s", arg)
						}

						vary.Headers[arg] = struct{}{}
					}
				case "cookies":
					args := d.RemainingArgs()

					if len(args) == 0 {
						return d.ArgErr()
					}

					for _, arg := range args {
						if _, exists := vary.Cookies[arg]; exists {
							return d.Errf("duplicate cookie: %s", arg)
						}

						vary.Cookies[arg] = struct{}{}
					}
				default:
					return d.Errf("unrecognized subdirective %s", d.Val())
				}
			}

			varies[name] = vary
		}
	}

	c.Varies = varies

	return nil
}

func (c *Caching) unmarshalCaddyfileTypeKeys(d *caddyfile.Dispenser) error {
	fields := make(map[string]struct{})
	typeKeys := make(graphql.RequestTypes)

	for d.Next() {
		for d.NextBlock(0) {
			typeName := d.Val()
			args := d.RemainingArgs()

			if len(args) == 0 {
				return d.ArgErr()
			}

			for _, field := range args {
				fields[field] = struct{}{}
			}

			typeKeys[typeName] = fields
		}
	}

	c.TypeKeys = typeKeys

	return nil
}
