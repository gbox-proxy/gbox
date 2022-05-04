<h1 align="center"><img width="220px" src="https://gbox-proxy.github.io/img/gbox-full.png" /></h1>

Fast :zap: reverse proxy in front of any GraphQL server for caching, securing and monitoring.

[![Unit Tests](https://github.com/gbox-proxy/gbox/actions/workflows/ci.yml/badge.svg)](https://github.com/gbox-proxy/gbox/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gbox-proxy/gbox/branch/main/graph/badge.svg?token=U5DIBIY1FG)](https://codecov.io/gh/gbox-proxy/gbox)
[![Go Report Card](https://goreportcard.com/badge/github.com/gbox-proxy/gbox)](https://goreportcard.com/report/github.com/gbox-proxy/gbox)
[![Go Reference](https://pkg.go.dev/badge/github.com/gbox-proxy/gbox.svg)](https://pkg.go.dev/github.com/gbox-proxy/gbox)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/gbox)](https://artifacthub.io/packages/search?repo=gbox)

Features
--------

+ :floppy_disk: Caching
  + [RFC7234](https://httpwg.org/specs/rfc7234.html) compliant HTTP Cache.
  + Cache query operations results through types.
  + Auto invalidate cache through mutation operations.
  + [Swr](https://web.dev/stale-while-revalidate/) query results in background.
  + Cache query results to specific headers, cookies (varies).
+ :closed_lock_with_key: Securing
  + Disable introspection.
  + Limit operations depth, nodes and complexity.
+ :chart_with_upwards_trend: Monitoring ([Prometheus](https://prometheus.io/) metrics)
  + Operations in flight.
  + Operations count.
  + Operations request durations.
  + Operations caching statuses.

How it works
------------

Every single request sent by your clients will serve by GBox. The GBox reverse proxy will cache, validate, and collect metrics before pass through requests to your GraphQL server.

![Diagram](https://gbox-proxy.github.io/img/diagram.png)

