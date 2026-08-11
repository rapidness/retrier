import React, { useEffect, useState } from 'react';
import { Table, Button, Tag, Popconfirm, Space, message, Drawer, Form, Input, InputNumber, Select, Switch, Card, Divider } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { api } from '../api/client';
import type { Rule, Config, MatchSpec, BackoffSpec, ActionSpec } from '../types/config';

const strategyOptions = [
  { value: 'fixed', label: 'Fixed (固定)' },
  { value: 'exponential', label: 'Exponential (指数)' },
  { value: 'linear', label: 'Linear (线性)' },
];

const operatorOptions = [
  { value: '==', label: '== (等于)' },
  { value: '!=', label: '!= (不等于)' },
  { value: '>', label: '> (大于)' },
  { value: '<', label: '< (小于)' },
  { value: '>=', label: '>= (大于等于)' },
  { value: '<=', label: '<= (小于?于等于)' },
  { value: 'contains', label: 'contains (包含)' },
];

const Rules: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [form] = Form.useForm();

  const fetchConfig = async () => {
    try {
      const c = await api.getConfig();
      setConfig(c);
    } catch (e: any) {
      message.error('获取配置失败: ' + e.message);
    }
  };

  useEffect(() => { fetchConfig(); }, []);

  const handleDelete = async (name: string) => {
    if (!config) return;
    try {
      await api.deleteRule(name);
      message.success(`规则 "${name}" 已删除`);
      fetchConfig();
    } catch (e: any) {
      message.error('删除失败: ' + e.message);
    }
  };

  const openEditor = (rule?: Rule) => {
    setEditingRule(rule ?? null);
    if (rule) {
      form.setFieldsValue({
        name: rule.name,
        description: rule.description,
        http_status_type: typeof rule.match.http_status === 'string' ? 'range' : Array.isArray(rule.match.http_status) ? 'list' : 'single',
        http_status: rule.match.http_status,
        json_path: rule.match.json_path_match?.path ?? '',
        json_operator: rule.match.json_path_match?.operator ?? '==',
        json_value: rule.match.json_path_match?.value ?? '',
        text_contains: rule.match.text_match?.contains ?? '',
        text_regex: rule.match.text_match?.regex ?? '',
        max_attempts: rule.action.max_attempts,
        skip_retry: rule.action.skip_retry,
        idempotent_only: rule.action.idempotent_only,
        strategy: rule.action.backoff.strategy,
        initial_delay: rule.action.backoff.initial_delay,
        multiplier: rule.action.backoff.multiplier,
        max_delay: rule.action.backoff.max_delay,
        jitter: rule.action.backoff.jitter,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({
        http_status_type: 'single',
        strategy: 'exponential',
        initial_delay: 1000,
        multiplier: 2.0,
        max_delay: 30000,
        max_attempts: 3,
        jitter: true,
      });
    }
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    if (!config) return;
    try {
      const values = await form.validateFields();
      const rule = buildRule(values);

      const newRules = editingRule
        ? config.rules.map(r => r.name === editingRule.name ? rule : r)
        : [...config.rules, rule];

      await api.updateConfig({ ...config, rules: newRules });
      message.success(editingRule ? '规则已更新' : '规则已添加');
      setDrawerOpen(false);
      fetchConfig();
    } catch (e: any) {
      if (e.errorFields) return; // form validation
      message.error('保存失败: ' + e.message);
    }
  };

  const buildRule = (v: any): Rule => {
    let httpStatus: any = v.http_status;
    if (v.http_status_type === 'range') httpStatus = String(v.http_status);
    else if (v.http_status_type === 'list' && typeof v.http_status === 'string') {
      httpStatus = v.http_status.split(',').map((s: string) => parseInt(s.trim())).filter((n: number) => !isNaN(n));
    }

    const match: MatchSpec = { http_status: httpStatus };
    if (v.json_path) match.json_path_match = { path: v.json_path, operator: v.json_operator ?? '==', value: v.json_value };
    if (v.text_contains || v.text_regex) match.text_match = { contains: v.text_contains ?? '', regex: v.text_regex ?? '' };

    const backoff: BackoffSpec = {
      strategy: v.strategy ?? 'exponential',
      initial_delay: v.initial_delay ?? 1000,
      multiplier: v.multiplier ?? 2.0,
      max_delay: v.max_delay ?? 30000,
      jitter: v.jitter ?? false,
    };

    const action: ActionSpec = {
      max_attempts: v.max_attempts ?? 3,
      skip_retry: v.skip_retry ?? false,
      backoff,
      idempotent_only: v.idempotent_only ?? false,
    };

    return { name: v.name, description: v.description ?? '', match, action };
  };

  const matchSummary = (match: MatchSpec): string => {
    const parts: string[] = [];
    if (match.http_status !== undefined && match.http_status !== null) parts.push(`状态码: ${JSON.stringify(match.http_status)}`);
    if (match.json_path_match) parts.push(`JSONPath: ${match.json_path_match.path} ${match.json_path_match.operator} ${match.json_path_match.value}`);
    if (match.text_match?.contains) parts.push(`包含: ${match.text_match.contains}`);
    if (match.text_match?.regex) parts.push(`正则: ${match.text_match.regex}`);
    return parts.join(' ∧ ') || '无匹配条件';
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (n: string) => <Tag color="blue">{n}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '匹配条件', key: 'match', render: (_: any, r: Rule) => <span style={{ fontSize: 12 }}>{matchSummary(r.match)}</span> },
    { title: '最大重试', key: 'attempts', render: (_: any, r: Rule) => r.action.skip_retry ? <Tag color="red">不重试</Tag> : r.action.max_attempts },
    { title: '退避策略', key: 'backoff', render: (_: any, r: Rule) => <Tag>{r.action.backoff.strategy}</Tag> },
    {
      title: '操作', key: 'action', width: 120,
      render: (_: any, r: Rule) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => openEditor(r)} />
          <Popconfirm title={`确定删除规则 "${r.name}"？`} onConfirm={() => handleDelete(r.name)}>
            <Button type="link" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()} style={{ marginBottom: 16 }}>
          新增规则
        </Button>
        <Table
          columns={columns}
          dataSource={config?.rules ?? []}
          rowKey="name"
          pagination={false}
          size="middle"
        />
      </Card>

      <Drawer
        title={editingRule ? `编辑规则: ${editingRule.name}` : '新增规则'}
        width={520}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={<Button type="primary" onClick={handleSave}>保存</Button>}
      >
        <Form form={form} layout="vertical">
          <Form.Item label="规则名称" name="name" rules={[{ required: true }]}>
            <Input placeholder="retry-on-code-700" disabled={!!editingRule} />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input placeholder="配额耗尽或临时业务错误，重试3次" />
          </Form.Item>

          <Divider>匹配条件</Divider>

          <Form.Item label="HTTP 状态码类型" name="http_status_type">
            <Select options={[{ value: 'single', label: '精确匹配' }, { value: 'list', label: '列表 (逗号分隔)' }, { value: 'range', label: '范围 (如 5xx)' }]} />
          </Form.Item>
          <Form.Item label="HTTP 状态码" name="http_status">
            <Input placeholder="429 或 429,502,503 或 5xx" />
          </Form.Item>

          {/* JSONPath 匹配 */}
          <Form.Item label="JSONPath 路径" name="json_path">
            <Input placeholder="$.code" />
          </Form.Item>
          <Form.Item label="运算符" name="json_operator">
            <Select options={operatorOptions} />
          </Form.Item>
          <Form.Item label="期望值" name="json_value">
            <Input placeholder="700" />
          </Form.Item>

          <Divider>文本匹配</Divider>
          <Form.Item label="包含 (contains)" name="text_contains">
            <Input placeholder="error" />
          </Form.Item>
          <Form.Item label="正则 (regex)" name="text_regex">
            <Input placeholder="error_code_\d+" />
          </Form.Item>

          <Divider>退避策略</Divider>
          <Form.Item label="策略" name="strategy">
            <Select options={strategyOptions} />
          </Form.Item>
          <Form.Item label="初始延迟 (ms)" name="initial_delay">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item label="乘数" name="multiplier">
            <InputNumber min={1} step={0.5} />
          </Form.Item>
          <Form.Item label="最大延迟 (ms)" name="max_delay">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item label="随机抖动 (Jitter)" name="jitter" valuePropName="checked">
            <Switch />
          </Form.Item>

          {/* 高级 */}
          <Divider>高级</Divider>
          <Form.Item label="最大重试次数" name="max_attempts">
            <InputNumber min={1} max={20} />
          </Form.Item>
          <Form.Item label="跳过重试 (直接透传)" name="skip_retry" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="仅幂等请求" name="idempotent_only" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Drawer>
    </div>
  );
};

export default Rules;
