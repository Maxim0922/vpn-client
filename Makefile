
MODULE      := github.com/max-tsx/max-vpn
BIN_DIR     := bin
DAEMON      := $(BIN_DIR)/maxvpnd
GOFLAGS     :=
DAEMON_TAGS := with_gvisor,with_utls

HELPER_DIR  := /Library/PrivilegedHelperTools
HELPER_BIN  := $(HELPER_DIR)/maxvpnd
PLIST_SRC   := packaging/de.max-tsx.maxvpn.daemon.plist
PLIST_DST   := /Library/LaunchDaemons/de.max-tsx.maxvpn.daemon.plist

.PHONY: all build daemon test vet fmt clean run-dryrun install-daemon uninstall-daemon \
        frontend app pkg sign notarize

all: build

build: daemon

daemon:
	@mkdir -p $(BIN_DIR)
	CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build $(GOFLAGS) -tags $(DAEMON_TAGS) -o $(DAEMON) ./cmd/maxvpnd

test:
	go test -tags $(DAEMON_TAGS) ./...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

clean:
	rm -rf $(BIN_DIR)

new:
	sudo rm -rf build
	make pkg && sudo installer -pkg build/max-vpn.pkg -target /
	open /Applications/max-vpn.app

run-dryrun: daemon
	sudo MAXVPN_DRYRUN=1 $(DAEMON)

install-daemon: daemon
	sudo mkdir -p $(HELPER_DIR)
	sudo install -m 0755 $(DAEMON) $(HELPER_BIN)
	sudo install -m 0644 $(PLIST_SRC) $(PLIST_DST)
	sudo launchctl bootstrap system $(PLIST_DST)
	@echo "installed. check: sudo launchctl print system/de.max-tsx.maxvpn.daemon"

uninstall-daemon:
	-sudo launchctl bootout system/de.max-tsx.maxvpn.daemon
	-sudo rm -f $(PLIST_DST) $(HELPER_BIN)
	@echo "uninstalled."

frontend:
	cd app/frontend && npm install && npm run build

app:
	ARCH=$(or $(ARCH),arm64) packaging/build-app.sh

pkg:
	ARCH=$(or $(ARCH),arm64) packaging/build-pkg.sh

sign:
	@[ -n "$(IDENTITY)" ] || { echo "set IDENTITY=..."; exit 1; }
	codesign --force --options runtime --sign "$(IDENTITY)" build/maxvpnd
	codesign --force --options runtime --deep --sign "$(IDENTITY)" build/max-vpn.app
	@echo "signed. (stub — verify with: codesign -dv --verbose=4 build/max-vpn.app)"

notarize:
	@[ -n "$(KEYCHAIN_PROFILE)" ] || { echo "set KEYCHAIN_PROFILE=..."; exit 1; }
	xcrun notarytool submit build/max-vpn.pkg --keychain-profile "$(KEYCHAIN_PROFILE)" --wait
	xcrun stapler staple build/max-vpn.pkg
	@echo "notarized + stapled. (stub)"
