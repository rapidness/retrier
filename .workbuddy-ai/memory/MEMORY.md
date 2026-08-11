# 智能重试中间层 - 项目记忆

## 技术栈
- Go 语言，net/http/httputil.ReverseProxy 核心
- 第三方库: yaml.v3, fsnotify, ojg/jp (JSONPath), lumberjack, prometheus, uuid, testify

## 项目结构
- `cmd/proxy/main.go` - 入口
- `internal/config/` - 配置加载 + fsnotify 热加载 + SaveAndReload
- `internal/rule/` - 规则引擎（状态码/头部/JSONPath/文本/逻辑组合）
- `internal/retry/` - 退避策略 + 重试预算 + 执行器
- `internal/proxy/` - ReverseProxy + ModifyResponse 钩子
- `internal/logger/` - 零开销 toggle 日志 + 脱敏 + 轮转
- `internal/middleware/` - X-Request-ID 注入
- `internal/metrics/` - Prometheus 自定义 registry
- `internal/admin/` - Web 管理界面 REST API + go:embed 前端
- `web/` - React 18 + Vite + Ant Design 5 + TypeScript 前端工程

## 关键设计决策
- 日志零开销: atomic.Bool 门控，关闭时 1.58ns/op
- Prometheus: 自定义 registry 避免测试中重复注册 panic
- 重试请求重建: 用 http.NewRequestWithContext 而非 Clone，避免 RequestURI 冲突
- 配置热加载: fsnotify 目录监听 + 500ms 防抖 + atomic.Pointer RCU 替换
- Web管理: go:embed 内嵌前端dist/，SaveAndReload 暂停fsnotify→写YAML→恢复
- JSON字段: config 结构体 json tag 与 yaml tag 同名 (snake_case)，API/前端对齐
- 前端构建: cd web && npm run build && cp -r dist ../internal/admin/dist
