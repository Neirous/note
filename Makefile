.PHONY: all build test clean dev dev-frontend docker-build docker-run seed fmt lint help

APP_NAME  := note
GO_SRC    := ./cmd/server
SEED_SRC  := ./cmd/seednotes
VITE_DIR  := web/frontend
STATIC_DIR := web/static
DOCKER_IMAGE ?= rag-note

# ---- 开发 ----

# 启动后端（本地开发，前端用代理连 8080）
dev: build-frontend
	@echo "==> Starting server on :8080"
	@go run $(GO_SRC)

# 启动前端开发服务器（:5173，API 代理到 :8080）
dev-frontend:
	@echo "==> Vite dev server on :5173"
	@cd $(VITE_DIR) && npm run dev

# ---- 构建 ----

# 构建全部：前端 + Go 二进制
all: build-frontend build-server
	@echo "==> Build complete: $(APP_NAME), $(STATIC_DIR)"

# 构建前端产物到 web/static
build-frontend:
	@echo "==> Building frontend..."
	@cd $(VITE_DIR) && npm ci && npm run build

# 构建 Go 二进制
build-server:
	@echo "==> Building server..."
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o $(APP_NAME) $(GO_SRC)

# 仅构建 Go 二进制（跳过前端）
server: build-server
	@echo "==> Server binary: $(APP_NAME)"

# ---- 测试 ----

test:
	@echo "==> Running tests..."
	@go test ./...

test-verbose:
	@echo "==> Running tests (verbose)..."
	@go test -v ./...

test-cover:
	@echo "==> Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report: coverage.html"

# ---- 演示数据 ----

seed:
	@echo "==> Generating demo notes..."
	@go run $(SEED_SRC)

# ---- Docker ----

docker-build:
	@echo "==> Building Docker image: $(DOCKER_IMAGE)"
	@docker build -t $(DOCKER_IMAGE) .

docker-run:
	@echo "==> Running Docker container on :8080"
	@docker run -d --name $(APP_NAME) \
		-p 8080:8080 \
		-v $(APP_NAME)-data:/app/data \
		$(DOCKER_IMAGE)

docker-stop:
	@echo "==> Stopping container: $(APP_NAME)"
	@docker stop $(APP_NAME) 2>/dev/null || true
	@docker rm $(APP_NAME) 2>/dev/null || true

# ---- 代码质量 ----

fmt:
	@echo "==> Formatting Go code..."
	@go fmt ./...

lint:
	@echo "==> Running vet..."
	@go vet ./...

# ---- 清理 ----

clean:
	@echo "==> Cleaning build artifacts..."
	@rm -f $(APP_NAME)
	@rm -rf $(STATIC_DIR)/*
	@rm -f coverage.out coverage.html
	@echo "==> Clean complete"

# ---- 帮助 ----

help:
	@echo ""
	@echo "  make dev              - Build frontend + start Go server"
	@echo "  make dev-frontend     - Start Vite dev server"
	@echo "  make all              - Build frontend + Go binary"
	@echo "  make build-frontend   - Build Vue frontend only"
	@echo "  make build-server     - Build Go binary only"
	@echo "  make test             - Run all tests"
	@echo "  make test-cover       - Run tests with coverage HTML report"
	@echo "  make seed             - Generate demo notes"
	@echo "  make docker-build     - Build Docker image"
	@echo "  make docker-run       - Start Docker container"
	@echo "  make docker-stop      - Stop & remove Docker container"
	@echo "  make fmt              - go fmt ./..."
	@echo "  make lint             - go vet ./..."
	@echo "  make clean            - Remove build artifacts"
	@echo ""
