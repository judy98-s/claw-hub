.PHONY: help keys dev dev-down api worker web tunnel check-slack test test-integration check fmt vet build up down logs setup-account

TEST_DATABASE_URL ?= postgres://clawhub:clawhub@localhost:5432/clawhub?sslmode=disable

help:
	@echo "개발"
	@echo "  make keys              암호화 키와 DB 비밀번호 생성 (.env 에 붙여넣기)"
	@echo "  make dev               postgres + redis 기동"
	@echo "  make api               API 서버 실행"
	@echo "  make worker            워커 실행"
	@echo "  make web               프론트 개발 서버 실행"
	@echo "  make tunnel            공개 HTTPS 주소 붙이기 (손님 폰에서 접속)"
	@echo "  make check-slack       Slack 웹훅이 살아 있는지 테스트 메시지 발송"
	@echo "  make setup-account     첫 매장과 사장님 계정 생성"
	@echo ""
	@echo "검증"
	@echo "  make test              단위 테스트 (DB 불필요)"
	@echo "  make test-integration  통합 테스트 (postgres 필요)"
	@echo "  make check             fmt + vet + 전체 테스트"
	@echo ""
	@echo "배포"
	@echo "  make up                전체 스택 기동 (docker compose)"
	@echo "  make down              전체 스택 종료"
	@echo "  make logs              로그 보기"

keys:
	@echo "DATA_ENCRYPTION_KEY=$$(openssl rand -base64 32)"
	@echo "HASH_PEPPER=$$(openssl rand -base64 32)"
	@echo "SESSION_SECRET=$$(openssl rand -base64 32)"
	@echo "POSTGRES_PASSWORD=$$(openssl rand -base64 24 | tr -d '/+=')"

dev:
	docker compose -f docker-compose.dev.yml up -d

dev-down:
	docker compose -f docker-compose.dev.yml down

api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

web:
	cd web && npm run dev

tunnel:
	./scripts/dev-tunnel.sh

check-slack:
	./scripts/check-slack.sh

setup-account:
	@test -n "$(STORE)"    || (echo "사용법: make setup-account STORE=매장이름 EMAIL=주소 PASSWORD=비밀번호" && exit 1)
	@test -n "$(EMAIL)"    || (echo "EMAIL 이 필요합니다" && exit 1)
	@test -n "$(PASSWORD)" || (echo "PASSWORD 가 필요합니다" && exit 1)
	go run ./cmd/setup -store "$(STORE)" -email "$(EMAIL)" -password "$(PASSWORD)" -phone "$(PHONE)"

test:
	go test -race ./...
	cd web && npm run typecheck

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -race ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

check: fmt vet test

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f --tail=100
