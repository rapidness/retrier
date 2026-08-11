// TypeScript types matching Go config structs

export interface LoggingConfig {
  enabled: boolean;
  log_requests: boolean;
  log_responses: boolean;
  log_retries: boolean;
  max_body_size: number;
  output: 'file' | 'stdout' | 'both';
  file_path: string;
  max_file_size: number;
  max_files: number;
}

export interface HeaderMatch {
  name: string;
  value: string;
}

export interface JSONPathMatch {
  path: string;
  operator: string;
  value: any;
}

export interface TextMatch {
  contains: string;
  regex: string;
}

export interface MatchSpec {
  http_status?: number | number[] | string;
  headers?: HeaderMatch[];
  json_path_match?: JSONPathMatch;
  text_match?: TextMatch;
  logic?: LogicMatch;
}

export interface LogicMatch {
  and?: MatchSpec[];
  or?: MatchSpec[];
  not?: MatchSpec;
}

export interface BackoffSpec {
  strategy: 'fixed' | 'exponential' | 'linear';
  initial_delay: number;
  multiplier: number;
  max_delay: number;
  jitter: boolean;
}

export interface ActionSpec {
  max_attempts: number;
  skip_retry: boolean;
  backoff: BackoffSpec;
  idempotent_only: boolean;
}

export interface Rule {
  name: string;
  description: string;
  match: MatchSpec;
  action: ActionSpec;
}

export interface ProxyConfig {
  listen: string;
  upstream: string;
  timeout_seconds: number;
  global_timeout: number;
}

export interface RateLimitConfig {
  retry_burst: number;
  retry_burst_window: number;
}

export interface Config {
  logging: LoggingConfig;
  rules: Rule[];
  proxy: ProxyConfig;
  rate_limit: RateLimitConfig;
}

export interface Status {
  proxy_listening: string;
  upstream: string;
  rules_count: number;
  logging_enabled: boolean;
  retry_total: number;
  retry_success: number;
  retry_exhausted: number;
}
