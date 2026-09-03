import { Button, Modal, Tooltip } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { AIAssistantWorkspace } from './AIAssistantWorkspace';

export function FloatingResearchAssistant({ workspace }: { workspace: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Tooltip title="打开研究助手" placement="left">
        <Button
          type="primary"
          shape="circle"
          className="research-assistant-fab"
          aria-label="打开研究助手"
          icon={<RobotOutlined />}
          onClick={() => setOpen(true)}
        />
      </Tooltip>
      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={1120}
        centered
        destroyOnHidden={false}
        className="research-assistant-modal"
        title={<div className="quote-dialog-title"><div><strong>研究助手</strong><span>当前工作区：{workspace}</span></div><small>RESEARCH COPILOT</small></div>}
      >
        <AIAssistantWorkspace embedded workspaceContext={workspace} />
      </Modal>
    </>
  );
}
