.PHONY: build test e2e e2e-live lint tidy
build:
	go build -o bin/captcha-solver-mcp ./cmd/captcha-solver-mcp
test:
	go test ./...
e2e:
	go test -tags e2e ./test/e2e/...
e2e-live:
	CAPTCHA_E2E_LIVE=1 go test -tags e2e,live ./test/e2e/...
lint:
	golangci-lint run
tidy:
	go mod tidy
