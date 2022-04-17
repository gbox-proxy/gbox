// Copied from https://github.com/caddyserver/xcaddy/blob/b7fd102f41e12be4735dc77b0391823989812ce8/environment.go#L251
package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/gbox-proxy/gbox"
)

func main() {
	caddycmd.Main()
}
