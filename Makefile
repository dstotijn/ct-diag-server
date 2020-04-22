GOFILES = $(shell find . -name '*.go')

default: build

build: $(GOFILES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o workdir/ct-diag-server .