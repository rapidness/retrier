import React, { useEffect, useState } from 'react';
import { Card, Form, Switch, InputNumber, Input, Select, Button, message, Row, Col, Tag, Divider } from 'antd';
import { api } from '../api/client';
import type { Config, LoggingConfig } from '../types/config';

const outputOptions = [
  { value: 'file', label: '文件' },
  { value: 'stdout', label: '标准输出' },
  { value: 'both', label: '两者' },
];

const Logging: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    api.getConfig().then(c => {
      setConfig(c);
      form.setFieldsValue(c.logging);
    }).catch(e => message.error('加载失败: ' + e.message));
  }, []);

  const handleSave = async () => {
    if (!config) return;
    try {
      const values = await form.validateFields();
      const newLogging: LoggingConfig = { ...config.logging, ...values };
      await api.updateLogging(newLogging);
      setConfig({ ...config, logging: newLogging });
      message.success('日志配置已保存');
    } catch (e: any) {
      if (e.errorFields) return;
      message.error('保存失败: ' + e.message);
    }
  };

  /** Switch 即时生效：切换后立即持久化到后端 */
  const handleSwitchToggle = async (field: keyof LoggingConfig, checked: boolean) => {
    if (!config) return;
    const newLogging = { ...config.logging, [field]: checked };
    try {
      await api.updateLogging(newLogging);
      setConfig({ ...config, logging: newLogging });
      form.setFieldValue(field, checked);
    } catch (e: any) {
      // 回滚表单值
      form.setFieldValue(field, !checked);
      message.error('切换失败: ' + e.message);
    }
  };

  return (
    <Card title="日志配置" extra={config && <Tag color={config.logging.enabled ? 'green' : 'default'}>{config.logging.enabled ? 'ON' : 'OFF'}</Tag>}>
      <Form form={form} layout="vertical">
        <Divider>总开关</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label="启用日志" name="enabled" valuePropName="checked">
              <Switch onChange={(v) => handleSwitchToggle('enabled', v)} />
            </Form.Item>
          </Col>
        </Row>

        <Divider>分级记录</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label="记录请求" name="log_requests" valuePropName="checked">
              <Switch onChange={(v) => handleSwitchToggle('log_requests', v)} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label="记录响应" name="log_responses" valuePropName="checked">
              <Switch onChange={(v) => handleSwitchToggle('log_responses', v)} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label="记录重试" name="log_retries" valuePropName="checked">
              <Switch onChange={(v) => handleSwitchToggle('log_retries', v)} />
            </Form.Item>
          </Col>
        </Row>

        <Divider>输出配置</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label="输出位置" name="output">
              <Select options={outputOptions} />
            </Form.Item>
          </Col>
          <Col span={16}>
            <Form.Item label="文件路径" name="file_path">
              <Input placeholder="./logs/retry-middleware.log" />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label="Body 最大记录 (字节)" name="max_body_size">
              <InputNumber min={0} step={1024} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label="单文件上限 (MB)" name="max_file_size">
              <InputNumber min={1} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label="保留文件数" name="max_files">
              <InputNumber min={1} />
            </Form.Item>
          </Col>
        </Row>

        <Button type="primary" onClick={handleSave} style={{ marginTop: 16 }}>保存配置</Button>
      </Form>
    </Card>
  );
};

export default Logging;
