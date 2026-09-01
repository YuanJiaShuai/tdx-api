import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import App from './App';
import './styles.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#0f5bd8',
          colorSuccess: '#16825d',
          colorError: '#c73f3f',
          colorWarning: '#b77713',
          colorText: '#102033',
          colorBgLayout: '#eef2f8',
          colorBgContainer: '#ffffff',
          colorBorder: '#d7e0ec',
          colorBorderSecondary: '#e6edf5',
          borderRadius: 8,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif'
        },
        components: {
          Button: {
            fontWeight: 700,
            controlHeight: 38
          },
          Card: {
            headerFontSize: 16,
            colorBorderSecondary: '#d7e0ec'
          },
          Table: {
            headerBg: '#f7f9fc',
            headerColor: '#5f6f82',
            rowHoverBg: '#f8fbff',
            borderColor: '#d7e0ec'
          },
          Input: {
            activeBorderColor: '#0f5bd8',
            hoverBorderColor: '#86aef1'
          },
          Select: {
            activeBorderColor: '#0f5bd8',
            hoverBorderColor: '#86aef1'
          },
          Tabs: {
            inkBarColor: '#0f5bd8',
            itemColor: '#5f6f82',
            itemSelectedColor: '#0a3e9a'
          }
        }
      }}
    >
      <App />
    </ConfigProvider>
  </React.StrictMode>
);
