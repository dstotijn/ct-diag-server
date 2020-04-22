# ct-diag-server

[![CircleCI](https://circleci.com/gh/dstotijn/ct-diag-server.svg?style=shield)](https://circleci.com/gh/dstotijn/ct-diag-server)
[![Coverage Status](https://coveralls.io/repos/github/dstotijn/ct-diag-server/badge.svg?branch=master)](https://coveralls.io/github/dstotijn/ct-diag-server?branch=master)
![GitHub](https://img.shields.io/github/license/dstotijn/ct-diag-server)
[![GoDoc](https://godoc.org/github.com/dstotijn/ct-diag-server?status.svg)](https://godoc.org/github.com/dstotijn/ct-diag-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/dstotijn/ct-diag-server)](https://goreportcard.com/report/github.com/dstotijn/ct-diag-server)

> **ct-diag-server** is an server written in Go for storing and retrieving
> Diagnosis Keys, as defined in Apple/Google's [draft specification](https://www.apple.com/covid19/contacttracing/)
> of its Contact Tracing Framework. It aims to respect the privacy of its users
> and store only the bare minimum of data needed for anonymous contact tracing.

In anticipation of the general release of Apple and Google's native APIs (planned
for May 2020), this application provides a bare bones implementation for the
framework's server component: a central repository for submitting Diagnosis Keys
after a positive test, and retrieving a collection of all previously submitted
Diagnosis Keys.

## Goals

- Privacy first: Don't store or log any personally identifiable information.
- Built for high availability:
  - Aims to have a small memory footprint.
  - Retrieval of diagnosis keyset as binary stream, easily cachable by CDN.
  - Ships with a Dockerfile, for easy deployment as a workload on a wide range
    of hosting platforms.
- Security: rely on Go's standard library where possible, minimal usage of vendor
  dependencies.
- Solid test coverage, for easy auditing and review.
- Ships with PostgreSQL adapter for storage of Diagnosis Keys, but can easily be
  forked for new adapters.
- Liberal license, easily forkable for other developers.

## Features

- HTTP server for storing and retrieving Diagnosis Keys. Uses
  binary streams for sending and receiving as little data as possible over the
  wire: 16 bytes per _Diagnosis Key_, 2 bytes per _Day Number_.
- PostgreSQL support for storage.

## TODO

- Design and implement spec for creation of short lived OTPs for health care
  professionals, to be used as a bearer token for device users to authenticate
  the `POST` request for uploading Diagnosis Keys.

## Status

The project is currently under active development.

## License

[MIT](LICENSE)

(c) 2020 David Stotijn
