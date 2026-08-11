# 智能重试中间层 —— 需求文档

**文档版本**：v1.1  
**撰写日期**：2026-08-07  


## 1. 项目背景

### 1.1 为什么要做这个中间层？

当前 AI Agent 调用 LLM API 时，**重试逻辑基本"硬编码"在 Agent 内部**，带来了几个典型问题：

- **规则僵化**：多数 Agent 只对 HTTP 429/5xx 做简单重试，无法处理**业务级错误**（例如响应码 700 表示"配额耗尽"，但 HTTP 状态码仍是 200）。
- **决策权不在网关**：CC Switch 类代理虽然具备重试和故障转移能力，但其"重试条件"是**写死在代码里的**（基于 HTTP 状态码分类），不支持用户自定义规则。
- **Agent 被迫感知错误**：当遇到可恢复错误时，Agent 会收到异常并自行处理，导致代码臃肿且重复。

### 1.2 本中间层的定位

> **不做另一个 CC Switch**，而是做 **CC Switch 的"重试规则增强插件"**，或者作为 **Agent 与 CC Switch 之间的规则决策层**。

核心职责：
1. **规则可配置**：允许用户通过配置文件或 UI 定义"什么情况下重试"。
2. **响应体深度匹配**：不仅看 HTTP 状态码，还能**解析 JSON 响应体**，根据字段值决定是否重试。
3. **短路重试**：匹配到重试规则后，**由中间层内部完成重试**，仅将最终结果（成功或彻底失败）返回给 Agent。
4. **日志按需开启**：提供请求/响应日志的开关，日常运行时关闭以节省存储，排障时一键开启。


## 2. 核心需求

### 2.1 规则配置能力

用户可以通过 **YAML/JSON 配置文件** 或 **可视化界面** 自定义重试规则。

| 匹配维度 | 支持的匹配方式 |
|---------|--------------|
| **HTTP 状态码** | 精确匹配（如 `429`）、列表匹配（如 `[429, 502, 503]`）、范围匹配（`5xx` 表示所有 500-599） |
| **响应头** | 匹配特定 Header 是否存在或值等于某字符串 |
| **响应体 JSON 字段** | 支持 JSONPath / JMESPath 表达式，如 `$.error.code == 700` |
| **响应体文本** | 包含（contains）、正则匹配（regex） |
| **逻辑组合** | 支持 AND / OR / NOT 组合多个匹配条件 |

### 2.2 重试策略配置

每条规则可独立配置重试行为：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| `max_attempts` | 最大重试次数（含首次） | `3` |
| `backoff_strategy` | 退避策略 | `fixed` / `exponential` / `linear` |
| `initial_delay` | 首次重试前等待时间（毫秒） | `1000` |
| `multiplier` | 指数退避的乘数因子 | `2.0` |
| `max_delay` | 单次最大等待上限（毫秒） | `30000` |
| `jitter` | 是否添加随机抖动，避免惊群效应 | `true` / `false` |

### 2.3 短路行为（核心价值）

- 当请求的 **响应匹配某条重试规则** 时，中间层**直接在内部启动重试循环**，**不向 Agent 返回任何错误信息**。
- Agent 视角：仅仅感觉这一次请求"慢了一点"，最终收到的是成功响应或明确失败。
- 仅当 **所有重试次数耗尽且仍然失败** 时，中间层才向 Agent 返回最终错误（可配置为返回原始错误或包装后的友好提示）。

### 2.4 🔥 新增：请求/响应日志开关

提供独立的日志开关，支持精细控制日志记录行为：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `logging.enabled` | 总开关，是否记录请求/响应日志 | `false` |
| `logging.log_requests` | 是否记录请求体（含 Headers、Body） | `false` |
| `logging.log_responses` | 是否记录响应体（含 Headers、Body） | `false` |
| `logging.log_retries` | 是否记录重试事件（含每次重试的触发原因、延迟、结果） | `true` |
| `logging.max_body_size` | 单条日志中 Body 最大记录长度（字节），超出截断 | `10240` |
| `logging.output` | 日志输出位置 | `file` / `stdout` / `both` |
| `logging.file_path` | 日志文件路径 | `./logs/middleware.log` |
| `logging.max_file_size` | 单个日志文件最大大小（MB），超出自动轮转 | `100` |
| `logging.max_files` | 日志文件保留数量 | `10` |

