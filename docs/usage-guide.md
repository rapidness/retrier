# 智能重试中间层 —— 使用手册

## 1. 快速开始

### 1.1 前置条件

- Go 1.22+
- Node.js 18+（仅前端开发时需要）

### 1.2 构建与启动

```bash
# 构建 Go 二进制（前端已内嵌）
go build -o ./bin/proxy.exe ./cmd/proxy

# 启动服务
./bin/proxy.exe -config ./configs/config.yaml

# 指定端口
./bin/proxy.exe \
  -config ./configs/config.yaml \
  -admin-addr :15723 \
  -metrics-addr :9090
```

### 1.3 启动后端口

| 端口 | 服务 | 说明 |
|------|------|------|
| `15722` | **代理** | AI Agent 连接此端口，透明转发到上游 |
| `15723` | **管理界面** | 浏览器打开 http://localhost:15723 |
| `9090` | **Prometheus** | 指标端点 http://localhost:9090/metrics |

### 1.4 Agent 接入

将 AI Agent 的 `base_url` 改为代理地址即可，无需改动代码：

```python
# 之前：直连 LLM API
client = OpenAI(base_url="https://api.deepseek.com/v1", api_key="sk-xxx")

# 现在：经过重试中间层
client = OpenAI(base_url="http://127.0.0.1:15722/v1", api_key="sk-xxx")
```

---

## 2. 配置说明

配置文件路径：`configs/config.yaml`

### 2.1 代理配置

```yaml
proxy:
  listen: "127.0.0.1:15722"       # 代理监听地址
  upstream: "http://127.0.0.1:15721"  # 上游 LLM API 或 CC Switch
  timeout_seconds: 120              # 单次请求超时
  global_timeout: 60000             # 重试总时长上限（毫秒）
```

### 2.2 日志配置

```yaml
logging:
  enabled: false             # 总开关，日常关闭零开销
  log_requests: true         # 记录请求体
  log_responses: true        # 记录响应体
  log_retries: true          # 记录重试事件
  max_body_size: 10240       # Body 最大记录长度（字节）
  output: file               # 输出位置：file / stdout / both
  file_path: "./logs/retry-middleware.log"
  max_file_size: 100         # 单文件上限（MB）
  max_files: 10              # 保留轮转文件数
```

**日常运行**：`enabled: false`，完全不写日志，零 I/O 开销。

**排障时**：改为 `true`，热加载即时生效，无需重启。

### 2.3 重试规则

每条规则包含 **匹配条件** + **重试动作**：

```yaml
rules:
  - name: "retry-on-code-700"
    description: "配额耗尽或临时业务错误，重试3次"
    match:
      http_status: 200
      json_path_match:
        path: "$.code"
        operator: "=="
        value: 700
    action:
      max_attempts: 3
      backoff:
        strategy: "exponential"
        initial_delay: 2000
        multiplier: 2.0
        max_delay: 30000
        jitter: true
```

#### 匹配维度

| 维度 | 配置字段 | 示例 |
|------|---------|------|
| HTTP 状态码 | `match.http_status` | `429`、`[429, 502, 503]`、`"5xx"` |
| 响应头 | `match.headers` | `[{name: "X-RateLimit", value: "true"}]` |
| JSON 字段 | `match.json_path_match` | `path: "$.code", operator: "==", value: 700` |
| 文本包含 | `match.text_match.contains` | `"quota exceeded"` |
| 正则匹配 | `match.text_match.regex` | `"error_code_\\d+"` |
| 逻辑组合 | `match.logic` | `and: [...]` / `or: [...]` / `not: {...}` |

#### JSONPath 运算符

| 运算符 | 含义 | 示例 |
|--------|------|------|
| `==` | 等于 | `$.code == 700` |
| `!=` | 不等于 | `$.code != 0` |
| `>` / `<` / `>=` / `<=` | 数值比较 | `$.retry_after > 0` |
| `contains` | 包含子串 | `$.msg contains "error"` |

#### 退避策略

| 策略 | 计算公式 | 适用场景 |
|------|---------|---------|
| `fixed` | `delay = initial_delay` | 固定间隔 |
| `exponential` | `delay = initial_delay × multiplier^(attempt-1)` | 通用推荐 |
| `linear` | `delay = initial_delay × attempt` | 温和增长 |

**jitter**：添加随机抖动（±25%），防止多个客户端同时重试造成惊群效应。

#### 跳过重试

```yaml
- name: "never-retry-on-400"
  match:
    http_status: 400
  action:
    max_attempts: 1
    skip_retry: true    # 匹配后直接返回，不重试
```

### 2.4 重试预算

