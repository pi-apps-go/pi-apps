.PHONY: all build clean install

BINDIR := $(DESTDIR)/usr/local/bin
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ || echo "Warning: unable to get current date")
GIT_COMMIT_HASH=$(shell git rev-parse HEAD || echo "Warning: unable to get Git commit hash")
LDFLAGS=-X main.BuildDate="$(BUILD_DATE)" -X main.GitCommit="$(GIT_COMMIT_HASH)"

all: build

build: build-api build-pi-apps build-manage build-settings build-updater
build-debug: build-api-debug build-pi-apps-debug build-manage-debug build-settings-debug build-updater-debug

build-api:
	go build -o bin/api -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/api

build-pi-apps:
	go build -o bin/pi-apps -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/pi-apps

build-api-debug:
	go build -o bin/api -ldflags "$(LDFLAGS)" ./cmd/api

build-pi-apps-debug:
	go build -o bin/pi-apps -ldflags "$(LDFLAGS)" ./cmd/pi-apps

build-manage:
	go build -o bin/manage -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/manage/main.go

build-manage-debug:
	go build -o bin/manage -ldflags "$(LDFLAGS)" ./cmd/manage/main.go

build-settings:
	go build -o bin/settings -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/settings

build-settings-debug:
	go build -o bin/settings -ldflags "$(LDFLAGS)" ./cmd/settings

build-updater:
	go build -o bin/updater -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/updater

build-updater-debug:
	go build -o bin/updater -ldflags "$(LDFLAGS)" ./cmd/updater

# Note: error-report-server is not meant to be compiled by a user and is not included during compiling unless you are hosting the error report server yourself.
build-error-report-server:
	go build -o bin/error-report-server -ldflags "-w -s" -trimpath ./cmd/error-report-server/main.go

build-error-report-server-debug:
	go build -o bin/error-report-server ./cmd/error-report-server/main.go

clean:
	rm -rf bin/

install: build
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api
	
install-debug: build-debug
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./... 
