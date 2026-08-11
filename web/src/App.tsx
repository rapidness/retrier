import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import AppLayout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Rules from './pages/Rules';
import Logging from './pages/Logging';
import Proxy from './pages/Proxy';

const App: React.FC = () => (
  <ConfigProvider locale={zhCN}>
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/rules" element={<Rules />} />
          <Route path="/logging" element={<Logging />} />
          <Route path="/proxy" element={<Proxy />} />
        </Routes>
      </AppLayout>
    </BrowserRouter>
  </ConfigProvider>
);

export default App;