```yaml
rate_limit:
  retry_burst: 100          # 窗口内最大重试次数
  retry_burst_window: 60    # 窗口秒数
```

防止重试风暴耗尽上游配额。

---

## 3. Web 管理界面

浏览器打开 **http://localhost:15723**

### 3.1 仪表盘

- 代理运行状态、上游地址、规则数量
- 重试统计：总次数、成功、耗尽
- **日志一键开关**：Switch 组件即时切换

### 3.2 重试规则管理

- 表格展示所有规则：名称、匹配条件摘要、重试次数、退避策略
- 新增/编辑：Drawer 抽屉表单，完整配置匹配条件 + 退避策略
- 删除：二次确认后移除

### 3.3 日志配置

- 总开关 + 分级开关（request / response / retry）
- 文件路径、轮转参数

### 3.4 代理配置

- 监听地址、上游地址、超时、重试预算

### 3.5 热加载

| 配置项 | 是否需要重启 |
|--------|-------------|
| 重试规则 | ❌ 即时生效 |
| 日志开关 | ❌ 即时生效 |
| 代理监听端口 | ✅ 需重启 |
| 上游地址 | ⚠️ 建议重启 |

---

## 4. REST API

管理界面后端提供以下 API，可直接用 curl 调用：

### 4.1 获取配置

```bash
curl http://localhost:15723/api/config
```

### 4.2 更新完整配置

```bash
curl -X PUT http://localhost:15723/api/config \
  -H "Content-Type: application/json" \
  -d @config.json
```

### 4.3 规则 CRUD

```bash
# 列出所有规则
curl http://localhost:15723/api/rules

# 新增规则
curl -X POST http://localhost:15723/api/rules \
  -H "Content-Type: application/json" \
  -d '{"name":"new-rule","description":"...","match":{...},"action":{...}}'

# 更新规则
curl -X PUT http://localhost:15723/api/rules/old-name \
  -H "Content-Type: application/json" \
  -d '{"name":"new-name",...}'

# 删除规则
curl -X DELETE http://localhost:15723/api/rules/rule-name
```

### 4.4 日志控制

```bash
# 开启日志
curl -X PUT http://localhost:15723/api/logging \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"log_requests":true,"log_responses":true,"log_retries":true,...}'

# 关闭日志
curl -X PUT http://localhost:15723/api/logging \
  -H "Content-Type: application/json" \
  -d '{"enabled":false,...}'
```

### 4.5 运行状态

```bash
curl http://localhost:15723/api/status
```

---

## 5. 开发调试

### 5.1 项目结构

```
proxy-api/
├── cmd/proxy/main.go              # 入口
├── internal/
│   ├── config/                     # 配置加载 + 热加载
│   ├── rule/                       # 规则引擎（5种匹配器 + 逻辑组合）
│   ├── retry/                      # 退避策略 + 预算 + 执行器
│   ├── proxy/                      # ReverseProxy + ModifyResponse
│   ├── logger/                     # 零开销 toggle 日志 + 脱敏 + 轮转
│   ├── middleware/                  # X-Request-ID 注入
│   ├── metrics/                    # Prometheus 自定义 registry
│   └── admin/                      # 管理界面 REST API + go:embed
├── web/                            # React 18 + Vite + Ant Design 前端
├── configs/config.yaml             # 配置文件
└── Makefile
```

### 5.2 运行测试

```bash
# 单元 + 集成测试
go test ./internal/... -v

# 带覆盖率
go test ./internal/... -cover

# 基准测试
go test ./internal/... -bench=. -benchmem
```

### 5.3 前端开发

```bash
cd web

# 安装依赖
npm install

# 开发模式（Vite dev server，API 代理到 :15723）
npm run dev

# 构建生产版本
npm run build
```

**前端开发流程**：

1. 先启动 Go 后端（`go run ./cmd/proxy`）
2. 再启动 Vite dev server（`npm run dev`，默认 :5173）
3. Vite 自动将 `/api` 请求代理到 `:15723`
4. 浏览器打开 `http://localhost:5173`

### 5.4 前端构建产物内嵌

修改前端后需要重新构建并嵌入 Go 二进制：

```bash
# 1. 构建前端
cd web && npm run build

# 2. 复制到 admin 包的 dist 目录
cp -r dist ../internal/admin/dist

# 3. 重新编译 Go（go:embed 自动嵌入）
cd .. && go build -o ./bin/proxy.exe ./cmd/proxy
```

### 5.5 热加载调试

运行中修改 `configs/config.yaml`，观察日志输出：

```
[config] reloaded: 3 rules (was 2), logging.enabled=true (was false)
```

---

## 6. 构建部署

### 6.1 本地构建

