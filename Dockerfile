FROM caddy:2-alpine

COPY gbox /usr/bin/caddy
COPY Caddyfile.dist /etc/caddy/Caddyfile