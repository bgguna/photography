.PHONY: help build build-arm64 build-arm build-local test deploy-info clean

help:
	@echo "Photography Portfolio Build Targets"
	@echo "===================================="
	@echo "build-local     - Build for local development (current OS/architecture)"
	@echo "build-arm64     - Cross-compile for Raspberry Pi 4/5 (64-bit aarch64)"
	@echo "build-arm       - Cross-compile for Raspberry Pi 3 (32-bit armv7l)"
	@echo "test            - Run all tests"
	@echo "deploy-info     - Show deployment instructions"
	@echo "clean           - Remove build artifacts"

build-local:
	@echo "Building for local development..."
	cd server && go build -o ../bin/server .

build-arm64:
	@echo "Building for Raspberry Pi (64-bit aarch64)..."
	mkdir -p bin
	cd server && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/server-arm64 .
	@ls -lh bin/server-arm64
	@echo "✓ Binary ready: bin/server-arm64"

build-arm:
	@echo "Building for Raspberry Pi (32-bit armv7l)..."
	mkdir -p bin
	cd server && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/server-arm .
	@ls -lh bin/server-arm
	@echo "✓ Binary ready: bin/server-arm"

test:
	@echo "Running tests..."
	cd server && go test -v ./...

deploy-info:
	@echo "For deployment instructions, see: deploy/DEPLOYMENT.md"
	@echo ""
	@echo "Quick deployment checklist:"
	@echo "1. Check Pi architecture: uname -m"
	@echo "2. Cross-compile: make build-arm64 (or build-arm)"
	@echo "3. Review deploy/DEPLOYMENT.md for complete setup"
	@echo "4. Set up systemd service and external drive"
	@echo "5. Run smoke tests"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	cd server && go clean
	@echo "✓ Clean"

# Development targets
server-run:
	cd server && go run .

server-test:
	cd server && go test -v ./...

server-test-coverage:
	cd server && go test -cover ./...
