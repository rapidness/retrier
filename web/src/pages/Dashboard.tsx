import React, { useEffect, useState } from 'react';
import { Card, Col, Row, Statistic, Switch, Tag, Typography, Spin, message } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { api } from '../api/client';
import type { Status, Config } from '../types/config';

const Dashboard: React.FC = () => {
  const [status, setStatus] = useState<Status | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchData = async () => {
    try {
      const [s, c] = await Promise.all([api.getStatus(), api.getConfig()]);
      setStatus(s);
      setConfig(c);
    } catch (e: any) {
      message.error('获取状态失败: ' + e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, []);

  const toggleLogging = async (checked: boolean) => {
    if (!config) return;
    try {
      const newLogging = { ...config.logging, enabled: checked };
      await api.updateLogging(newLogging);
      setConfig({ ...config, logging: newLogging });
      message.success(checked ? '日志已开启' : '日志已关闭');
    } catch (e: any) {
      message.error('切换失败: ' + e.message);
    }
  };

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;
  if (!status || !config) return <Typography.Text type="danger">无法加载配置</Typography.Text>;

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic
              title="代理状态"
              value={status.proxy_listening ? '运行中' : '未启动'}
              prefix={status.proxy_listening ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <CloseCircleOutlined style={{ color: '#ff4d4f' }} />}
              valueStyle={{ color: status.proxy_listening ? '#52c41a' : '#ff4d4f', fontSize: 18 }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="上游地址" value={status.upstream} valueStyle={{ fontSize: 14 }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="重试规则" value={status.rules_count} suffix="条" prefix={<SafetyIcon />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="重试总次数"
              value={status.retry_total}
              prefix={<ReloadOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="重试成功"
              value={status.retry_success}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="重试耗尽"
              value={status.retry_exhausted}
              valueStyle={{ color: status.retry_exhausted > 0 ? '#ff4d4f' : undefined }}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="日志开关" extra={<Tag color={config.logging.enabled ? 'green' : 'default'}>{config.logging.enabled ? 'ON' : 'OFF'}</Tag>}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Switch checked={config.logging.enabled} onChange={toggleLogging} />
              <span>{config.logging.enabled ? '日志记录中，排障模式' : '日志关闭，零开销模式'}</span>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

const SafetyIcon: React.FC = () => <ThunderboltOutlined style={{ color: '#1677ff' }} />;

export default Dashboard;
