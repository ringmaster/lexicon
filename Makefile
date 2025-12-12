.PHONY: build run test clean dev

# Build the binary
build:
	go build -o lexicon ./cmd/lexicon

# Run in development mode
dev:
	LEXICON_HTTP_MODE=true \
	LEXICON_SESSION_SECRET=dev-secret-must-be-32-characters- \
	go run ./cmd/lexicon

# Run the built binary in development mode
run: build
	LEXICON_HTTP_MODE=true \
	LEXICON_SESSION_SECRET=dev-secret-must-be-32-characters- \
	./lexicon

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f lexicon
	rm -rf data/

# Build for multiple platforms
release:
	GOOS=linux GOARCH=amd64 go build -o lexicon-linux-amd64 ./cmd/lexicon
	GOOS=linux GOARCH=arm64 go build -o lexicon-linux-arm64 ./cmd/lexicon
	GOOS=darwin GOARCH=amd64 go build -o lexicon-darwin-amd64 ./cmd/lexicon
	GOOS=darwin GOARCH=arm64 go build -o lexicon-darwin-arm64 ./cmd/lexicon
	GOOS=windows GOARCH=amd64 go build -o lexicon-windows-amd64.exe ./cmd/lexicon
