import React, { useEffect, useState } from 'react';
import { Card, Form, Input, InputNumber, Button, message, Descriptions, Divider } from 'antd';
import { api } from '../api/client';
import type { Config } from '../types/config';

const Proxy: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    api.getConfig().then(c => {
      setConfig(c);
      form.setFieldsValue({
        listen: c.proxy.listen,
        upstream: c.proxy.upstream,
        timeout_seconds: c.proxy.timeout_seconds,
        global_timeout: c.proxy.global_timeout,
        retry_burst: c.rate_limit.retry_burst,
        retry_burst_window: c.rate_limit.retry_burst_window,
      });
    }).catch(e => message.error('加载失败: ' + e.message));
  }, []);

  const handleSave = async () => {
    if (!config) return;
    try {
      const values = await form.validateFields();
      const newCfg: Config = {
        ...config,
        proxy: {
          ...config.proxy,
          listen: values.listen,
          upstream: values.upstream,
          timeout_seconds: values.timeout_seconds,
          global_timeout: values.global_timeout,
        },
        rate_limit: {
          retry_burst: values.retry_burst,
          retry_burst_window: values.retry_burst_window,
        },
      };
      await api.updateConfig(newCfg);
      setConfig(newCfg);
      message.success('代理配置已保存（重启后生效）');
    } catch (e: any) {
      if (e.errorFields) return;
      message.error('保存失败: ' + e.message);
    }
  };

  return (
    <div>
      <Card title="代理配置" style={{ marginBottom: 16 }}>
        <Form form={form} layout="vertical">
          <Form.Item label="监听地址" name="listen" rules={[{ required: true }]}>
            <Input placeholder="127.0.0.1:15722" />
          </Form.Item>
          <Form.Item label="上游地址" name="upstream" rules={[{ required: true }]}>
            <Input placeholder="http://127.0.0.1:15721" />
          </Form.Item>
          <Form.Item label="请求超时 (秒)" name="timeout_seconds">
            <InputNumber min={1} />
          </Form.Item>
          <Form.Item label="全局重试超时 (毫秒)" name="global_timeout">
            <InputNumber min={0} />
          </Form.Item>

          <Divider>重试预算</Divider>
          <Form.Item label="每分钟最大重试次数" name="retry_burst">
            <InputNumber min={1} />
          </Form.Item>
          <Form.Item label="窗口秒数" name="retry_burst_window">
            <InputNumber min={1} />
          </Form.Item>

          <Button type="primary" onClick={handleSave}>保存配置</Button>
        </Form>
      </Card>

      {config && (
        <Card title="当前状态" size="small">
          <Descriptions column={2}>
            <Descriptions.Item label="监听">{config.proxy.listen}</Descriptions.Item>
            <Descriptions.Item label="上游">{config.proxy.upstream}</Descriptions.Item>
            <Descriptions.Item label="超时">{config.proxy.timeout_seconds}s</Descriptions.Item>
            <Descriptions.Item label="全局重试超时">{config.proxy.global_timeout}ms</Descriptions.Item>
          </Descriptions>
        </Card>
      )}
    </div>
  );
};

export default Proxy;
