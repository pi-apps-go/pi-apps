.PHONY: all build clean install

# Enable Go experiments via enviroment variable
export GOEXPERIMENT=greenteagc,heapminimum512kib,newinliner

BINDIR := $(DESTDIR)/usr/local/bin
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ || echo "Warning: unable to get current date")
GIT_COMMIT_HASH=$(shell git rev-parse HEAD || echo "Warning: unable to get Git commit hash")
LDFLAGS=-X main.BuildDate="$(BUILD_DATE)" -X main.GitCommit="$(GIT_COMMIT_HASH)"

PKG_MGR := $(shell \
    if command -v apt >/dev/null 2>&1; then echo apt; \
    elif command -v apk >/dev/null 2>&1; then echo apk; \
    else echo dummy; fi)

ifeq ($(PKG_MGR),dummy)
$(warning "Unknown package manager, using dummy package manager")
PKG_MGR := dummy
endif

all: build

build: build-api build-pi-apps build-manage build-settings build-updater build-gui build-xgotext
build-debug: build-api-debug build-pi-apps-debug build-manage-debug build-settings-debug build-updater-debug build-gui-debug build-xgotext-debug
build-with-multi-call: build-multi-call-pi-apps
build-with-multi-call-debug: build-multi-call-pi-apps-debug

build-api:
	go build -o bin/api -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/api

build-pi-apps:
	go build -o bin/pi-apps -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/pi-apps

build-api-debug:
	go build -o bin/api -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/api

build-pi-apps-debug:
	go build -o bin/pi-apps -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/pi-apps

build-manage:
	go build -o bin/manage -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/manage/main.go

build-manage-debug:
	go build -o bin/manage -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/manage/main.go

build-gui:
	go build -o bin/gui -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/gui/main.go

build-gui-debug:
	go build -o bin/gui -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/gui/main.go

build-settings:
	go build -o bin/settings -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/settings

build-settings-debug:
	go build -o bin/settings -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/settings

build-updater:
	go build -o bin/updater -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/updater

build-updater-debug:
	go build -o bin/updater -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/updater

# xpi-apps utility is currently disabled from being built due to it not being feature complete
build-xpi-apps:
	go build -o bin/xpi-apps -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/xpi-apps

build-xpi-apps-debug:
	go build -o bin/xpi-apps -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/xpi-apps

build-xgotext:
	go build -o bin/xgotext -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/xgotext

build-xgotext-debug:
	go build -o bin/xgotext -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/xgotext

# Note: error-report-server is not meant to be compiled by a user and is not included during compiling unless you are hosting the error report server yourself.
build-error-report-server:
	go build -o bin/error-report-server -ldflags "-w -s" -trimpath ./cmd/error-report-server/main.go

build-error-report-server-debug:
	go build -o bin/error-report-server ./cmd/error-report-server/main.go

# If multi-call-pi-apps is used, the normal pi-apps-go seperated binaries cannot be used at the same time..
build-multi-call-pi-apps:
	go build -o bin/multi-call-pi-apps -ldflags "$(LDFLAGS) -w -s" -trimpath -tags=$(PKG_MGR) ./cmd/multi-call-pi-apps

build-multi-call-pi-apps-debug:
	go build -o bin/multi-call-pi-apps -ldflags "$(LDFLAGS)" -tags=$(PKG_MGR) ./cmd/multi-call-pi-apps

clean:
	rm -rf bin/ api-go manage pi-apps settings updater gui error-report-server multi-call-pi-apps xpi-apps

install: build
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	install -m 755 bin/updater updater
	install -m 755 bin/gui gui
	install -m 755 bin/xpi-apps xpi-apps
	install -m 755 bin/xgotext xgotext
	sudo install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#sudo install -m 755 bin/xpi-apps $(BINDIR)/xpi-apps
	#install -m 755 bash-go-api $(BINDIR)/api
	
install-debug: build-debug
	install -m 755 bin/api api-go
	install -m 755 bin/manage manage
	install -m 755 bin/settings settings
	install -m 755 bin/updater updater
	install -m 755 bin/gui gui
	install -m 755 bin/xpi-apps xpi-apps
	install -m 755 bin/xgotext xgotext
	sudo install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	#sudo install -m 755 bin/xpi-apps $(BINDIR)/xpi-apps
	#install -m 755 bash-go-api $(BINDIR)/api

install-with-multi-call: clean build-with-multi-call build-pi-apps
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	sudo install -m 755 bin/pi-apps $(BINDIR)/pi-apps
	ln -s multi-call-pi-apps api-go
	ln -s multi-call-pi-apps manage
	ln -s multi-call-pi-apps settings
	ln -s multi-call-pi-apps updater
	ln -s multi-call-pi-apps gui

install-with-multi-call-debug: clean build-with-multi-call-debug build-pi-apps-debug
	install -m 755 bin/multi-call-pi-apps multi-call-pi-apps
	sudo install -m 755 bin/pi-apps $(BINDIR)/pi-apps
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

help:
	@echo "Available targets:"
	@echo "General:"
	@echo "  build          - Build all binaries"
	@echo "  build-debug    - Build all binaries with debug symbols"
	@echo "  build-multi-call-pi-apps - Build multi-call Pi-Apps Go"
	@echo "  build-multi-call-pi-apps-debug - Build multi-call Pi-Apps Go with debug symbols"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Build and install all binaries"
	@echo "  install-debug  - Build and install all binaries with debug symbols"
	@echo "  install-with-multi-call - Install multi-call Pi-Apps Go"
	@echo "  install-with-multi-call-debug - Install multi-call Pi-Apps Go with debug symbols"
	@echo "Per binary targets:"
	@echo "  build-api      - Build api binary"
	@echo "  build-pi-apps  - Build pi-apps binary"
	@echo "  build-manage   - Build manage binary"
	@echo "  build-settings - Build settings binary"
	@echo "  build-updater  - Build updater binary"
	@echo "  build-gui      - Build gui binary"
	@echo "  build-xpi-apps - Build xpi-apps binary"
	@echo "  build-xgotext  - Build xgotext binary"
	@echo "  build-error-report-server - Build error-report-server binary"
	@echo "  build-api-debug      - Build api binary with debug symbols"
	@echo "  build-pi-apps-debug  - Build pi-apps binary with debug symbols"
	@echo "  build-manage-debug   - Build manage binary with debug symbols"
	@echo "  build-settings-debug - Build settings binary with debug symbols"
	@echo "  build-updater-debug  - Build updater binary with debug symbols"
	@echo "  build-gui-debug      - Build gui binary with debug symbols"
	@echo "  build-xpi-apps-debug - Build xpi-apps binary with debug symbols"
	@echo "  build-xgotext-debug  - Build xgotext binary with debug symbols"
	@echo "  build-error-report-server-debug - Build error-report-server binary with debug symbols"
	@echo "Development:"
	@echo "  test           - Test all binaries"
	@echo "  fmt            - Format all code"
	@echo "  vet            - Vet all code"
	@echo "  help           - Show this help" 