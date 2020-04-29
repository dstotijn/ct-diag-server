# ct-diag-server

[![CircleCI](https://circleci.com/gh/dstotijn/ct-diag-server.svg?style=shield)](https://circleci.com/gh/dstotijn/ct-diag-server)
[![Coverage Status](https://coveralls.io/repos/github/dstotijn/ct-diag-server/badge.svg?branch=master)](https://coveralls.io/github/dstotijn/ct-diag-server?branch=master)
[![GitHub](https://img.shields.io/github/license/dstotijn/ct-diag-server)](LICENSE)
[![GoDoc](https://godoc.org/github.com/dstotijn/ct-diag-server?status.svg)](https://godoc.org/github.com/dstotijn/ct-diag-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/dstotijn/ct-diag-server)](https://goreportcard.com/report/github.com/dstotijn/ct-diag-server)

> **ct-diag-server** is an HTTP server written in Go for storing and retrieving
> _Diagnosis Keys_, as defined in Apple/Google's [draft specification](https://www.apple.com/covid19/contacttracing/)
> of its Exposure Notification framework. It aims to respect the privacy of its users
> and store only the bare minimum of data needed for anonymous exposure notifications.

In anticipation of the general release of Apple and Google's native APIs (planned
for May 2020) to assist health organizations with contact tracing, this application
provides a reference implementation for the framework's server component: a central
repository for submitting Diagnosis Keys after a positive test, and retrieving a
collection of all previously submitted Diagnosis Keys, to be used on the device
for offline key matching.

ℹ️ The terminology and usage corresponds with v1.2 of the specification, as found
[here](https://www.apple.com/covid19/contacttracing/) and [here](https://www.blog.google/inside-google/company-announcements/apple-and-google-partner-covid-19-contact-tracing-technology/).

👉 Are you an app developer or working for a government and/or health care organization
looking to implement/use this server? Please [contact me](mailto:dstotijn@gmail.com) if you have questions,
or [open an issue](https://github.com/dstotijn/exp-notif-crypto/issues/new).

## Table of contents

- [Goals](#goals)
- [Features](#features)
- [API reference](#api-reference)
- [TODO](#todo)
- [Status](#status)
- [Contributors](#contributors)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Goals

- Privacy by design: Doesn't store or log any personally identifiable information.
- Built for high workloads and heavy use:
  - Aims to have a small memory footprint.
  - Minimal data transfer: Diagnosis Keys are uploaded/downloaded as bytestreams,
    easily cachable by CDNs or upstream (government) proxy services.
  - Ships with a Dockerfile, for easy deployment as a workload on a wide range
    of hosting platforms.
- Security: relies on Go's standard library where possible, and has minimal vendor
  dependencies.
- Solid test coverage, for easy auditing and review.
- Ships with PostgreSQL adapter for storage of Diagnosis Keys, but can easily be
  forked for different adapters and/or caching services.
- Permissive [license](LICENSE), easily forkable for other developers.

## Features

- HTTP server for storing and retrieving Diagnosis Keys. Uses
  bytestreams for sending and receiving as little data as possible over the
  wire: 20 bytes per _Diagnosis Key_ (16 bytes for the `TemporaryExposureKey`,
  4 bytes for the `ENIntervalNumber`).
- PostgreSQL support for storage.

---

## API reference

💡 Check out the [OpenAPI reference](https://app.swaggerhub.com/apis/dstotijn84/ct-diag-server)
or import [openapi.yaml](openapi.yaml) in a compatible client for exploring the
API and creating client code stubs. Also check out the [example client code](examples/client/main.go).

### Listing Diagnosis Keys

To be used for fetching a list of Diagnosis Keys. A typical client is either a mobile
device or the intermediate platform/server of an app developer, for manual/custom
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
| `Content-Type: application/octet-stream` | The HTTP response is a bytestream of Diagnosis Keys (see below).             |
| `Content-Length: {n * 20}`               | Content length is `n * 20`, where `n` is the amount of found Diagnosis Keys. |

#### Response body

The HTTP response body is a bytestream of Diagnosis Keys. A diagnosis key consists
of two parts: the `TemporaryExposureKey` itself (16 bytes), and 4 bytes (big endian)
for the `ENIntervalNumber` of the key, referred to as the "startRollingNumber" in
the spec. Because the amount of bytes per Diagnosis Key is fixed, there is no delimiter.

### Uploading Diagnosis Keys

To be used for uploading a set of Diagnosis Keys by a mobile client device.
**Note:** It's still undecided if this server should authenticate requests. Given the
wide range of per-country use cases and processes, this is now delegated to the server
operator to shield this endpoint against unauthorized access, and provide its own
upstream proxy, e.g. tailored to handle auth-z for health personnel.

#### Request

`POST /diagnosis-keys`

Any request headers (e.g. `Content-Length` and `Content-Type`) are not needed.

#### Body

The HTTP request body should be a bytestream of `1 <= 14` Diagnosis Keys.
A diagnosis key consists of two parts: the `TemporaryExposureKey` itself (16 bytes),
and 2 bytes (big endian) to denote the `ENIntervalNumber` (see above). Because
the amount of bytes per diagnosis key is fixed, there is no delimiter.

An unexpected end of the bytestream (e.g. incomplete key) results
in a `400 Bad Request` response.

Duplicate keys are silently ignored.

#### Response

A `200 OK` response with body `OK` should be expected on successful storage of the
keyset in the database.
A `400 Bad Request` response is used for client errors. A `500 Internal Server Error`
response is used for server errors, and warrants a retry. Error reasons are written
in a `text/plain; charset=utf-8` response body.

## TODO

- [ ] Add cache interface and (in-memory/file system) implementations.
- [ ] Add `ETag` response header to assist client side caching.
- [ ] Add script (optionally worker) to delete keys > 14 days.
- [ ] Add `since` query parameter to `listDiagnosisKeys` endpoint to allow
      offsetting the Diagnosis Key list by `ENIntervalNumber` to minimize data sent over the wire.
- [ ] Write benchmarks.
- [ ] Add FAQ and/or guide for server operators.

👉 See [issue tracker](https://github.com/dstotijn/ct-diag-server/issues).

## Status

The project is currently under active development.

## Contributors

David Stotijn, Martin van de Belt, Milo van der Linden, Peter Hellberg.

## Acknowledgements

Thanks to the community of [Code for NL](https://www.codefor.nl/) (`#corona-apps`
and `#corona-ct-diag-server` on [Slack](https://praatmee.codefor.nl)) for all the
valuable feedback and discussions!

## License

[MIT](LICENSE)

---

© 2020 David Stotijn — [Twitter](https://twitter.com/dstotijn), [Email](mailto:dstotijn@gmail.com)