```bash
# 使用 Makefile
make build

# 或直接 go build
go build -ldflags "-s -w" -o ./bin/retry-middleware ./cmd/proxy
```

### 6.2 跨平台构建

```bash
make build-all

# 产物：
# ./bin/retry-middleware-linux-amd64
# ./bin/retry-middleware-darwin-amd64
# ./bin/retry-middleware-darwin-arm64
# ./bin/retry-middleware-windows-amd64.exe
```

### 6.3 Docker 部署

```bash
# 构建镜像
docker build -t retry-middleware:latest .

# 运行
docker run -d \
  -p 15722:15722 \
  -p 15723:15723 \
  -p 9090:9090 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/logs:/app/logs \
  retry-middleware:latest
```

### 6.4 Dockerfile 说明

- 两阶段构建：`golang:1.23-alpine` 编译 → `alpine:3.19` 运行
- 最终镜像仅包含二进制 + 配置，体积 ~20MB
- 暴露端口：15722（代理）、15723（管理）、9090（指标）

---

## 7. 可观测性

### 7.1 Prometheus 指标

访问 `http://localhost:9090/metrics`，关键指标：

| 指标 | 类型 | 含义 |
|------|------|------|
| `retry_middleware_retry_total` | Counter | 重试总次数 |
| `retry_middleware_retry_by_rule{rule}` | Counter | 按规则统计 |
| `retry_middleware_retry_success_total` | Counter | 重试后成功 |
| `retry_middleware_retry_exhausted_total` | Counter | 重试耗尽 |
| `retry_middleware_retry_delay_seconds` | Histogram | 退避延迟分布 |
| `retry_middleware_request_duration_seconds` | Histogram | 端到端延迟 |
| `retry_middleware_active_requests` | Gauge | 当前在途请求数 |
| `retry_middleware_log_entries_written_total` | Counter | 日志写入条数 |

### 7.2 日志格式

开启后每条日志为 JSON，通过 `request_id` 串联全链路：

```json
{"timestamp":"...","request_id":"req_abc","event":"request","method":"POST","url":"...","headers":{...},"body":"..."}
{"timestamp":"...","request_id":"req_abc","event":"retry_triggered","attempt":1,"trigger_rule":"retry-on-code-700","response_code":700,"next_delay_ms":2000}
{"timestamp":"...","request_id":"req_abc","event":"response","attempt":3,"success":true,"status_code":200,"total_elapsed_ms":6678}
```

**API Key 自动脱敏**：`Authorization: Bearer sk-abc123` → `Bearer sk-***`

---

## 8. 常见场景

### 场景1：CC Switch + 重试中间层

```
Agent → :15722 (本中间层) → :15721 (CC Switch) → LLM API
```

```yaml
proxy:
  listen: "127.0.0.1:15722"
  upstream: "http://127.0.0.1:15721"
```

### 场景2：直接代理 LLM API

```
Agent → :15722 (本中间层) → https://api.deepseek.com
```

```yaml
proxy:
  listen: "127.0.0.1:15722"
  upstream: "https://api.deepseek.com"
```

### 场景3：排障时临时开启日志

```bash
# 方式1：管理界面 → 仪表盘 → 日志开关
# 方式2：API 调用
curl -X PUT http://localhost:15723/api/logging \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"log_requests":true,"log_responses":true,"log_retries":true,"max_body_size":10240,"output":"file","file_path":"./logs/retry-middleware.log","max_file_size":100,"max_files":10}'

# 排障完毕后关闭
# 同上，enabled: false
```

### 场景4：新增一条重试规则

```bash
curl -X POST http://localhost:15723/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "retry-on-timeout",
    "description": "上游超时重试",
    "match": {"http_status": 504},
    "action": {
      "max_attempts": 3,
      "skip_retry": false,
      "backoff": {"strategy":"exponential","initial_delay":1000,"multiplier":2,"max_delay":10000,"jitter":true},
      "idempotent_only": false
    }
  }'
```

---

## 9. 性能基准

| 场景 | 耗时 | 分配 |
|------|------|------|
| 日志 Toggle **关闭** | 1.58 ns/op | 0 B, 0 allocs |
| 日志 Toggle 开启 | ~13 μs/op | 1526 B, 35 allocs |
| 退避计算（无抖动） | 17 ns/op | 0 B |
| 退避计算（有抖动） | 24 ns/op | 0 B |
| 代理转发（无重试） | < 3 ms | — |

---

## 10. 命令行参数

```
Usage: retry-middleware [options]

Options:
  -config string       配置文件路径 (默认 "./configs/config.yaml")
  -admin-addr string   管理界面监听地址 (默认 ":15723")
  -metrics-addr string Prometheus 指标监听地址 (默认 ":9090")
```
