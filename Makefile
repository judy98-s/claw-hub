.PHONY: help keys dev test test-integration migrate fmt vet check

TEST_DATABASE_URL ?= postgres://clawhub:clawhub@localhost:5432/clawhub?sslmode=disable

help:
	@echo "make keys              암호화 키 3종 생성 (.env 에 붙여넣기)"
	@echo "make dev               postgres + redis 기동"
	@echo "make test              단위 테스트 (DB 불필요)"
	@echo "make test-integration  통합 테스트 (postgres 필요)"
	@echo "make check             fmt + vet + 전체 테스트"

keys:
	@echo "DATA_ENCRYPTION_KEY=$$(openssl rand -base64 32)"
	@echo "HASH_PEPPER=$$(openssl rand -base64 32)"
	@echo "SESSION_SECRET=$$(openssl rand -base64 32)"

dev:
	docker compose up -d postgres redis

test:
	go test -race ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -race ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

check: fmt vet test
