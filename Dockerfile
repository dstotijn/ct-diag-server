FROM golang:1.14-alpine AS builder

WORKDIR /app
COPY . .
ARG CGO_ENABLED=0
RUN go build

FROM alpine:3.11

ENV POSTGRES_DSN=postgres://localhost:5432/ct-diag
WORKDIR /app
COPY --from=builder /app/ct-diag-server .

ENTRYPOINT ["./ct-diag-server"]

EXPOSE 80