package gbox

import (
	"strconv"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func (h *Handler) unmarshalCaddyfileComplexity(d *caddyfile.Dispenser) error {
	var disabled bool
	complexity := new(Complexity)

	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "enabled":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseBool(d.Val())

				if err != nil {
					return err
				}

				disabled = !v
			case "max_depth":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseInt(d.Val(), 10, 32)

				if err != nil {
					return err
				}

				complexity.MaxDepth = int(v)
			case "node_count_limit":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseInt(d.Val(), 10, 32)

				if err != nil {
					return err
				}

				complexity.NodeCountLimit = int(v)
			case "max_complexity":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseInt(d.Val(), 10, 32)

				if err != nil {
					return err
				}

				complexity.MaxComplexity = int(v)
			default:
				return d.Errf("unrecognized subdirective %s", d.Val())
			}
		}
	}

	if !disabled {
		h.Complexity = complexity
	}

	return nil
}
