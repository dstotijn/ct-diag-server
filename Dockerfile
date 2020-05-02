ARG GO_VERSION=1.14
ARG CGO_ENABLED=0

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build .

FROM alpine:3.11

ENV POSTGRES_DSN=postgres://localhost:5432/ct-diag
WORKDIR /app
COPY --from=builder /app/ct-diag-server .

ENTRYPOINT ["./ct-diag-server"]

EXPOSE 80