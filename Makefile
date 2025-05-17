.PHONY: all build clean install

BINDIR := $(DESTDIR)/usr/local/bin
VERSION := 0.1.0

all: build

build: build-api build-pi-apps build-manage
build-debug: build-api-debug build-pi-apps-debug build-manage-debug

build-api:
	go build -o bin/api -ldflags "-X main.Version=$(VERSION) -w -s" -trimpath ./cmd/api

build-pi-apps:
	go build -o bin/pi-apps -ldflags "-X main.Version=$(VERSION) -w -s" -trimpath ./cmd/pi-apps

build-api-debug:
	go build -o bin/api -ldflags "-X main.Version=$(VERSION)" ./cmd/api

build-pi-apps-debug:
	go build -o bin/pi-apps -ldflags "-X main.Version=$(VERSION)" ./cmd/pi-apps

build-manage:
	go build -o bin/manage -ldflags "-X main.Version=$(VERSION) -w -s" -trimpath ./cmd/manage/main.go

build-manage-debug:
	go build -o bin/manage -ldflags "-X main.Version=$(VERSION)" ./cmd/manage/main.go

clean:
	rm -rf bin/

install: build
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api
	
install-debug: build-debug
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./... 