#### 设计原则

- **默认关闭**：日常运行时 `enabled: false`，完全不写入日志文件，**零存储开销**，对性能无影响。
- **按需开启**：当需要排查问题时，修改配置将 `enabled: true` 即可实时生效（热加载），无需重启进程。
- **分级记录**：区分请求体、响应体、重试事件，避免无关信息占用空间。
- **自动轮转**：防止日志文件无限膨胀。

#### 配置示例

```yaml
logging:
  # 日常关闭，排障时改为 true
  enabled: false
  
  # 排障时可选择性开启
  log_requests: true
  log_responses: true
  log_retries: true
  
  max_body_size: 10240
  
  output: file
  file_path: "./logs/retry-middleware.log"
  max_file_size: 100
  max_files: 10
```

#### 日志输出示例

```json
{
  "timestamp": "2026-08-07T14:32:11.123Z",
  "request_id": "req_abc123",
  "event": "request",
  "method": "POST",
  "url": "https://api.deepseek.com/v1/chat/completions",
  "headers": {"content-type": "application/json", "authorization": "Bearer sk-***"},
  "body": "{\"model\":\"deepseek-chat\",\"messages\":[...]}",
  "body_size": 245
}
```

```json
{
  "timestamp": "2026-08-07T14:32:13.456Z",
  "request_id": "req_abc123",
  "event": "retry_triggered",
  "attempt": 1,
  "trigger_rule": "retry-on-code-700",
  "response_code": 700,
  "response_body": "{\"code\":700,\"msg\":\"quota exceeded\"}",
  "next_delay_ms": 2000
}
```

```json
{
  "timestamp": "2026-08-07T14:32:17.789Z",
  "request_id": "req_abc123",
  "event": "response",
  "attempt": 3,
  "success": true,
  "status_code": 200,
  "response_body_size": 1024,
  "total_elapsed_ms": 6678
}
```


## 3. 配置示例（完整 YAML）

```yaml
# ============================================================
# 智能重试中间层 —— 完整配置文件
# ============================================================

# ---------- 日志配置 ----------
logging:
  enabled: false                    # 总开关，日常关闭
  log_requests: true
  log_responses: true
  log_retries: true
  max_body_size: 10240
  output: file
  file_path: "./logs/retry-middleware.log"
  max_file_size: 100
  max_files: 10

# ---------- 重试规则 ----------
rules:
  # 规则1：响应码700 → 立即重试
  - name: "retry-on-code-700"
    description: "配额耗尽或临时业务错误，重试3次"
    match:
      http_status: 200
      json_path: "$.code"
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

  # 规则2：限流 + 服务端错误
  - name: "retry-on-429-and-5xx"
    description: "限流或服务端临时故障"
    match:
      http_status:
        - 429
        - 502
        - 503
        - 504
    action:
      max_attempts: 5
      backoff:
        strategy: "exponential"
        initial_delay: 1000
        multiplier: 1.5

  # 规则3：永不重试的请求（直接透传失败）
  - name: "never-retry-on-400"
    match:
      http_status: 400
    action:
      max_attempts: 1
      skip_retry: true

# ---------- 代理配置 ----------
proxy:
  listen: "127.0.0.1:15722"
  upstream: "http://127.0.0.1:15721"   # 指向 CC Switch，也可直接指向 LLM API
  timeout_seconds: 120

# ---------- 全局重试预算（可选） ----------
rate_limit:
  retry_burst: 100           # 每分钟最大重试次数
  retry_burst_window: 60     # 窗口秒数
```


## 4. 集成方式

### 4.1 部署位置

本中间层部署在 **Agent 与上游 LLM API（或 CC Switch）之间**：

```
Agent (AI Agent) --> Middleware (本中间层: 重试规则引擎) --> Upstream (上游 LLM API 或 CC Switch 代理)
Upstream --> Middleware -->|仅最终结果| Agent
```

### 4.2 两种接入模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **透明代理模式** | 监听本地端口（如 `127.0.0.1:15722`），Agent 只需修改 `base_url` | 不想改动 Agent 代码 |
| **SDK/库模式** | 以 Python/Go/Node.js 库的形式嵌入 Agent 代码中 | 希望更轻量，无额外网络开销 |

