.PHONY: ios-build ios-install build build-agent-linux build-agent-macos build-desktop-agent test check run

build:
	go build -o bin/servterm ./cmd/servterm

build-agent-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-agent-linux-amd64 ./cmd/servterm-agent
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-runner-probe-linux-amd64 ./cmd/servterm-runner-probe
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-accelerator-probe-linux-amd64 ./cmd/servterm-accelerator-probe
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-resource-guard-linux-amd64 ./cmd/servterm-resource-guard

build-agent-macos:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -trimpath -o bin/servterm-agent-darwin-arm64 ./cmd/servterm-agent
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-agent-darwin-amd64 ./cmd/servterm-agent
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-accelerator-probe-darwin-arm64 ./cmd/servterm-accelerator-probe
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-accelerator-probe-darwin-amd64 ./cmd/servterm-accelerator-probe

build-desktop-agent:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-desktop-agent-linux-amd64 ./cmd/servterm-desktop-agent
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-desktop-agent-darwin-arm64 ./cmd/servterm-desktop-agent
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/servterm-desktop-agent-darwin-amd64 ./cmd/servterm-desktop-agent

test:
	go test ./...

check:
	go fmt ./...
	go vet ./...
	go test -race ./...

run:
	go run ./cmd/servterm --config servterm.example.yaml

# The iPhone app. DEVELOPMENT_TEAM signs it; override it for another team.
IOS_TEAM ?= A2Y4895Q34
ios-build ios-install: export DEVELOPER_DIR ?= /Applications/Xcode.app/Contents/Developer
IOS_FLAGS = -scheme ServtermMobile -derivedDataPath build/dd -skipPackagePluginValidation -skipMacroValidation

ios-build:
	cd ios && xcodegen generate && xcodebuild $(IOS_FLAGS) -destination 'generic/platform=iOS' \
		DEVELOPMENT_TEAM=$(IOS_TEAM) -allowProvisioningUpdates build

ios-install: ios-build
	@device=$$(xcrun devicectl list devices 2>/dev/null | grep connected | grep -oE '[0-9A-F]{8}-[0-9A-F]{16}' | head -1); \
	if [ -z "$$device" ]; then echo "no connected iPhone"; exit 1; fi; \
	xcrun devicectl device install app --device $$device ios/build/dd/Build/Products/Debug-iphoneos/ServtermMobile.app && \
	xcrun devicectl device process launch --device $$device --terminate-existing com.servterm.mobile
