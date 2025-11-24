# Redbook Auth Service

This repository contains a lightweight authentication / session demo built with Gin,
GORM, MySQL and Redis. It exposes registration, login, refresh and logout endpoints
and ships with a stress-test harness that simulates hundreds of concurrent devices.

## Architecture overview

- **API layer (`api/v1`)** – Gin handlers that perform validation and translate HTTP requests into service calls.
- **Service layer (`service`)** – orchestrates DAO access, password hashing and session bookkeeping.
- **Auth helpers (`internal/auth`)** – JWT generation/validation plus Redis-based session manager & blacklist.
- **Data layer (`dao`, `model`)** – GORM models and repository helpers.
- **Test harness (`internal/test`)** – high-load scenario that verifies access/refresh behaviour and produces reports.

## Features

- JWT based access / refresh token pair with device binding.
- Redis-backed refresh token store plus access-token blacklist.
- Gin HTTP APIs with request validation and middleware-protected routes.
- Redis-powered login rate limiting to curb brute-force attacks.
- Prometheus `/metrics` endpoint exposing login/refresh/logout counters.
- Concurrency test runner (`internal/test/test_suite.go`) that produces CSV + HTML reports.

## Project layout

| Path | Description |
| --- | --- |
| `cmd/main.go` | Bootstraps config, DB, Redis and HTTP routes. |
| `api/v1` | HTTP handlers and request DTOs. |
| `service` | Business logic (user registration, login). |
| `dao` | Database access via GORM. |
| `internal/auth` | JWT helpers and Redis session manager. |
| `internal/test` | Go-based stress / regression tests. |
| `config` | Config loader, read from `config.yaml`. |

## Getting started

1. **Install prerequisites**
   - Go 1.21+
   - MySQL 8.x
   - Redis 6.x
2. **Install Go dependencies**
   ```bash
   go mod download
   ```
3. **Configure services**
   - Update `config.yaml` with your MySQL DSN and Redis connection info.
   - Optional: use `docker-compose.yaml` to spin up local MySQL/Redis.
4. **Run database migrations**
   ```bash
   go run cmd/main.go # first run auto-migrates user schema
   ```
5. **Start the API server**
   ```bash
   go run cmd/main.go
   ```
   The server listens on `config.server.port` (default `:8080`).

## API endpoints

| Method | Path | Description | Auth |
| --- | --- | --- | --- |
| `POST` | `/api/v1/users/register` | Create a new user with username/password/mobile. | No |
| `POST` | `/api/v1/users/login` | Issue access/refresh token pair, device-bound via `X-Device` (rate-limited). | No |
| `POST` | `/api/v1/users/refresh` | Validate refresh token, rotate both tokens, blacklist the old refresh. | Refresh token |
| `POST` | `/api/v1/users/logout` | Accepts access or refresh token, blacklists tokens and deletes refresh entry. | Access or refresh |

## Observability & rate limiting

- **Metrics**: scrape `GET /metrics` with Prometheus; counters include `redbook_login_attempts_total`, `redbook_refresh_rotations_total`, `redbook_logout_events_total`, and `redbook_rate_limit_hits_total`.
- **Rate limiting**: login endpoint guarded by a Redis-backed per-IP limiter (default 5 requests/min). Tune by editing `cmd/main.go` or promote to config/env variables.

## Stress test harness

The script under `internal/test/test_suite.go` can simulate hundreds of devices:

```bash
cd internal/test
go run test_suite.go
```

Outputs:
- `logout_report.csv`: raw per-device results.
- `logout_report.html`: quick visual of successes / failures.

Tune device count, concurrency and output paths inside `main()` of the script.

## Testing and linting

- Unit tests: `go test ./...`
- Integration tests: set `INTEGRATION_BASE_URL` (or run via docker-compose, see below) then `go test ./tests/integration`.
- Static analysis: `golangci-lint run` (configure as needed).
- Concurrency validation: run the stress harness above; inspect CSV/HTML outputs plus Redis key logs.

## Docker & integration workflow

1. Build and launch stack (app + MySQL + Redis):
   ```bash
   docker-compose up --build app mysql redis
   ```
2. Run integration tests inside Docker (talking to the live stack):
   ```bash
   docker-compose run --rm integration-tests
   ```
3. Prometheus can scrape the host's `localhost:8080/metrics` or target `app:8080/metrics` inside the compose network.

