import type { Config, Rule, LoggingConfig, Status } from '../types/config';

const BASE = '';

async function request<T>(method: string, path: string, body?: any): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${BASE}${path}`, opts);
  if (!res.ok) {
    const err = await res.text();
    throw new Error(`API ${res.status}: ${err}`);
  }
  // 204 No Content 或无 body 时跳过 JSON 解析
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as unknown as T;
  }
  const text = await res.text();
  if (!text) return undefined as unknown as T;
  return JSON.parse(text) as T;
}

export const api = {
  // Config
  getConfig: () => request<Config>('GET', '/api/config'),
  updateConfig: (cfg: Config) => request<Config>('PUT', '/api/config', cfg),

  // Rules
  getRules: () => request<Rule[]>('GET', '/api/rules'),
  addRule: (rule: Rule) => request<Rule>('POST', '/api/rules', rule),
  updateRule: (name: string, rule: Rule) => request<Rule>('PUT', `/api/rules/${encodeURIComponent(name)}`, rule),
  deleteRule: (name: string) => request<void>('DELETE', `/api/rules/${encodeURIComponent(name)}`),

  // Logging
  updateLogging: (logging: LoggingConfig) => request<LoggingConfig>('PUT', '/api/logging', logging),

  // Status
  getStatus: () => request<Status>('GET', '/api/status'),
};
