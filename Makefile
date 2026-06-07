.PHONY: build test e2e e2e-live e2e-browser lint tidy
build:
	go build -o bin/captcha-solver-mcp ./cmd/captcha-solver-mcp
test:
	go test ./...
e2e:
	go test -tags e2e ./test/e2e/...
e2e-live:
	CAPTCHA_E2E_LIVE=1 go test -tags e2e,live ./test/e2e/...
# Live browser e2e against the 2captcha demo (network + Chromium required).
# Prereqs: npm install && npx playwright install chromium
e2e-browser:
	CAPTCHA_E2E_LIVE=1 \
	CAPTCHA_LLM_PROVIDER=openai-compatible \
	CAPTCHA_LLM_BASE_URL=https://llmgw.h.raczylo.com \
	CAPTCHA_LLM_MODEL=openai/gpt-5.4 \
	CAPTCHA_LLM_TIMEOUT=120s \
	go test -tags 'e2e live' -run Browser -v ./test/e2e/...
lint:
	golangci-lint run
tidy:
	go mod tidy