## Optimization notes

- Removed the unused `service/SessionManager` implementation to avoid diverging
  logic—everything now uses `internal/auth/session.go`.
- Added consistent comments and helpers across modules to improve maintainability.
- Centralized HTTP helper logic in the test suite to reduce duplicated request code.

Potential future improvements:
- Extract a dedicated refresh endpoint with rotation + sliding expiration.
- Add integration tests covering DAO and middleware behavior (can leverage the stress harness data).
- Containerize the API server plus dependencies for one-command bootstrapping.
- Instrument with metrics / tracing (Prometheus + OpenTelemetry) to observe token churn under load.
- Implement rate limiting on login/refresh endpoints to avoid brute-force attacks.
- Expand integration-test matrix to cover refresh rotations, invalid device headers and rate-limit behaviour automatically.

---

## 中文版 README

### 项目简介

Redbook Auth Service 是一个基于 Gin + GORM + MySQL + Redis 的轻量级认证 / 会话演示，提供用户注册、登录、刷新与注销 API，并附带一个可以模拟数百设备并发操作的压测脚本。

### 架构概览

- **API 层（`api/v1`）**：Gin Handler，负责参数校验与请求路由。
- **Service 层（`service`）**：封装业务逻辑、密码校验、会话管理。
- **Auth 模块（`internal/auth`）**：JWT 生成/解析以及 Redis Session Manager。
- **数据访问层（`dao`、`model`）**：基于 GORM 的模型与仓储。
- **测试与压测（`internal/test`）**：包含 CLI 压测脚本与集成测试。

### 功能特性

- Access/Refresh 双 Token，绑定设备信息。
- Redis 存储 Refresh Token，并维护 Access Token 黑名单。
- 登录接口具备 Redis 限流（默认每 IP 每分钟 5 次）与 Prometheus 指标采集。
- `/metrics` 暴露登录/刷新/注销及限流统计。
- `internal/test/test_suite.go` 可输出 CSV + HTML 的多端压测报告。

### 快速开始

1. 安装依赖：Go 1.21+、MySQL 8、Redis 6。
2. `go mod download`
3. 根据环境修改 `config.yaml`（或通过 `CONFIG_PATH`、`MYSQL_DSN` 等环境变量覆盖）。
4. 首次运行 `go run cmd/main.go` 自动迁移用户表。
5. 通过 `go run cmd/main.go` 启动服务（默认端口 `:8080`）。

### API 列表（节选）

| Method | Path | 描述 | 鉴权 |
| --- | --- | --- | --- |
| POST | `/api/v1/users/register` | 创建用户（用户名、密码、手机号） | 无 |
| POST | `/api/v1/users/login` | 签发 Access/Refresh Token，需 `X-Device` | 无 |
| POST | `/api/v1/users/refresh` | 校验 refresh、旋转 token 并拉黑旧 refresh | Refresh Token |
| POST | `/api/v1/users/logout` | 支持 access 或 refresh 注销，清理黑名单与 Redis | Access/Refresh |

### 观测性与限流

- Prometheus 采集：`redbook_login_attempts_total`、`redbook_refresh_rotations_total`、`redbook_logout_events_total`、`redbook_rate_limit_hits_total` 等指标。
- 登录限流：Redis 计数器实现滑动窗口，可在 `cmd/main.go` 中调整阈值或替换为配置项。

### 测试 & 压测

- 单元测试：`go test ./...`
- 集成测试：设置 `INTEGRATION_BASE_URL` 或使用 `docker-compose run --rm integration-tests`
- 压测脚本：`cd internal/test && go run test_suite.go`，默认模拟 200 个设备并输出 CSV/HTML。

### Docker / docker-compose

1. `docker-compose up --build app mysql redis` 启动 API + MySQL + Redis。
2. `docker-compose run --rm integration-tests` 在容器内执行端到端测试。
3. Prometheus 可抓取宿主 `localhost:8080/metrics` 或 Compose 网络内的 `app:8080/metrics`。

### 后续优化建议

- 拆分独立的 refresh 轮换与滑动过期策略。
- 扩展集成测试场景（设备不匹配、限流命中、黑名单验证等）。
- 引入容器化打包 / 多环境部署，以及 Prometheus + OpenTelemetry 的更完整可观测性。
- 在登录/刷新入口加入更多安全策略（例如 IP 黑名单、验证码、人机验证等）。

