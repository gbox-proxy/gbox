GBox
==========================

Fast :zap: reverse proxy in front of any GraphQL server for caching, securing and monitoring.

[![Unit Tests](https://github.com/gbox-proxy/gbox/actions/workflows/ci.yml/badge.svg)](https://github.com/gbox-proxy/gbox/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gbox-proxy/gbox/branch/main/graph/badge.svg?token=U5DIBIY1FG)](https://codecov.io/gh/gbox-proxy/gbox)
[![Go Report Card](https://goreportcard.com/badge/github.com/gbox-proxy/gbox)](https://goreportcard.com/report/github.com/gbox-proxy/gbox)

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
  