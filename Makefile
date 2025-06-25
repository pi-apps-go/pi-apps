.PHONY: all build clean install

BINDIR := $(DESTDIR)/usr/local/bin
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ || echo "Warning: unable to get current date")
GIT_COMMIT_HASH=$(shell git rev-parse HEAD || echo "Warning: unable to get Git commit hash")
LDFLAGS=-X main.BuildDate="$(BUILD_DATE)" -X main.GitCommit="$(GIT_COMMIT_HASH)"

all: build

build: build-api build-pi-apps build-manage build-settings build-updater build-gui build-multi-call-pi-apps
build-debug: build-api-debug build-pi-apps-debug build-manage-debug build-settings-debug build-updater-debug build-gui-debug build-multi-call-pi-apps-debug
build-with-xlunch: build-api build-pi-apps build-manage build-settings build-updater build-gui-with-xlunch
build-with-xlunch-debug: build-api-debug build-pi-apps-debug build-manage-debug build-settings-debug build-updater-debug build-gui-with-xlunch-debug
build-with-multi-call: build-multi-call-pi-apps
build-with-multi-call-debug: build-multi-call-pi-apps-debug

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

build-gui:
	go build -o bin/gui -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/gui/main.go

build-gui-debug:
	go build -o bin/gui -ldflags "$(LDFLAGS)" ./cmd/gui/main.go

build-gui-with-xlunch:
	go build -o bin/gui -ldflags "$(LDFLAGS) -w -s" -tags=xlunch -trimpath ./cmd/gui/main.go

build-gui-with-xlunch-debug:
	go build -o bin/gui -ldflags "$(LDFLAGS)" -tags=xlunch ./cmd/gui/main.go

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

build-multi-call-pi-apps:
	go build -o bin/multi-call-pi-apps -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/multi-call-pi-apps

build-multi-call-pi-apps-debug:
	go build -o bin/multi-call-pi-apps -ldflags "$(LDFLAGS)" ./cmd/multi-call-pi-apps

clean:
	rm -rf bin/ api-go manage pi-apps settings updater gui error-report-server multi-call-pi-apps

install: build
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	install -m 755 bin/updater updater
	install -m 755 bin/gui gui
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api
	
install-debug: build-debug
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	install -m 755 bin/updater updater
	install -m 755 bin/gui gui
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	#install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#install -m 755 bash-go-api $(BINDIR)/api

install-with-multi-call: build-with-multi-call
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	ln -s multi-call-pi-apps api-go
	ln -s multi-call-pi-apps manage
	ln -s multi-call-pi-apps settings
	ln -s multi-call-pi-apps updater
	ln -s multi-call-pi-apps gui

install-with-multi-call-debug: build-with-multi-call-debug
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	ln -s multi-call-pi-apps api-go
	ln -s multi-call-pi-apps manage
	ln -s multi-call-pi-apps settings
	ln -s multi-call-pi-apps updater
	ln -s multi-call-pi-apps gui

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./... 
