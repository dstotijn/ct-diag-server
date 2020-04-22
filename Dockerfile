FROM golang:1.14-alpine

WORKDIR /go/src/ct-diag-server
COPY . .

RUN go get -d -v ./... && \
    go install -v ./...

EXPOSE 80

CMD ["ct-diag-server"]