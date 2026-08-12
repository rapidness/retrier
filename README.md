# Retry Middleware — 智能重试中间层

[English](#english) | [中文](#中文)

---

<a name="中文"></a>

一个部署在 **AI Agent 与 LLM API 之间**的透明反向代理，根据可配置的规则对失败的 API 调用执行**短路重试**——Agent 只需修改 `base_url`，无需任何代码改动。

```
┌─────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│  AI Agent   │────▶│  Retry Middleware    │────▶│  Upstream API    │
│             │◀────│  (规则引擎 + 重试)    │◀────│  (LLM / CC Switch)│
└─────────────┘     └──────────────────────┘     └──────────────────┘
                     仅返回最终结果给 Agent
```

## 为什么需要它？

传统重试方案的问题：

| 问题 | 说明 |
|------|------|
| **规则僵化** | 多数 Agent 只对 HTTP 429/5xx 做简单重试，无法处理业务级错误（如 HTTP 200 + `{"code": 700}`） |
| **Agent 被迫感知错误** | 可恢复错误一路穿透到 Agent，导致代码臃肿且重复 |
| **重试逻辑硬编码** | 改重试条件就要改代码、重新部署 |

**Retry Middleware** 的核心价值：匹配到可恢复错误后，**在中间层内部完成重试**，Agent 只收到最终的成功响应或彻底失败——就像这次请求只是"慢了一点"。

## 核心特性

- 🎯 **多维度规则匹配** — HTTP 状态码 / 响应头 / JSONPath / 文本(contains+regex) / AND·OR·NOT 逻辑组合
- 🔄 **短路重试** — 匹配即重试，Agent 零感知，仅收到最终结果
- ⏱️ **三种退避策略** — Fixed / Exponential / Linear，支持 Jitter 防惊群
- 🔒 **重试预算** — 滑动窗口限流，防止重试风暴拖垮上游
- 🔥 **配置热加载** — 修改 YAML 后 500ms 内生效，无需重启
- 🪶 **零开销日志** — 关闭时 atomic.Bool 快路径 ~1ns，开启后结构化 JSON + 自动脱敏 + 轮转
- 📊 **Prometheus 指标** — 8 项自定义指标，自定义 Registry 避免注册冲突
- 🖥️ **Web 管理界面** — React 18 + Ant Design 5 SPA，支持在线编辑规则和日志开关
- 🐳 **开箱即用** — Docker 镜像 + 跨平台二进制 (Linux / macOS / Windows)

## 快速开始

### 1. 编译运行

```bash
# 编译
make build

# 运行（使用默认配置）
./bin/retry-middleware -config ./configs/config.yaml
```

### 2. Docker

```bash
# 构建镜像
make docker

# 运行
docker run -d \
  -p 15722:15722 \
  -p 9090:9090 \
  -p 15723:15723 \
  -v $(pwd)/configs:/app/configs \
  retry-middleware:latest
```

### 3. 让 Agent 接入

只需将 Agent 的 `base_url` 从上游 API 地址改为中间层地址：

```python
# 之前
client = OpenAI(base_url="https://api.upstream.com/v1")

# 之后
client = OpenAI(base_url="http://127.0.0.1:15722/v1")
```

无需改动任何业务代码。

## 配置说明

完整配置文件见 [`configs/config.yaml`](configs/config.yaml)，以下为核心结构：

```yaml
# ---- 日志配置 ----
logging:
  enabled: false              # 总开关（日常关闭，排障时一键开启）
  log_requests: false         # 记录请求体
  log_responses: false        # 记录响应体
  log_retries: true           # 记录重试事件
  max_body_size: 1048576      # Body 最大记录长度 (bytes)
  output: file                # file / stdout / both
  file_path: ./logs/retry-middleware.log
  max_file_size: 100          # 单文件最大 (MB)
  max_files: 10               # 保留文件数

# ---- 重试规则 ----
rules:
  # 示例1: HTTP 200 但业务码 700 → 重试
  - name: retry-on-code-700
    description: "配额耗尽或临时业务错误，重试3次"
    match:
      http_status: 200
      json_path_match:
        path: $.code
        operator: "=="
        value: 700
    action:
      max_attempts: 3
      backoff:
        strategy: exponential
        initial_delay: 500     # ms
        multiplier: 2
        max_delay: 5000        # ms
        jitter: true

  # 示例2: 限流或服务端错误 → 重试
  - name: retry-on-429-and-5xx
    description: "限流或服务端临时故障"
    match:
      http_status: [429, 502, 503, 504]
    action:
      max_attempts: 5
      backoff:
        strategy: exponential
        initial_delay: 500
        multiplier: 1.5
        max_delay: 5000
        jitter: true

  # 示例3: 文本匹配 → 重试
  - name: retry-on-502-text
    description: "CC Switch local proxy failed"
    match:
      text_match:
        contains: "502 CC Switch local proxy failed"
    action:
      max_attempts: 10
      backoff:
        strategy: exponential
        initial_delay: 1000
        multiplier: 2
        max_delay: 30000
        jitter: true

  # 示例4: 永不重试
  - name: never-retry-on-400
    match:
      http_status: 400
    action:
      max_attempts: 1
      skip_retry: true

# ---- 代理配置 ----
proxy:
  listen: 127.0.0.1:15722
  upstream: http://127.0.0.1:15721    # 上游 LLM API 或 CC Switch
  timeout_seconds: 120
  global_timeout: 60000               # 重试总耗时上限 (ms)

# ---- 全局重试预算 ----
rate_limit:
  retry_burst: 100                    # 窗口内最大重试次数
  retry_burst_window: 60              # 窗口 (seconds)
```

### 规则匹配维度

| 维度 | 配置方式 | 说明 |
|------|---------|------|
| **HTTP 状态码** | `http_status: 429` / `[429, 502, 503]` / `"5xx"` | 精确 / 列表 / 范围匹配 |
| **响应头** | `headers: [{name: "X-RateLimited", value: "true"}]` | 存在性或值匹配 |
| **JSONPath** | `json_path_match: {path: "$.code", operator: "==", value: 700}` | 支持 `==` `!=` `>` `<` `>=` `<=` `contains` |
| **文本** | `text_match: {contains: "error"}` / `{regex: "^5\\d{2}"}` | 子串 / 正则匹配 |
| **逻辑组合** | `logic: {and: [...]} / {or: [...]} / {not: ...}` | 任意嵌套组合 |

多个维度同时指定时为 **AND 语义**（全部满足才匹配），规则按配置顺序 **first-match-wins**。

### 退避策略

| 策略 | 公式 | 适用场景 |
|------|------|---------|
| `fixed` | `delay = initial_delay` | 固定间隔 |
| `linear` | `delay = initial_delay × attempt` | 线性增长 |
| `exponential` | `delay = initial_delay × multiplier^(attempt-1)` | 指数退避（推荐） |

开启 `jitter: true` 后，实际延迟在 `[0.5×delay, 1.5×delay]` 间随机，避免惊群效应。

## 管理界面

中间层内置 Web 管理界面（默认 `http://localhost:15723`），基于 React 18 + Ant Design 5：

- 📋 规则列表 — 查看、新增、编辑、删除重试规则
- 🔧 日志开关 — 一键开启/关闭日志，实时生效
- 📊 运行状态 — 查看当前配置和代理状态

### REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/config` | 获取当前完整配置 |
| `PUT` | `/api/config` | 更新完整配置（写盘 + 热加载） |
| `GET` | `/api/rules` | 列出所有规则 |
| `POST` | `/api/rules` | 新增规则 |
| `PUT` | `/api/rules/:name` | 更新指定规则 |
| `DELETE` | `/api/rules/:name` | 删除指定规则 |
| `PUT` | `/api/logging` | 更新日志配置 |
| `GET` | `/api/status` | 运行状态 |

## Prometheus 指标

默认暴露在 `:9090`，所有指标在 `retry_middleware` 子系统下：

| 指标 | 类型 | 说明 |
|------|------|------|
| `retry_middleware_retry_total` | Counter | 重试总次数 |
| `retry_middleware_retry_by_rule` | CounterVec | 按规则统计的重试次数 |
| `retry_middleware_retry_success_total` | Counter | 重试最终成功的次数 |
| `retry_middleware_retry_exhausted_total` | Counter | 重试次数耗尽的次数 |
| `retry_middleware_retry_delay_seconds` | Histogram | 实际退避延迟分布 |
| `retry_middleware_request_duration_seconds` | Histogram | 端到端请求耗时 |
| `retry_middleware_active_requests` | Gauge | 当前在飞请求数 |
| `retry_middleware_log_entries_written_total` | Counter | 写入的日志条目数 |

Grafana 示例面板（后续补充）。

## 日志系统

### 零开销设计

日志默认关闭时，所有写方法在 `atomic.Bool` 快路径后立即返回（~1ns），**无任何 I/O、无内存分配**。

### 脱敏规则

- `Authorization` / `Api-Key` / `X-Api-Key` 头值自动脱敏
- `Bearer sk-xxxxx` → `Bearer sk-***`
- `sk-xxxxx` → `sk-***`

### 日志输出格式

结构化 JSON，每行一条，`request_id` 串联全链路：

```json
{"timestamp":"2026-08-07T14:32:11.123Z","request_id":"req_abc123","event":"request","method":"POST","url":"...","headers":{"authorization":"Bearer sk-***"}}
{"timestamp":"2026-08-07T14:32:13.456Z","request_id":"req_abc123","event":"retry_triggered","attempt":1,"trigger_rule":"retry-on-code-700","next_delay_ms":2000}
{"timestamp":"2026-08-07T14:32:17.789Z","request_id":"req_abc123","event":"response","attempt":3,"success":true,"status_code":200,"total_elapsed_ms":6678}
```

## 项目结构

```
.
├── cmd/proxy/main.go          # 入口
├── internal/
│   ├── config/                # 配置加载 + fsnotify 热加载 + SaveAndReload
│   ├── rule/                  # 规则引擎（状态码/头部/JSONPath/文本/逻辑组合）
│   ├── retry/                 # 退避策略 + 滑动窗口预算 + 重试执行器
│   ├── proxy/                 # ReverseProxy + ModifyResponse 钩子
│   ├── logger/                # 零开销 toggle 日志 + 脱敏 + lumberjack 轮转
│   ├── middleware/            # X-Request-ID 注入
│   ├── metrics/               # Prometheus 自定义 registry
│   └── admin/                 # REST API + go:embed 前端 SPA
├── web/                       # React 18 + Vite + Ant Design 5 + TypeScript
├── configs/config.yaml        # 示例配置
├── Dockerfile                 # 多阶段构建
└── Makefile                   # 编译/测试/构建目标
```

## 开发

```bash
# 编译
make build

# 跨平台编译
make build-all

# 运行测试
make test

# 测试覆盖率
make test-cover

# 性能基准
make benchmark

# 本地运行
make run

# 调试模式（开启日志）
make run-debug

# 构建 Docker 镜像
make docker

# 构建前端
cd web && npm install && npm run build
cp -r dist ../internal/admin/dist   # 嵌入到 Go 二进制
```

## 设计决策

| 决策 | 原因 |
|------|------|
| `atomic.Bool` 门控日志 | 关闭时 1.58ns/op，真正零开销 |
| Prometheus 自定义 Registry | 避免测试中重复注册 panic |
| `http.NewRequestWithContext` 重建请求 | 避免 Clone 时 RequestURI 冲突 |
| fsnotify 目录监听 + 500ms 防抖 | 兼容编辑器原子写入（临时文件+重命名） |
| `atomic.Pointer` RCU 替换配置 | 无锁热加载，无读写竞争 |
| `go:embed` 内嵌前端 dist/ | 单二进制部署，无需额外静态文件 |
| 滑动窗口重试预算 | 防止重试风暴拖垮上游服务 |

## 与 CC Switch 的边界

本项目**不做** CC Switch 已成熟实现的功能（多供应商 Failover / 协议转换 / API Key 管理 / 熔断器等），而是作为其**重试规则增强层**，部署在 Agent 与 CC Switch 之间。

## License

MIT
