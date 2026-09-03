import {
  Button,
  Card,
  Empty,
  Input,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message
} from 'antd';
import {
  ClearOutlined,
  CopyOutlined,
  ReloadOutlined,
  RobotOutlined,
  SendOutlined,
  StopOutlined,
  UserOutlined
} from '@ant-design/icons';
import { useEffect, useMemo, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { apiFetch } from '../lib/api';
import type { AICredential } from '../types';

const { Text } = Typography;

type ChatRole = 'user' | 'assistant';

interface ChatMessage {
  id: string;
  role: ChatRole;
  content: string;
  reasoning?: string;
  time: string;
  question?: string;
  toolCalls?: string[];
}

interface StreamEvent {
  content?: string;
  reasoning_content?: string;
  tool_calls?: Array<{
    function?: {
      name?: string;
      arguments?: string;
    };
  }>;
}

interface AnalysisContext {
  generated_at?: string;
  items?: unknown[];
}

const SESSION_STORAGE_KEY = 'tdx-ai-assistant-session';
const MODEL_STORAGE_KEY = 'tdx-ai-assistant-model';
const DEFAULT_VISIBLE_MESSAGES = 20;
const DEFAULT_MEMORY_COUNT = 5;

const DEFAULT_SYSTEM_PROMPT = `你是一个本地 A 股研究工作台的 AI 助手。请帮助用户理解行情、K 线、财务、资讯和交易纪律。

回答要求：
1. 涉及价格、涨跌幅、财务数字和新闻时，只使用输入中提供的实时数据，并明确数据时间。
2. 先说明事实，再给出可能的解释和需要观察的条件。
3. 不提供确定性买卖指令、不承诺收益，明确提示风险。
4. 使用清晰的 Markdown，适当使用标题、列表和表格。
5. 如果输入没有足够数据，直接说明缺少什么，不要猜测。`;

function nowText() {
  return new Date().toLocaleString('zh-CN', { hour12: false });
}

function makeId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function initialMessages(): ChatMessage[] {
  return [{
    id: makeId(),
    role: 'assistant',
    content: '你好，我是你的股票研究助手。可以问我行情、K 线、财务、资讯或复盘问题。',
    reasoning: '',
    time: nowText()
  }];
}

function extractSymbols(text: string) {
  const matches = text.match(/\b(?:60|68|00|30|83|87|43)\d{4}(?:\.(?:SH|SZ|BJ))?\b/gi) || [];
  return [...new Set(matches.map((item) => item.toUpperCase()))];
}

function stringifyContext(context: AnalysisContext) {
  const items = Array.isArray(context.items) ? context.items : [];
  return JSON.stringify({
    generated_at: context.generated_at,
    items
  }, null, 2);
}

function readStoredMessages(): ChatMessage[] {
  try {
    const value = JSON.parse(localStorage.getItem(SESSION_STORAGE_KEY) || 'null');
    if (!Array.isArray(value) || !value.length) return initialMessages();
    return value.filter((item): item is ChatMessage =>
      item && (item.role === 'user' || item.role === 'assistant') && typeof item.content === 'string'
    );
  } catch {
    return initialMessages();
  }
}

function appendStreamEvent(
  event: StreamEvent,
  currentContent: string,
  currentReasoning: string,
  currentToolCalls: string[]
) {
  let content = currentContent;
  let reasoning = currentReasoning;
  const toolCalls = [...currentToolCalls];
  if (event.content) content += event.content;
  if (event.reasoning_content) reasoning += event.reasoning_content;
  for (const call of event.tool_calls || []) {
    const name = call.function?.name || '工具';
    const args = call.function?.arguments || '';
    toolCalls.push(`${name}${args ? `：${args}` : ''}`);
  }
  return { content, reasoning, toolCalls };
}

async function readSSE(
  response: Response,
  onEvent: (event: StreamEvent) => void
) {
  if (!response.body) throw new Error('AI 服务没有返回流式内容');
  const reader = response.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const blocks = buffer.split(/\n\n/);
    buffer = blocks.pop() || '';

    for (const block of blocks) {
      let eventType = 'message';
      const dataLines: string[] = [];
      for (const line of block.split('\n')) {
        if (line.startsWith('event:')) eventType = line.slice(6).trim();
        if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
      }
      const dataText = dataLines.join('\n');
      if (eventType === 'done' || dataText === '[DONE]') return;
      if (eventType === 'error') {
        try {
          const error = JSON.parse(dataText) as { message?: string };
          throw new Error(error.message || 'AI 流式请求失败');
        } catch (error) {
          if (error instanceof Error && error.message !== dataText) throw error;
          throw new Error(dataText || 'AI 流式请求失败');
        }
      }
      if (!dataText) continue;
      try {
        onEvent(JSON.parse(dataText) as StreamEvent);
      } catch {
        // Ignore malformed keep-alive chunks from compatible providers.
      }
    }
  }
}

