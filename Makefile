.PHONY: build clean run all

BINARY  = lmtm
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/tunneler

run:
	go run ./cmd/tunneler

clean:
	rm -f $(BINARY) $(BINARY)-*

# Cross-compile for all platforms.
all: clean
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64   ./cmd/tunneler
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64   ./cmd/tunneler
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64  ./cmd/tunneler
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64  ./cmd/tunneler
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe ./cmd/tunneler
