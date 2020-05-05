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

⚠️ Apple/Google released [sample code](https://developer.apple.com/documentation/exposurenotification/building_an_app_to_notify_users_of_covid-19_exposure) on May 4,
which clarifies some terminology and states best practices for apps and server
implementations. Check out the [issue tracker](https://github.com/dstotijn/ct-diag-server/issues)
for an up to date overview of the ongoing work as this project is being updated
accordingly.

👉 Are you an app developer or working for a government and/or health authority
looking to implement this server? Please [contact me](mailto:dstotijn@gmail.com) if you have questions,
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
- Permissive [license](LICENSE), easily forkable for other developers.

## Features

- HTTP server for storing and retrieving Diagnosis Keys. Uses
  bytestreams for sending and receiving as little data as possible over the
  wire: 21 bytes per _Diagnosis Key_ (16 bytes for the `TemporaryExposureKey`,
  4 bytes for the `RollingStartNumber` and 1 byte for the `TransmissionRiskLevel`).
- Ships with PostgreSQL adapter for storage of Diagnosis Keys, but can easily be
  forked for different adapters.
- Caching interface, with in-memory implementation.
- Cursor based offsetting for listing Diagnosis Keys, with support for byte ranges
  and cache control headers.

---

## API reference

💡 Check out the [OpenAPI reference](https://app.swaggerhub.com/apis/dstotijn84/ct-diag-server)
or import [openapi.yaml](docs/openapi.yaml) in a compatible client for exploring the
API and creating client code stubs. Also check out the [example client code](examples/client/main.go).

### Listing Diagnosis Keys

To be used for fetching a list of Diagnosis Keys. A typical client is either a mobile
device or the intermediate platform/server of an app developer, for manual/custom
distribution of the payload to clients. In either case, the keyset can be
regarded as public; it doesn't contain PII.

#### Request

`GET /diagnosis-keys`

The endpoint supports byte range requests as defined in [RFC 7233](https://tools.ietf.org/html/rfc7233).
The `HEAD` method may be used to obtain `Last-Modified` and `Content-Length` headers
for cache control purposes.

A query parameter (`after`) allows clients to only fetch keys that haven't been
handled on the device yet, to minimize redundant network traffic and parsing time.
Pass the last known/handled key (hexadecimal encoding) to retrieve only new keys
uploaded _after_ the given key.

#### Query parameters

| Name    | Description                                                                                                                                                                       |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `after` | Used for listing diagnosis keys uploaded _after_ the given key. Format: hexadecimal encoding of a Temporary Exposure Key. Example: `a7752b99be501c9c9e893b213ad82842`. (Optional) |

#### Response

A `200 OK` response should be expected for normal requests (non-empty and empty),
and `206 Partial Content` for responses to byte range requests.
In case of an empty reply, a `Content-Length: 0` header is written.

A `500 Internal Server Error` response indicates server failure, and warrants a retry.

#### Response headers

| Name                                             | Description                                                                                                                       |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| `Content-Type: application/octet-stream`         | The HTTP response is a bytestream of Diagnosis Keys (see below).                                                                  |
| `Content-Length: {n * 21}`                       | Content length is `n * 21`, where `n` is the amount of returned Diagnosis Keys (byte range requests may yield different lengths). |
| `Cache-Control: public, max-age=0, s-maxage=600` | For (upstream) caching purposes, this header may be used.                                                                         |

#### Response body

The HTTP response body is a bytestream of Diagnosis Keys. A Diagnosis Key is 21
bytes and consists of three parts: the `TemporaryExposureKey` itself (16 bytes), the `RollingStartNumber` (4 bytes, big endian) and the `TransmissionRiskLevel` (1 byte).
Because the amount of bytes per Diagnosis Key is fixed, there is no delimiter.

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

The HTTP request body should be a bytestream of `1 <= n` Diagnosis Keys, where
`n` is the max upload batch size configured on the server (default: 14).
A diagnosis key consists of three parts: the `TemporaryExposureKey` itself (16 bytes),
the `RollingStartNumber` (4 bytes, big endian) and the `TransmissionRiskLevel` (1 byte).
Because the amount of bytes per Diagnosis Key is fixed, there is no delimiter.

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

👉 See [issue tracker](https://github.com/dstotijn/ct-diag-server/issues).

## Status

The project is currently under active development.

## Contributors

David Stotijn, Martin van de Belt, Milo van der Linden, Peter Hellberg, Arian van Putten.

## Acknowledgements

Thanks to the community of [Code for NL](https://www.codefor.nl/) (`#corona-apps`
and `#corona-ct-diag-server` on [Slack](https://praatmee.codefor.nl)) for all the
valuable feedback and discussions!

## License

[MIT](LICENSE)

---

© 2020 David Stotijn — [Twitter](https://twitter.com/dstotijn), [Email](mailto:dstotijn@gmail.com)
