# ct-diag-server

[![CircleCI](https://circleci.com/gh/dstotijn/ct-diag-server.svg?style=shield)](https://circleci.com/gh/dstotijn/ct-diag-server)
[![Coverage Status](https://coveralls.io/repos/github/dstotijn/ct-diag-server/badge.svg?branch=master)](https://coveralls.io/github/dstotijn/ct-diag-server?branch=master)
[![GitHub](https://img.shields.io/github/license/dstotijn/ct-diag-server)](LICENSE)
[![GoDoc](https://godoc.org/github.com/dstotijn/ct-diag-server?status.svg)](https://godoc.org/github.com/dstotijn/ct-diag-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/dstotijn/ct-diag-server)](https://goreportcard.com/report/github.com/dstotijn/ct-diag-server)

> **ct-diag-server** is a HTTP server written in Go for storing and retrieving
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
  - Retrieval of diagnosis keyset as a bytestream, easily cachable by CDN.
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
  bytestreams for sending and receiving as little data as possible over the
  wire: 16 bytes per _Diagnosis Key_, 2 bytes per _Day Number_.
- PostgreSQL support for storage.

## API reference

### Listing Diagnosis Keys

To be used for fetching a list of Diagnosis Keys. A typical client is either a mobile
device, or an intermediate platform/server of an app developer, for manual/custom
distribution of the payload to clients. In either case, the keyset can be
regarded as public; it doesn't contain PII.

#### Request

`GET /diagnosis-keys`

#### Response

A `200 OK` response should be expected, both for an empty and a non-empty reply.
In case of an empty reply, a `Content-Length: 0` header is written.
A `500 Internal Server Error` response indicates server failure, and warrants a retry.

#### Response headers

| Name                                     | Description                                                                  |
| ---------------------------------------- | ---------------------------------------------------------------------------- |
| `Content-Type: application/octet-stream` | The HTTP response is a byte stream (see below).                              |
| `Content-Length: {n * 18}`               | Content length is `n * 18`, where `n` is the amount of found Diagnosis Keys. |

#### Response body

The HTTP response body is a bytestream of Diagnosis Keys. A diagnosis key consists
of two parts: the key data itself (16 bytes), and 2 bytes (big endian) to denote
the _Day Number_; the day since Unix Epoch (see [this note](diag/diag.go#L20) on
why 2 bytes was chosen and what the consequence is). Because the amount of bytes
per diagnosis key is fixed, there is no delimiter.

### Uploading Diagnosis Keys

To be used for uploading a set of Diagnosis Keys by a mobile client device.

**Note:** There currently is no authentication, nor is there a means to obtain
and compute a bearer token. See [TODO](#todo).

#### Request

`POST /diagnosis-keys`

Any request headers (e.g. `Content-Length` and `Content-Type`) are not needed.

#### Body

The HTTP request body should be a bytestream of `1 <= 14` Diagnosis Keys.
A diagnosis key consists of two parts: the key data itself (16 bytes), and 2 bytes
(big endian) to denote the _Day Number_ (see above). Because the amount of bytes
per diagnosis key is fixed, there is no delimiter.

An unexpected end of the bytestream (e.g. incomplete key or day number) results
in a `400 Bad Request` response.

Duplicate keys are silently ignored.

#### Response

A `200 OK` response with body `OK` should be expected on storage of the keyset in the database.
A `400 Bad Request` response is used for client errors. A `500 Internal Server Error`
response is used for server errors, and warrants a retry. Error reasons are written
in a `text/plain; charset=utf-8` response body.

## TODO

- [ ] Implement privacy friendly authentication for uploading.
- [ ] Add benchmarks.
- [ ] Add `since` query parameter to `listDiagnosisKeys` endpoint to allow
      offsetting the Diagnosis Key list to minimize data size.

## Status

The project is currently under active development.

## License

[MIT](LICENSE)

(c) 2020 David Stotijn
