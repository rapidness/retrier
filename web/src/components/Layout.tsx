import React from 'react';
import { Layout as AntLayout, Menu, Typography } from 'antd';
import {
  DashboardOutlined,
  SafetyCertificateOutlined,
  FileTextOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';

const { Sider, Content, Header } = AntLayout;
const { Title } = Typography;

const menuItems = [
  { key: '/', label: '仪表盘', icon: <DashboardOutlined /> },
  { key: '/rules', label: '重试规则', icon: <SafetyCertificateOutlined /> },
  { key: '/logging', label: '日志配置', icon: <FileTextOutlined /> },
  { key: '/proxy', label: '代理配置', icon: <CloudServerOutlined /> },
];

const AppLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const navigate = useNavigate();
  const location = useLocation();

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider width={200} theme="light" style={{ borderRight: '1px solid #f0f0f0' }}>
        <div style={{ padding: '16px', textAlign: 'center' }}>
          <Title level={5} style={{ margin: 0, color: '#1677ff' }}>
            重试中间层
          </Title>
          <span style={{ fontSize: 12, color: '#999' }}>管理控制台</span>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ border: 'none' }}
        />
      </Sider>
      <AntLayout>
        <Header style={{ background: '#fff', padding: '0 24px', borderBottom: '1px solid #f0f0f0' }}>
          <Title level={4} style={{ margin: '14px 0' }}>
            {menuItems.find(m => m.key === location.pathname)?.label ?? '管理'}
          </Title>
        </Header>
        <Content style={{ padding: 24, background: '#f5f5f5' }}>
          {children}
        </Content>
      </AntLayout>
    </AntLayout>
  );
};

export default AppLayout;