export function AIAssistantWorkspace() {
  const [credentials, setCredentials] = useState<AICredential[]>([]);
  const [selectedCredentialId, setSelectedCredentialId] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>(readStoredMessages);
  const [inputValue, setInputValue] = useState('');
  const [systemPrompt, setSystemPrompt] = useState(DEFAULT_SYSTEM_PROMPT);
  const [memoryMode, setMemoryMode] = useState(true);
  const [memoryCount, setMemoryCount] = useState(DEFAULT_MEMORY_COUNT);
  const [thinkingMode, setThinkingMode] = useState(true);
  const [marketContextMode, setMarketContextMode] = useState(true);
  const [visibleCount, setVisibleCount] = useState(DEFAULT_VISIBLE_MESSAGES);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [loadingCredentials, setLoadingCredentials] = useState(false);
  const controllerRef = useRef<AbortController | null>(null);

  const selectedCredential = credentials.find((item) => item.id === selectedCredentialId);
  const displayedMessages = messages.slice(Math.max(0, messages.length - visibleCount));
  const hiddenCount = Math.max(0, messages.length - visibleCount);
  const canSend = inputValue.trim().length > 0 && !loading && Boolean(selectedCredential);

  async function loadCredentials() {
    setLoadingCredentials(true);
    try {
      const items = await apiFetch<AICredential[]>('/api/ai/credentials');
      setCredentials(items || []);
      const storedId = localStorage.getItem(MODEL_STORAGE_KEY) || '';
      const nextId = items.some((item) => item.id === storedId && item.enabled !== false)
        ? storedId
        : items.find((item) => item.enabled !== false)?.id || items[0]?.id || '';
      setSelectedCredentialId(nextId);
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'AI 模型加载失败');
    } finally {
      setLoadingCredentials(false);
    }
  }

  useEffect(() => {
    void loadCredentials();
  }, []);

  useEffect(() => {
    localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(messages));
  }, [messages]);

  useEffect(() => {
    if (selectedCredentialId) localStorage.setItem(MODEL_STORAGE_KEY, selectedCredentialId);
  }, [selectedCredentialId]);

  function updateAssistant(index: number, next: Partial<ChatMessage>) {
    setMessages((current) => current.map((item, itemIndex) =>
      itemIndex === index ? { ...item, ...next } : item
    ));
  }

  async function fetchMarketContext(question: string) {
    const symbols = extractSymbols(question);
    if (!marketContextMode || !symbols.length) return '';
    const context = await apiFetch<AnalysisContext>(
      `/api/analysis/context?codes=${encodeURIComponent(symbols.join(','))}&kline_limit=60&news_limit=8`
    );
    return `\n\n【当前项目实时行情上下文】\n${stringifyContext(context)}\n`;
  }

  async function send() {
    if (!canSend || !selectedCredential) {
      if (!selectedCredential) message.warning('请先在“AI 模型”中配置并启用一个模型');
      return;
    }
    const question = inputValue.trim();
    const history = memoryMode
      ? messages.slice(-Math.max(1, memoryCount)).map((item) => ({
        role: item.role,
        content: item.content
      }))
      : [];
    const userMessage: ChatMessage = {
      id: makeId(),
      role: 'user',
      content: question,
      time: nowText()
    };
    const assistantIndex = messages.length + 1;
    const assistantMessage: ChatMessage = {
      id: makeId(),
      role: 'assistant',
      content: '',
      reasoning: '',
      time: nowText(),
      question
    };
    setInputValue('');
    setMessages((current) => [...current, userMessage, assistantMessage]);
    setExpanded((current) => ({ ...current, [assistantMessage.id]: true }));
    setLoading(true);
    controllerRef.current = new AbortController();

    let content = '';
    let reasoning = '';
    let toolCalls: string[] = [];
    try {
      let context = '';
      try {
        context = await fetchMarketContext(question);
      } catch {
        message.warning('行情上下文暂时不可用，已继续对话');
      }
      const prompt = `${systemPrompt.trim() || DEFAULT_SYSTEM_PROMPT}

${thinkingMode ? '请先充分分析，再给出结构化结论。' : '请直接给出简洁、结构化的结论。'}
当前时间：${nowText()}`;
      const response = await fetch('/api/ai/chat/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        signal: controllerRef.current.signal,
        body: JSON.stringify({
          provider: selectedCredential.provider,
          model: selectedCredential.model,
          credential_id: selectedCredential.id,
          messages: [
            { role: 'system', content: prompt },
            ...history,
            { role: 'user', content: `${question}${context}` }
          ],
          options: {
            temperature: thinkingMode ? 0.2 : 0.4,
            max_tokens: 2400
          }
        })
      });
      if (!response.ok) {
        let errorText = await response.text();
        try {
          const payload = JSON.parse(errorText) as { message?: string };
          errorText = payload.message || errorText;
        } catch {
          // Keep the upstream text when it is not JSON.
        }
        throw new Error(errorText || 'AI 请求失败');
      }
      await readSSE(response, (event) => {
        const next = appendStreamEvent(event, content, reasoning, toolCalls);
        content = next.content;
        reasoning = next.reasoning;
        toolCalls = next.toolCalls;
        updateAssistant(assistantIndex, {
          content,
          reasoning,
          toolCalls
        });
      });
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        const text = error instanceof Error ? error.message : 'AI 请求失败';
        message.error(text);
        updateAssistant(assistantIndex, {
          content: content || `请求失败：${text}`,
          reasoning,
          toolCalls
        });
      }
    } finally {
      setLoading(false);
      controllerRef.current = null;
    }
  }

  function stop() {
    controllerRef.current?.abort();
    controllerRef.current = null;
    setLoading(false);
    message.info('已中断本次回答');
  }

  function startNewChat() {
    if (loading) {
      message.warning('当前有回答正在生成，请先中断或等待完成');
      return;
    }
    const next = initialMessages();
    setMessages(next);
    setVisibleCount(DEFAULT_VISIBLE_MESSAGES);
    setExpanded({ [next[0].id]: true });
  }

  async function copyText(text: string) {
    if (!text.trim()) return;
    try {
      await navigator.clipboard.writeText(text);
      message.success('已复制');
    } catch {
      message.warning('复制失败，请手动选择文本');
    }
  }

  return (
    <div className="ai-assistant-workspace">
      <Card
        className="work-card ai-assistant-card"
        title={
          <div className="ai-assistant-title">
            <RobotOutlined />
            <div>
              <strong>AI 助手</strong>
              <Text type="secondary">实时研究与复盘</Text>
            </div>
          </div>
        }
        extra={
          <Space>
            <Tag color={selectedCredential ? 'green' : 'orange'}>
              {selectedCredential ? `${selectedCredential.provider} · ${selectedCredential.model || '默认模型'}` : '未配置模型'}
            </Tag>
            <Button icon={<ReloadOutlined />} onClick={() => void loadCredentials()} loading={loadingCredentials}>
              刷新
            </Button>
          </Space>
        }
      >
        <div className="ai-assistant-toolbar">
          <Select
            showSearch
            value={selectedCredentialId || undefined}
            placeholder="选择模型"
            options={credentials.map((item) => ({
              value: item.id,
              label: `${item.name || item.provider}${item.model ? ` · ${item.model}` : ''}`,
              disabled: item.enabled === false
            }))}
            onChange={setSelectedCredentialId}
            className="ai-assistant-model-select"
            notFoundContent="暂无模型配置"
          />
          <label className="ai-assistant-toggle">
            <span>记忆</span>
            <Switch size="small" checked={memoryMode} onChange={setMemoryMode} />
            {memoryMode ? (
              <Select
                size="small"
                value={memoryCount}
                options={[5, 10, 20, 30, 50].map((value) => ({ value, label: `${value} 条` }))}
                onChange={setMemoryCount}
                className="ai-assistant-memory-select"
              />
            ) : null}
          </label>
          <label className="ai-assistant-toggle">
            <span>思考</span>
            <Switch size="small" checked={thinkingMode} onChange={setThinkingMode} />
          </label>
          <label className="ai-assistant-toggle">
            <span>行情上下文</span>
            <Switch size="small" checked={marketContextMode} onChange={setMarketContextMode} />
          </label>
          <Button icon={<ClearOutlined />} onClick={startNewChat}>新会话</Button>
        </div>

        <div className="ai-assistant-layout">
          <section className="ai-assistant-chat-panel">
            <div className="ai-assistant-message-list">
              {hiddenCount > 0 ? (
                <Button type="link" size="small" onClick={() => setVisibleCount(messages.length)}>
                  展开更多历史（{hiddenCount} 条）
                </Button>
              ) : null}
              {displayedMessages.map((item, index) => {
                const absoluteIndex = messages.length - displayedMessages.length + index;
                const isExpanded = expanded[item.id] !== false;
                const isLong = item.content.length + (item.reasoning || '').length > 500;
                return (
                  <article className={`ai-assistant-message ai-assistant-message--${item.role}`} key={item.id}>
                    <div className="ai-assistant-avatar">
                      {item.role === 'assistant' ? <RobotOutlined /> : <UserOutlined />}
                    </div>
                    <div className="ai-assistant-bubble">
                      <div className="ai-assistant-message-meta">
                        <strong>{item.role === 'assistant' ? 'AI 助手' : '我'}</strong>
                        <Text type="secondary">{item.time}</Text>
                      </div>
                      {item.role === 'assistant' && item.reasoning ? (
                        <details className="ai-assistant-reasoning" open={loading && absoluteIndex === messages.length - 1}>
                          <summary>思考过程</summary>
                          <pre>{item.reasoning}</pre>
                        </details>
                      ) : null}
                      {isLong && !isExpanded ? (
                        <div className="ai-assistant-content ai-assistant-content--collapsed">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{`${item.content.slice(0, 500)}...`}</ReactMarkdown>
                        </div>
                      ) : (
                        <div className="ai-assistant-content">
                          {item.content ? (
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{item.content}</ReactMarkdown>
                          ) : loading && absoluteIndex === messages.length - 1 ? '正在思考...' : null}
                        </div>
                      )}
                      {item.toolCalls?.length ? (
                        <div className="ai-assistant-tool-log">
                          {item.toolCalls.map((tool, toolIndex) => <Tag key={`${item.id}-${toolIndex}`}>{tool}</Tag>)}
                        </div>
                      ) : null}
                      <div className="ai-assistant-message-actions">
                        {isLong ? (
                          <Button
                            type="link"
                            size="small"
                            onClick={() => setExpanded((current) => ({ ...current, [item.id]: !isExpanded }))}
                          >
                            {isExpanded ? '收起' : '展开'}
                          </Button>
                        ) : null}
                        {item.role === 'assistant' ? (
                          <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => void copyText(item.content)}>
                            复制
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  </article>
                );
              })}
              {!messages.length ? <Empty description="开始一段新的研究对话" /> : null}
            </div>
            <div className="ai-assistant-composer">
              <Input.TextArea
                value={inputValue}
                onChange={(event) => setInputValue(event.target.value)}
                onPressEnter={(event) => {
                  if (!event.shiftKey) {
                    event.preventDefault();
                    void send();
                  }
                }}
                autoSize={{ minRows: 3, maxRows: 7 }}
                disabled={loading}
                placeholder="输入问题，回车发送，Shift+Enter 换行"
              />
              <div className="ai-assistant-composer-footer">
                <Text type="secondary">
                  {selectedCredential ? '股票代码会自动补充实时分析上下文' : '请先配置 AI 模型'}
                </Text>
                <Space>
                  {loading ? (
                    <Button danger icon={<StopOutlined />} onClick={stop}>中断</Button>
                  ) : null}
                  <Button
                    type="primary"
                    icon={<SendOutlined />}
                    disabled={!canSend}
                    loading={loading}
                    onClick={() => void send()}
                  >
                    发送
                  </Button>
                </Space>
              </div>
            </div>
          </section>

          <aside className="ai-assistant-settings">
            <div className="ai-assistant-settings-head">
              <div>
                <strong>研究指令</strong>
                <Text type="secondary">本地工作区提示词</Text>
              </div>
              <Tag color="blue">PROMPT</Tag>
            </div>
            <Input.TextArea
              value={systemPrompt}
              onChange={(event) => setSystemPrompt(event.target.value)}
              autoSize={{ minRows: 12, maxRows: 18 }}
              placeholder="输入本轮对话的系统指令"
            />
            <div className="ai-assistant-settings-note">
              <Text type="secondary">
                当前服务支持 OpenAI-compatible 模型。行情上下文使用本项目的行情分析接口，避免重复配置数据源。
              </Text>
            </div>
          </aside>
        </div>
      </Card>
    </div>
  );
}