**优先推荐透明代理模式**，与 CC Switch 保持一致的体验。


## 5. 与 CC Switch 的边界划分

> 本中间层**不做**以下 CC Switch 已成熟实现的功能：

| 功能 | 归属 |
|------|------|
| 多供应商故障转移（Failover） | CC Switch |
| 协议转换（Responses API ↔ Chat Completions） | CC Switch |
| 多供应商 API Key 管理 | CC Switch |
| 请求用量统计与可视化仪表盘 | CC Switch |
| 隐私过滤（脱敏） | CC Switch |
| 熔断器（Circuit Breaker） | CC Switch |


## 6. 非功能性需求

| 项目 | 要求 |
|------|------|
| **额外延迟** | 无重试时，代理转发增加延迟 < 3ms |
| **资源占用** | 内存 < 50MB，CPU < 3% |
| **日志写入影响** | 日志关闭时 **零 I/O 开销**；开启后写入延迟 < 1ms/条 |
| **配置热加载** | 修改规则或日志开关后 5 秒内生效，无需重启 |
| **日志轮转** | 支持按大小自动轮转，保留最近 N 个文件 |
| **部署方式** | 提供 Docker 镜像 + 各平台独立二进制（Linux/macOS/Windows） |
| **可观测性** | 暴露 Prometheus 指标（重试总数、按规则统计、成功率、日志写入量） |


## 7. 验收标准

| 编号 | 验收项 |
|------|--------|
| AC-01 | 配置一条规则匹配 `$.code == 700`，模拟 API 返回该错误，中间层自动重试 3 次，**Agent 未收到任何 700 错误** |
| AC-02 | 配置一条规则匹配 HTTP 429，模拟限流，中间层按指数退避策略重试 5 次，最终成功后返回结果 |
| AC-03 | 配置一条规则匹配 `http_status: 400` 且 `skip_retry: true`，模拟 400 错误，中间层直接返回错误，不重试 |
| AC-04 | 修改配置文件（规则或日志开关），5 秒内新配置生效，无需重启进程 |
| AC-05 | **日志默认关闭**，运行 1 小时，日志目录无任何文件生成 |
| AC-06 | **日志开启后**，所有请求/响应/重试事件被正确记录，包含 request_id 串联全链路 |
| AC-07 | 日志文件达到 100MB 后自动轮转，保留最近 10 个文件 |
| AC-08 | 压测：100 QPS 下，日志关闭时代理延迟 < 3ms；日志开启后延迟 < 8ms |


## 8. 开放性问题

| 问题 | 建议 |
|------|------|
| **重试是否应该考虑请求幂等性？** | 对于非幂等的 POST 请求，重试可能导致重复扣费或重复生成。建议在规则中增加 `idempotent_only` 开关，默认 `false`（即不重试非幂等请求），需用户显式开启。 |
| **重试总时长超过 Agent 超时怎么办？** | 提供 `global_timeout` 配置，当重试总耗时超过该值时提前终止并返回错误。 |
| **日志中是否要记录 API Key？** | **绝不**。日志模块应默认脱敏 `Authorization` 和 `api-key` 头，仅保留 `sk-***` 格式。 |


## 9. 附录

### A. 术语表

| 术语 | 解释 |
|------|------|
| **短路重试** | 指中间层匹配到重试规则后，不向上游返回错误，而是内部完成重试，仅向 Agent 返回最终结果。 |
| **惊群效应（Thundering Herd）** | 多个客户端同时触发重试，导致服务端压力瞬间暴增。通过添加随机抖动（Jitter）来缓解。 |
| **日志轮转（Log Rotation）** | 当日志文件达到设定大小后自动归档并新建文件，避免单文件过大。 |

### B. 参考项目

- [CC Switch](https://github.com/cc-switch/cc-switch) — 本项目定位为其补充，而非替代。
- [Tenacity](https://tenacity.readthedocs.io/) — Python 重试库，提供了丰富的重试策略实现参考。
- [logrotate](https://linux.die.net/man/8/logrotate) — Linux 日志轮转工具，本中间件日志轮转机制参考其设计。
