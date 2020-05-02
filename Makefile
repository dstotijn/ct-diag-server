GOFILES = $(shell find . -name '*.go')

default: build

build:
	go build .

build-ci: $(GOFILES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o workdir/ct-diag-server .