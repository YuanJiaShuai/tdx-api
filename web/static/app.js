// 全局变量
let currentStock = '';
let formulas = [];
let pools = [];
let strategies = [];
let factors = [];
let automations = [];
let webhooks = [];
let selectionResults = [];
let decisionResults = [];
let dailyReview = null;
let reviewItems = [];
let currentHQOverlay = null;
let selectedDecisionResult = null;
let selectedReviewItem = null;
let decisionShowingToday = false;
let selectedStrategyID = '';
let strategyRunState = {
    running: false,
    strategyID: '',
    stepIndex: 0,
    timer: null,
    message: '',
    progress: 0
};

// 工具函数 - 显示加载
function showLoading() {
    document.getElementById('loading').style.display = 'flex';
}

// 工具函数 - 隐藏加载
function hideLoading() {
    document.getElementById('loading').style.display = 'none';
}

async function apiFetch(url, options = {}) {
    const response = await fetch(url, {
        headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
        ...options
    });
    const result = await response.json();
    if (result.code !== 0) {
        throw new Error(result.message || '请求失败');
    }
    return result.data;
}

async function refreshSystemStatus() {
    const serviceNode = document.getElementById('serviceStatusText');
    const engineNode = document.getElementById('formulaEngineText');
    const timeNode = document.getElementById('systemTimeText');
    if (!serviceNode || !engineNode || !timeNode) return;
    try {
        const [status, formula] = await Promise.all([
            apiFetch('/api/server-status'),
            apiFetch('/api/formula/health')
        ]);
        serviceNode.textContent = status.ready ? '运行正常' : (status.status || '异常');
        engineNode.textContent = formula.engine || 'fallback';
        timeNode.textContent = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    } catch (error) {
        serviceNode.textContent = '连接异常';
        engineNode.textContent = '--';
        timeNode.textContent = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    }
}

function loadQuickStock(code) {
    const input = document.getElementById('stockCode');
    if (input) input.value = code;
    currentStock = code;
    loadStockData(code);
}

function switchWorkspace(name, button) {
    document.querySelectorAll('.workspace-tab').forEach(btn => btn.classList.remove('active'));
    if (button) button.classList.add('active');
    document.querySelectorAll('.workspace').forEach(item => item.classList.remove('active'));
    document.getElementById(name + 'Workspace').classList.add('active');
    if (name === 'proChart') {
        setTimeout(() => {
            loadFormulaList();
            loadHQChart();
            if (window.TDXHQChart) window.TDXHQChart.resize();
        }, 50);
    }
    if (name === 'market') refreshDecisionDesk();
    if (name === 'dataCenter') loadDataCenter();
    if (name === 'selectionResults') loadSelectionResults();
    if (name === 'dailyReview') loadDailyReview();
    if (name === 'strategies') loadStrategyCenter();
    if (name === 'automations') loadAutomationData();
    if (name === 'webhooks') loadWebhooks();
}

// 工具函数 - 格式化数字
function formatNumber(num, decimals = 2) {
    if (!num || isNaN(num)) return '--';
    return parseFloat(num).toFixed(decimals);
}

// 工具函数 - 格式化金额（转换为万、亿）
function formatAmount(num) {
    if (!num || isNaN(num)) return '--';
    num = parseFloat(num);
    if (num >= 100000000) {
        return (num / 100000000).toFixed(2) + '亿';
    } else if (num >= 10000) {
        return (num / 10000).toFixed(2) + '万';
    }
    return num.toFixed(2);
}

// 工具函数 - 格式化价格（将厘转为元）
function formatPrice(price) {
    if (!price || isNaN(price)) return '--';
    return (parseFloat(price) / 1000).toFixed(2);
}

function prettyJSON(value) {
    return JSON.stringify(value, null, 2);
}

function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, ch => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
    }[ch]));
}

function escapeJSString(value) {
    return String(value ?? '').replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n').replace(/\r/g, '');
}

function compactValue(value) {
    if (value === null || value === undefined || value === '') return '--';
    if (typeof value === 'number') return Number.isInteger(value) ? String(value) : value.toFixed(4).replace(/\.?0+$/, '');
    if (typeof value === 'boolean') return value ? '是' : '否';
    if (Array.isArray(value)) return value.length > 6 ? `${value.slice(0, 6).join(', ')} ...` : value.join(', ');
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
}

function normalizeSymbol(symbol) {
    return String(symbol || '').trim().toUpperCase().replace(/\.(SH|SZ|BJ)$/i, '');
}

function renderHQKLine(containerId, symbol, period = 'day', options = {}) {
    const container = document.getElementById(containerId);
    if (!container) return false;
    if (!window.TDXHQChart || !window.TDXHQChart.isAvailable()) {
        container.innerHTML = '<div class="data-item">HQChart 未加载，无法显示图表</div>';
        return false;
    }
    const ok = window.TDXHQChart.renderKLine(container, {
        symbol: normalizeSymbol(symbol),
        period,
        ...options
    });
    if (!ok) {
        container.innerHTML = '<div class="data-item">HQChart 初始化失败</div>';
    }
    return ok;
}

function poolByID(id) {
    return pools.find(pool => pool.id === id);
}

function symbolInPool(symbol, poolID) {
    const pool = poolByID(poolID);
    const normalized = normalizeSymbol(symbol);
    return !!pool && (pool.symbols || []).map(normalizeSymbol).includes(normalized);
}

function localDateString(date = new Date()) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

function setLoadingText(containerId, text = '加载中...') {
    const container = document.getElementById(containerId);
    if (container) container.innerHTML = `<div class="data-item">${escapeHTML(text)}</div>`;
}

function setErrorText(containerId, error) {
    const container = document.getElementById(containerId);
    if (container) container.innerHTML = `<div class="data-item">${escapeHTML(error.message || error)}</div>`;
}

function renderMetricCards(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = items.map(item => `
        <div class="metric-card">
            <span class="metric-label">${escapeHTML(item.label)}</span>
            <span class="metric-value">${escapeHTML(item.value)}</span>
            ${item.note ? `<span class="metric-note">${escapeHTML(item.note)}</span>` : ''}
        </div>
    `).join('');
}

function renderTable(containerId, rows, columns) {
    const container = document.getElementById(containerId);
    if (!container) return;
    if (!rows || rows.length === 0) {
        container.innerHTML = '<div class="data-item">暂无数据</div>';
        return;
    }
    const visibleRows = rows.slice(0, 300);
    container.innerHTML = `
        <table class="data-table">
            <thead>
                <tr>${columns.map(col => `<th>${escapeHTML(col.label)}</th>`).join('')}</tr>
            </thead>
            <tbody>
                ${visibleRows.map(row => `
                    <tr>
                        ${columns.map(col => `<td>${escapeHTML(compactValue(typeof col.value === 'function' ? col.value(row) : row[col.key]))}</td>`).join('')}
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
}

function renderKeyValuePanel(title, data) {
    const entries = Object.entries(data || {}).filter(([, value]) => value !== undefined && value !== null && typeof value !== 'object');
    if (entries.length === 0) {
        return `
            <div class="data-panel">
                <div class="data-panel-title">${escapeHTML(title)}</div>
                <div class="data-panel-body"><pre class="json-output">${escapeHTML(prettyJSON(data || {}))}</pre></div>
            </div>
        `;
    }
    return `
        <div class="data-panel">
            <div class="data-panel-title">${escapeHTML(title)}</div>
            <div class="data-panel-body">
                <div class="kv-grid">
                    ${entries.slice(0, 80).map(([key, value]) => `
                        <div class="kv-item">
                            <span class="kv-key">${escapeHTML(key)}</span>
                            <span class="kv-value">${escapeHTML(compactValue(value))}</span>
                        </div>
                    `).join('')}
                </div>
            </div>
        </div>
    `;
}

function renderJsonPanel(title, data) {
    return `
        <div class="data-panel">
            <div class="data-panel-title">${escapeHTML(title)}</div>
            <div class="data-panel-body"><pre class="json-output">${escapeHTML(prettyJSON(data))}</pre></div>
        </div>
    `;
}

// 搜索股票
async function searchStock() {
    const keyword = document.getElementById('stockCode').value.trim();
    if (!keyword) {
        alert('请输入股票代码或名称');
        return;
    }

    // 如果直接输入的是6位股票代码，直接加载
    if (/^\d{6}$/.test(keyword)) {
        currentStock = keyword;
        loadStockData(keyword);
        return;
    }

    // 否则搜索
    showLoading();
    try {
        const response = await fetch(`/api/search?keyword=${encodeURIComponent(keyword)}`);
        const result = await response.json();
        
        if (result.code === 0 && result.data && result.data.length > 0) {
            displaySearchResults(result.data);
        } else {
            alert('未找到相关股票');
        }
    } catch (error) {
        console.error('搜索失败:', error);
        alert('搜索失败，请重试');
    } finally {
        hideLoading();
    }
}

// 显示搜索结果
function displaySearchResults(results) {
    const container = document.getElementById('searchResults');
    container.innerHTML = '';
    
    results.forEach(item => {
        const div = document.createElement('div');
        div.className = 'search-item';
        div.innerHTML = `
            <span class="search-item-code">${item.code}</span>
            <span class="search-item-name">${item.name}</span>
        `;
        div.onclick = () => {
            currentStock = item.code;
            loadStockData(item.code);
            container.innerHTML = '';
        };
        container.appendChild(div);
    });
}

// 加载股票数据
async function loadStockData(code) {
    showLoading();
    document.getElementById('mainContent').style.display = 'block';
    
    try {
        // 加载五档行情
        await loadQuote(code);
        
        // 默认加载日K线
        await loadKline('day');
        
        // 加载分时数据
        await loadMinute(code);
        
        // 加载分时成交
        await loadTrade(code);
        
    } catch (error) {
        console.error('加载数据失败:', error);
        alert('加载数据失败，请重试');
    } finally {
        hideLoading();
    }
}

// 加载五档行情
async function loadQuote(code) {
    try {
        const response = await fetch(`/api/quote?code=${code}`);
        const result = await response.json();
        
        if (result.code === 0 && result.data && result.data.length > 0) {
            const quote = result.data[0];
            displayQuote(quote);
        }
    } catch (error) {
        console.error('加载五档行情失败:', error);
    }
}

// 显示五档行情
function displayQuote(quote) {
    // 更新股票名称和代码
    document.getElementById('stockName').textContent = quote.Code || '--';
    document.getElementById('stockCode2').textContent = quote.Code || '--';
    
    // 计算价格（从厘转为元）
    const lastPrice = parseFloat(quote.K.Last) / 1000;
    const currentPrice = parseFloat(quote.K.Close) / 1000;
    const openPrice = parseFloat(quote.K.Open) / 1000;
    const highPrice = parseFloat(quote.K.High) / 1000;
    const lowPrice = parseFloat(quote.K.Low) / 1000;
    
    const priceChange = currentPrice - lastPrice;
    const priceChangePercent = lastPrice > 0 ? (priceChange / lastPrice * 100) : 0;
    
    // 更新基本信息
    document.getElementById('lastPrice').textContent = currentPrice.toFixed(2);
    document.getElementById('priceChange').textContent = priceChange > 0 ? '+' + priceChange.toFixed(2) : priceChange.toFixed(2);
    document.getElementById('priceChangePercent').textContent = priceChangePercent > 0 ? '+' + priceChangePercent.toFixed(2) + '%' : priceChangePercent.toFixed(2) + '%';
    document.getElementById('volume').textContent = formatAmount(quote.TotalHand * 100);
    document.getElementById('amount').textContent = formatAmount(quote.Amount);
    document.getElementById('openPrice').textContent = openPrice.toFixed(2);
    document.getElementById('highPrice').textContent = highPrice.toFixed(2);
    document.getElementById('lowPrice').textContent = lowPrice.toFixed(2);
    
    // 设置涨跌颜色
    const priceElements = [document.getElementById('lastPrice'), 
                          document.getElementById('priceChange'), 
                          document.getElementById('priceChangePercent')];
    priceElements.forEach(el => {
        el.className = 'value price ' + (priceChange >= 0 ? 'up' : 'down');
    });
    
    // 更新买卖五档
    const sellLevels = document.getElementById('sellLevels');
    sellLevels.innerHTML = '';
    for (let i = 4; i >= 0; i--) {
        const level = quote.SellLevel[i];
        const price = parseFloat(level.Price) / 1000;
        const volume = Math.round(level.Number / 100);
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>卖${i + 1}</td>
            <td>${price.toFixed(2)}</td>
            <td>${volume}</td>
        `;
        sellLevels.appendChild(tr);
    }
    
    const buyLevels = document.getElementById('buyLevels');
    buyLevels.innerHTML = '';
    for (let i = 0; i < 5; i++) {
        const level = quote.BuyLevel[i];
        const price = parseFloat(level.Price) / 1000;
        const volume = Math.round(level.Number / 100);
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>买${i + 1}</td>
            <td>${price.toFixed(2)}</td>
            <td>${volume}</td>
        `;
        buyLevels.appendChild(tr);
    }
}

// 加载K线数据
async function loadKline(type, buttonElement) {
    if (!currentStock) return;
    
    showLoading();
    try {
        renderHQKLine('klineChart', currentStock, type, { count: 800, pageSize: 80 });
        
        // 更新按钮状态
        document.querySelectorAll('.btn-control').forEach(btn => {
            btn.classList.remove('active');
        });
        // 如果提供了按钮元素，添加active类
        if (buttonElement) {
            buttonElement.classList.add('active');
        } else {
            // 如果没有提供，根据type查找对应按钮
            const buttons = document.querySelectorAll('.btn-control');
            buttons.forEach(btn => {
                if (btn.textContent.includes(getKlineTypeName(type))) {
                    btn.classList.add('active');
                }
            });
        }
    } catch (error) {
        console.error('加载K线失败:', error);
    } finally {
        hideLoading();
    }
}

// 获取K线类型中文名称
function getKlineTypeName(type) {
    const typeMap = {
        'day': '日K',
        'week': '周K',
        'month': '月K',
        'minute30': '30分',
        'minute15': '15分',
        'minute5': '5分'
    };
    return typeMap[type] || '日K';
}

function getActiveKlineType() {
    const active = document.querySelector('.btn-control.active');
    const label = active?.textContent || '日K';
    if (label.includes('周')) return 'week';
    if (label.includes('月')) return 'month';
    if (label.includes('30')) return 'minute30';
    if (label.includes('15')) return 'minute15';
    if (label.includes('5')) return 'minute5';
    return 'day';
}

// 加载分时数据
async function loadMinute(code) {
    try {
        renderHQKLine('minuteChart', code, 'minute1', { count: 240, pageSize: 80, dataWidth: 8 });
    } catch (error) {
        console.error('加载分时数据失败:', error);
    }
}

// 加载分时成交
async function loadTrade(code) {
    try {
        const response = await fetch(`/api/trade?code=${code}`);
        const result = await response.json();
        
        if (result.code === 0 && result.data) {
            displayTrade(result.data);
        }
    } catch (error) {
        console.error('加载分时成交失败:', error);
    }
}

// 显示分时成交
function displayTrade(data) {
    if (!data.List || data.List.length === 0) {
        document.getElementById('tradeTableBody').innerHTML = '<tr><td colspan="5">暂无数据</td></tr>';
        return;
    }
    
    const tbody = document.getElementById('tradeTableBody');
    tbody.innerHTML = '';
    
    // 只显示最近200条
    const trades = data.List.slice(0, 200);
    
    trades.forEach(item => {
        const time = new Date(item.Time);
        const timeStr = String(time.getHours()).padStart(2, '0') + ':' + 
                       String(time.getMinutes()).padStart(2, '0') + ':' +
                       String(time.getSeconds()).padStart(2, '0');
        const price = (parseFloat(item.Price) / 1000).toFixed(2);
        const volume = item.Volume;
        const amount = formatAmount(parseFloat(item.Price) / 1000 * volume * 100);
        const status = item.Status === 0 ? '买入' : (item.Status === 1 ? '卖出' : '--');
        const statusClass = item.Status === 0 ? 'trade-buy' : (item.Status === 1 ? 'trade-sell' : '');
        
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${timeStr}</td>
            <td class="${statusClass}">${price}</td>
            <td>${volume}</td>
            <td>${amount}</td>
            <td class="${statusClass}">${status}</td>
        `;
        tbody.appendChild(tr);
    });
}

// 切换标签页
function switchTab(evt, tabName) {
    // 更新标签按钮状态
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    if (evt && evt.target) {
        evt.target.classList.add('active');
    }
    
    // 更新内容显示
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    const activeTab = document.getElementById(tabName + 'Tab');
    activeTab.classList.add('active');

    // 切换到图表时触发自适应，解决在隐藏容器中初始化导致的宽度问题
    requestAnimationFrame(() => {
        if (tabName === 'kline' && window.TDXHQChart) {
            setTimeout(() => {
                if (currentStock) renderHQKLine('klineChart', currentStock, getActiveKlineType(), { count: 800, pageSize: 80 });
                window.TDXHQChart.resize(document.getElementById('klineChart'));
            }, 50);
        }
        if (tabName === 'minute' && window.TDXHQChart) {
            setTimeout(() => {
                if (currentStock) renderHQKLine('minuteChart', currentStock, 'minute1', { count: 240, pageSize: 80, dataWidth: 8 });
                window.TDXHQChart.resize(document.getElementById('minuteChart'));
            }, 50);
        }
    });
}

// 监听回车键搜索
document.getElementById('stockCode').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        searchStock();
    }
});

// 窗口大小改变时重新渲染图表
let resizeTimer;
window.addEventListener('resize', function() {
    // 防抖优化，避免频繁调用
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function() {
        if (window.TDXHQChart) window.TDXHQChart.resize();
    }, 100);
});

async function loadFormulaList() {
    formulas = await apiFetch('/api/formulas');
    const formulaOptions = formulas.map(f => `<option value="${f.id}">${escapeHTML(f.name)}</option>`).join('');
    const hqFormulaSelect = document.getElementById('hqFormulaSelect');
    if (hqFormulaSelect) {
        hqFormulaSelect.innerHTML = formulaOptions;
        hqFormulaSelect.onchange = renderHQFormulaArgsSummary;
        renderHQFormulaArgsSummary();
    }
    renderFormulaArgsEditor(parseHQFormulaArgs({ args_json: document.getElementById('formulaArgs')?.value || '[]' }));
    const automationFormula = document.getElementById('automationFormula');
    if (automationFormula) automationFormula.innerHTML = formulaOptions;
    const resultFilter = document.getElementById('resultFormulaFilter');
    if (resultFilter) {
        resultFilter.innerHTML = '<option value="">全部公式</option>' + formulaOptions;
    }
    document.getElementById('formulaList').innerHTML = formulas.map(f => `
        <div class="data-item formula-list-item">
            <div class="formula-list-head">
                <strong title="${escapeHTML(f.name)}">${escapeHTML(f.name)}</strong>
                <span>${f.enabled ? '启用' : '停用'}</span>
            </div>
            <div class="formula-list-meta">${escapeHTML(f.type)} · ${escapeHTML(f.period)} · 参数：${escapeHTML(formatFormulaArgs(f))}</div>
            <div class="formula-script-preview" title="${escapeHTML(f.script)}">${escapeHTML(compactFormulaScript(f.script))}</div>
            <div class="item-actions">
                <button onclick="fillFormula('${f.id}')">编辑</button>
                <button class="primary" onclick="quickTestFormula('${f.id}')">测试</button>
                <button onclick="deleteFormula('${f.id}')">删除</button>
            </div>
        </div>
    `).join('') || '<div class="data-item">暂无公式</div>';
}

function fillFormula(id) {
    const f = formulas.find(item => item.id === id);
    if (!f) return;
    document.getElementById('formulaName').dataset.id = f.id;
    document.getElementById('formulaName').value = f.name;
    document.getElementById('formulaType').value = f.type;
    document.getElementById('formulaPeriod').value = f.period;
    document.getElementById('formulaRight').value = String(f.right);
    document.getElementById('formulaScript').value = f.script;
    document.getElementById('formulaArgs').value = f.args_json || '[]';
    renderFormulaArgsEditor(parseHQFormulaArgs(f));
}

function formatFormulaArgs(formula) {
    const args = parseHQFormulaArgs(formula);
    return args.length ? args.map(item => `${item.Name}=${item.Value}`).join('，') : '无';
}

function compactFormulaScript(script) {
    return String(script || '').replace(/\s+/g, ' ').trim() || '暂无脚本';
}

function appendFormulaArgRow(arg = {}) {
    const editor = document.getElementById('formulaArgsEditor');
    if (!editor) return;
    const emptyNode = editor.querySelector('.formula-empty-note');
    if (emptyNode) emptyNode.remove();
    const row = document.createElement('div');
    row.className = 'formula-arg-row';
    row.innerHTML = `
        <input class="formula-arg-name" value="${escapeHTML(arg.Name || '')}" placeholder="参数名">
        <input class="formula-arg-value" value="${escapeHTML(arg.Value ?? '')}" placeholder="参数值">
        <button type="button" onclick="removeFormulaArgRow(this)">删除</button>
    `;
    editor.appendChild(row);
}

function renderFormulaArgsEditor(args = []) {
    const editor = document.getElementById('formulaArgsEditor');
    if (!editor) return;
    editor.innerHTML = '';
    if (!args.length) {
        editor.innerHTML = '<div class="formula-empty-note">暂无参数，可新增</div>';
        return;
    }
    args.forEach(arg => appendFormulaArgRow(arg));
}

function addFormulaArgRow() {
    appendFormulaArgRow();
}

function removeFormulaArgRow(target) {
    target?.closest('.formula-arg-row')?.remove();
    const editor = document.getElementById('formulaArgsEditor');
    if (editor && !editor.querySelector('.formula-arg-row')) {
        editor.innerHTML = '<div class="formula-empty-note">暂无参数，可新增</div>';
    }
    syncFormulaArgsJSON();
}

function getFormulaArgsFromEditor() {
    const editor = document.getElementById('formulaArgsEditor');
    if (!editor) return [];
    return Array.from(editor.querySelectorAll('.formula-arg-row')).map(row => ({
        Name: row.querySelector('.formula-arg-name')?.value.trim() || '',
        Value: normalizeHQArgValue(row.querySelector('.formula-arg-value')?.value || '')
    })).filter(item => item.Name && item.Value !== '');
}

function syncFormulaArgsJSON() {
    const value = JSON.stringify(getFormulaArgsFromEditor());
    const node = document.getElementById('formulaArgs');
    if (node) node.value = value;
    return value;
}

async function saveFormula() {
    try {
        const id = document.getElementById('formulaName').dataset.id || '';
        const argsJSON = syncFormulaArgsJSON();
        JSON.parse(argsJSON || '[]');
        await apiFetch(id ? `/api/formulas/${id}` : '/api/formulas', {
            method: id ? 'PUT' : 'POST',
            body: JSON.stringify({
                id,
                name: document.getElementById('formulaName').value,
                type: document.getElementById('formulaType').value,
                period: document.getElementById('formulaPeriod').value,
                right: Number(document.getElementById('formulaRight').value),
                script: document.getElementById('formulaScript').value,
                args_json: argsJSON || '[]',
                enabled: true
            })
        });
        document.getElementById('formulaName').dataset.id = '';
        await loadFormulaList();
        renderHQFormulaArgsSummary();
        alert('公式已保存');
    } catch (error) {
        alert(error.message);
    }
}

async function deleteFormula(id) {
    if (!confirm('确认删除这个公式？')) return;
    await apiFetch(`/api/formulas/${id}`, { method: 'DELETE' });
    await loadFormulaList();
}

async function quickTestFormula(id) {
    const symbol = currentStock || document.getElementById('hqSymbol').value || '000001';
    await runFormulaTest(id, symbol);
    switchWorkspace('proChart', document.querySelectorAll('.workspace-tab')[1]);
}

function editSelectedHQFormula() {
    const id = document.getElementById('hqFormulaSelect')?.value;
    if (!id) {
        alert('请先选择一个公式');
        return;
    }
    openFormulaDialog();
    fillFormula(id);
}

function openFormulaDialog() {
    const dialog = document.getElementById('formulaDialog');
    if (!dialog) return;
    dialog.classList.add('open');
    dialog.setAttribute('aria-hidden', 'false');
    loadFormulaList();
}

function closeFormulaDialog() {
    const dialog = document.getElementById('formulaDialog');
    if (!dialog) return;
    dialog.classList.remove('open');
    dialog.setAttribute('aria-hidden', 'true');
}

async function testSelectedFormula() {
    const id = document.getElementById('hqFormulaSelect').value;
    const symbol = document.getElementById('hqSymbol').value || currentStock || '000001';
    await runFormulaTest(id, symbol);
}

async function runFormulaTest(id, symbol, options = {}) {
    try {
        const data = await apiFetch(`/api/formulas/${id}/test`, {
            method: 'POST',
            body: JSON.stringify({
                symbol,
                calc_count: 240,
                out_count: 5,
                ...(options.args ? { args: options.args } : {})
            })
        });
        document.getElementById('formulaTestOutput').textContent = prettyJSON(data);
    } catch (error) {
        document.getElementById('formulaTestOutput').textContent = error.message;
    }
}

function selectedHQFormula() {
    const id = document.getElementById('hqFormulaSelect')?.value;
    return formulas.find(item => item.id === id) || null;
}

function normalizeHQArgValue(value) {
    const text = String(value ?? '').trim();
    if (text === '') return '';
    const numberValue = Number(text);
    return Number.isFinite(numberValue) ? numberValue : text;
}

function normalizeHQFormulaArgs(args) {
    if (!Array.isArray(args)) return [];
    return args.map(item => {
        const name = item?.Name ?? item?.name ?? '';
        const value = item?.Value ?? item?.value ?? '';
        return { Name: String(name).trim(), Value: normalizeHQArgValue(value) };
    }).filter(item => item.Name && item.Value !== '');
}

function parseHQFormulaArgs(formula) {
    const raw = formula?.args_json ?? formula?.args ?? [];
    if (Array.isArray(raw)) return normalizeHQFormulaArgs(raw);
    if (!String(raw || '').trim()) return [];
    try {
        return normalizeHQFormulaArgs(JSON.parse(raw));
    } catch (error) {
        updateHQOverlayStatus(`参数JSON解析失败：${error.message}`);
        return [];
    }
}

function renderHQFormulaArgsSummary() {
    const node = document.getElementById('hqFormulaArgsSummary');
    if (!node) return;
    const args = parseHQFormulaArgs(selectedHQFormula());
    if (!args.length) {
        node.textContent = '公式参数：无';
        return;
    }
    node.textContent = `公式参数：${args.map(item => `${item.Name}=${item.Value}`).join('，')}`;
}

async function loadPools() {
    pools = await apiFetch('/api/stock-pools');
    const automationPool = document.getElementById('automationPool');
    if (automationPool) automationPool.innerHTML = customPools().map(p => `<option value="${p.id}">${escapeHTML(p.name)}</option>`).join('');
    const poolList = document.getElementById('poolList');
    if (!poolList) return;
    const market = marketPools();
    const custom = customPools();
    poolList.innerHTML = `
        ${market.length ? `<div class="data-list-title">系统市场分组</div>${market.map(renderPoolItem).join('')}` : ''}
        ${custom.length ? `<div class="data-list-title">自定义股票池</div>${custom.map(renderPoolItem).join('')}` : ''}
    ` || '<div class="data-item">暂无股票池</div>';
}

function marketPools() {
    return pools.filter(pool => pool.category === 'market' || String(pool.id || '').startsWith('market-'));
}

function customPools() {
    return pools.filter(pool => !(pool.category === 'market' || String(pool.id || '').startsWith('market-')));
}

function renderPoolItem(p) {
    const readonly = p.readonly || p.category === 'market';
    return `
        <div class="data-item">
            <div class="data-item-title">${escapeHTML(p.name)}${readonly ? ' <span class="tag">系统分组</span>' : ''}</div>
            <div class="data-item-meta">${(p.symbols || []).length} 只股票 · ${escapeHTML((p.symbols || []).slice(0, 24).join(', '))}${(p.symbols || []).length > 24 ? ' ...' : ''}</div>
            ${p.description ? `<div class="data-item-meta">${escapeHTML(p.description)}</div>` : ''}
            ${readonly ? '' : `<div class="item-actions">
                <button onclick="fillPool('${p.id}')">编辑</button>
                <button onclick="deletePool('${p.id}')">删除</button>
            </div>`}
        </div>
    `;
}

function fillPool(id) {
    const p = pools.find(item => item.id === id);
    if (!p) return;
    document.getElementById('poolName').dataset.id = p.id;
    document.getElementById('poolName').value = p.name;
    document.getElementById('poolSymbols').value = p.symbols.join('\n');
}

async function savePool() {
    try {
        const id = document.getElementById('poolName').dataset.id || '';
        const symbols = document.getElementById('poolSymbols').value.split(/[\s,，]+/).filter(Boolean);
        await apiFetch(id ? `/api/stock-pools/${id}` : '/api/stock-pools', {
            method: id ? 'PUT' : 'POST',
            body: JSON.stringify({ id, name: document.getElementById('poolName').value, symbols })
        });
        document.getElementById('poolName').dataset.id = '';
        await loadPools();
        alert('股票池已保存');
    } catch (error) {
        alert(error.message);
    }
}

async function deletePool(id) {
    if (!confirm('确认删除这个股票池？')) return;
    await apiFetch(`/api/stock-pools/${id}`, { method: 'DELETE' });
    await loadPools();
}

async function loadStrategyCenter() {
    await Promise.all([loadStrategies(), loadFactors(), loadFormulaList(), loadPools()]);
    if (!selectedStrategyID && strategies.length) {
        fillStrategy(strategies[0].id);
    }
}

async function loadStrategies() {
    strategies = await apiFetch('/api/strategies') || [];
    renderStrategyList();
    const select = document.getElementById('automationStrategy');
    if (select) {
        select.innerHTML = strategies.map(item => `<option value="${item.id}">${escapeHTML(item.name)}${item.readonly ? '（模板）' : ''}</option>`).join('');
    }
}

async function loadFactors() {
    factors = await apiFetch('/api/factors') || [];
    const list = document.getElementById('factorList');
    if (!list) return;
    list.innerHTML = factors.map(factor => `
        <div class="data-item factor-item">
            <div class="data-item-title">${escapeHTML(factor.name)} <span class="tag">${escapeHTML(factor.kind)}</span></div>
            <div class="data-item-meta">${escapeHTML(factor.id)} · ${escapeHTML(factor.description)}</div>
            <div class="factor-param-list">${(factor.params || []).map(p => `<span>${escapeHTML(p.label)}=${escapeHTML(compactValue(p.default))}</span>`).join('')}</div>
            <div class="data-item-meta">${escapeHTML(strategyFactorEditHint(factor.id))}</div>
        </div>
    `).join('') || '<div class="data-item">暂无因子</div>';
}

function strategyFactorEditHint(factorID) {
    const map = {
        pool_exclude: '在“硬过滤条件 / 排除股票池”里勾选并选择股票池。',
        min_amount: '在“硬过滤条件 / 最低成交额”里勾选并输入金额。',
        price_range: '在“硬过滤条件 / 收盘价区间”里勾选并输入最低价、最高价。',
        change_range: '在“硬过滤条件 / 涨跌幅区间”里勾选并输入百分比范围。',
        ma_trend: '在“评分条件 / 均线多头”里勾选并设置均线和权重。',
        volume_up: '在“评分条件 / 阶段放量”里勾选并设置天数、倍数和权重。',
        break_high: '在“评分条件 / 突破新高”里勾选并设置回看天数和权重。',
        formula: '在“评分条件 / 公式因子”里勾选并选择公式。'
    };
    return map[factorID] || '在策略编辑区勾选并调整参数。';
}

function renderStrategyList() {
    const list = document.getElementById('strategyList');
    if (!list) return;
    list.innerHTML = strategies.map(item => {
        const cfg = parseStrategyConfig(item.config_json);
        const filterCount = Array.isArray(cfg.filters) ? cfg.filters.length : 0;
        const scoreCount = Array.isArray(cfg.scores) ? cfg.scores.length : 0;
        const selected = item.id === selectedStrategyID ? ' selected' : '';
        return `
            <div class="data-item strategy-item${selected}" onclick="fillStrategy('${escapeJSString(item.id)}')">
                <div class="data-item-title">${escapeHTML(item.name)}${item.readonly ? ' <span class="tag">内置模板</span>' : ''}</div>
                <div class="data-item-meta">${escapeHTML(item.description || '暂无说明')}</div>
                <div class="data-item-meta">${escapeHTML(strategyUniverseLabel(cfg))} · 过滤 ${filterCount} · 评分 ${scoreCount} · ${item.enabled ? '启用' : '停用'}</div>
                ${renderStrategyRunStatus(item.id)}
            </div>
        `;
    }).join('') || '<div class="data-item">暂无策略</div>';
}

function renderStrategyRunStatus(strategyID) {
    if (!strategyRunState.running || strategyRunState.strategyID !== strategyID) {
        return '<div class="strategy-run-mini idle">空闲</div>';
    }
    return `
        <div class="strategy-run-mini running">
            <span>${escapeHTML(strategyRunState.message || '运行中')}</span>
            <strong>${strategyRunState.progress}%</strong>
            <div class="mini-progress"><i style="width:${strategyRunState.progress}%"></i></div>
        </div>
    `;
}

function parseStrategyConfig(raw) {
    if (typeof raw === 'object' && raw) return raw;
    try {
        return JSON.parse(raw || '{}');
    } catch {
        return {};
    }
}

function strategyUniverseLabel(cfg = {}) {
    const universe = cfg.universe || 'pool';
    if (universe === 'symbols') return `自定义标的 ${(cfg.symbols || []).length} 只`;
    if (universe === 'all' || universe === 'all_a') return '全市场A股';
    if (universe === 'market') {
        const pool = pools.find(item => item.id === cfg.pool_id);
        return `市场分组 ${pool?.name || cfg.pool_id || '全部A股'}`;
    }
    return `股票池 ${cfg.pool_id || 'watchlist'}`;
}

function defaultStrategyConfig() {
    return {
        universe: 'pool',
        pool_id: 'watchlist',
        period: 'day',
        calc_count: 260,
        batch_size: 50,
        continue_on_error: true,
        filters: [
            { id: 'exclude_pool', factor: 'pool_exclude', params: { pool_id: 'exclude' } }
        ],
        scores: [
            { id: 'ma_trend', factor: 'ma_trend', weight: 20, params: { short: 5, mid: 10, long: 20 } }
        ],
        pass: { min_score: 20, top_n: 50 }
    };
}

function strategyRule(cfg, factor) {
    return [...(cfg.filters || []), ...(cfg.scores || [])].find(rule => rule.factor === factor) || null;
}

function poolOptionsHTML(selectedID) {
    return pools.map(pool => `<option value="${escapeHTML(pool.id)}" ${pool.id === selectedID ? 'selected' : ''}>${escapeHTML(pool.name)}（${(pool.symbols || []).length}只）</option>`).join('');
}

function marketPoolOptionsHTML(selectedID) {
    const items = marketPools();
    return items.map(pool => `<option value="${escapeHTML(pool.id)}" ${pool.id === selectedID ? 'selected' : ''}>${escapeHTML(pool.name)}（${(pool.symbols || []).length}只）</option>`).join('');
}

function customPoolOptionsHTML(selectedID) {
    const items = customPools();
    return items.map(pool => `<option value="${escapeHTML(pool.id)}" ${pool.id === selectedID ? 'selected' : ''}>${escapeHTML(pool.name)}（${(pool.symbols || []).length}只）</option>`).join('');
}

function poolPreviewHTML(poolID) {
    const pool = pools.find(item => item.id === poolID);
    if (!pool) return '<div class="strategy-pool-preview">未找到这个股票池</div>';
    const symbols = pool.symbols || [];
    return `<div class="strategy-pool-preview">${escapeHTML(pool.name)}：${symbols.length ? escapeHTML(symbols.slice(0, 18).join(', ')) : '暂无股票'}${symbols.length > 18 ? ' ...' : ''}</div>`;
}

function renderStrategyVisualEditor(cfg = {}) {
    const box = document.getElementById('strategyVisualEditor');
    if (!box) return;
    const exclude = strategyRule(cfg, 'pool_exclude');
    const minAmount = strategyRule(cfg, 'min_amount');
    const priceRange = strategyRule(cfg, 'price_range');
    const changeRange = strategyRule(cfg, 'change_range');
    const maTrend = strategyRule(cfg, 'ma_trend');
    const volumeUp = strategyRule(cfg, 'volume_up');
    const breakHigh = strategyRule(cfg, 'break_high');
    const formulaRule = strategyRule(cfg, 'formula');
    const universe = cfg.universe || 'pool';
    const poolID = cfg.pool_id || (universe === 'market' ? 'market-all-a' : 'watchlist');
    const excludePoolID = exclude?.params?.pool_id || 'exclude';
    const formulaID = formulaRule?.params?.formula_id || '';
    const formulaName = formulaRule?.params?.formula_name || '';
    const selectedFormula = formulas.find(item => item.id === formulaID || item.name === formulaName);

    box.innerHTML = `
        <div class="strategy-section">
            <div class="strategy-section-head">
                <strong>选股范围</strong>
                <span>先决定从哪些股票里选</span>
            </div>
            <div class="strategy-field-grid">
                <label>范围类型
                    <select id="strategyUniverse" onchange="handleStrategyUniverseChange()">
                        <option value="pool" ${universe === 'pool' || universe === '' ? 'selected' : ''}>股票池</option>
                        <option value="market" ${universe === 'market' ? 'selected' : ''}>市场分组</option>
                        <option value="symbols" ${universe === 'symbols' ? 'selected' : ''}>手动输入股票</option>
                        <option value="all_a" ${universe === 'all_a' || universe === 'all' ? 'selected' : ''}>全市场A股</option>
                    </select>
                </label>
                <label id="strategyPoolField">自定义股票池
                    <select id="strategyPoolID" onchange="updateStrategyPoolPreviews()">${customPoolOptionsHTML(poolID)}</select>
                </label>
                <label id="strategyMarketField">市场分组
                    <select id="strategyMarketPoolID" onchange="updateStrategyPoolPreviews()">${marketPoolOptionsHTML(poolID)}</select>
                </label>
            </div>
            <textarea id="strategySymbolsInput" rows="3" placeholder="手动输入股票代码，例如 000001, 600000">${escapeHTML((cfg.symbols || []).join('\n'))}</textarea>
            <div id="strategyUniversePreview">${poolPreviewHTML(poolID)}</div>
        </div>

        <div class="strategy-section">
            <div class="strategy-section-head">
                <strong>硬过滤条件</strong>
                <span>不满足这些条件的股票直接淘汰</span>
            </div>
            ${renderStrategyCheckRow('filterExcludePool', '排除股票池', !!exclude, `
                <label>排除哪个池
                    <select id="filterExcludePoolID" onchange="updateStrategyPoolPreviews()">${poolOptionsHTML(excludePoolID)}</select>
                </label>
                <div id="strategyExcludePreview">${poolPreviewHTML(excludePoolID)}</div>
            `)}
            ${renderStrategyCheckRow('filterMinAmount', '最低成交额', !!minAmount, `
                <label>成交额不低于
                    <input id="filterMinAmountValue" type="number" value="${Number(minAmount?.params?.value ?? 100000000)}" min="0" step="1000000">
                </label>
            `)}
            ${renderStrategyCheckRow('filterPriceRange', '收盘价区间', !!priceRange, `
                <label>最低价<input id="filterPriceMin" type="number" value="${Number(priceRange?.params?.min ?? 0)}" min="0" step="0.01"></label>
                <label>最高价<input id="filterPriceMax" type="number" value="${Number(priceRange?.params?.max ?? 9999)}" min="0" step="0.01"></label>
            `)}
            ${renderStrategyCheckRow('filterChangeRange', '涨跌幅区间', !!changeRange, `
                <label>最小涨跌幅 %<input id="filterChangeMin" type="number" value="${Number(changeRange?.params?.min ?? -10)}" step="0.1"></label>
                <label>最大涨跌幅 %<input id="filterChangeMax" type="number" value="${Number(changeRange?.params?.max ?? 10)}" step="0.1"></label>
            `)}
        </div>

        <div class="strategy-section">
            <div class="strategy-section-head">
                <strong>评分条件</strong>
                <span>命中条件后按权重加分</span>
            </div>
            ${renderStrategyCheckRow('scoreMaTrend', '均线多头', !!maTrend, `
                ${weightInput('scoreMaTrendWeight', maTrend?.weight ?? 20)}
                <label>短均线<input id="scoreMaShort" type="number" value="${Number(maTrend?.params?.short ?? 5)}" min="1"></label>
                <label>中均线<input id="scoreMaMid" type="number" value="${Number(maTrend?.params?.mid ?? 10)}" min="1"></label>
                <label>长均线<input id="scoreMaLong" type="number" value="${Number(maTrend?.params?.long ?? 20)}" min="1"></label>
            `)}
            ${renderStrategyCheckRow('scoreVolumeUp', '阶段放量', !!volumeUp, `
                ${weightInput('scoreVolumeWeight', volumeUp?.weight ?? 15)}
                <label>对比天数<input id="scoreVolumeDays" type="number" value="${Number(volumeUp?.params?.days ?? 5)}" min="1"></label>
                <label>放量倍数<input id="scoreVolumeRatio" type="number" value="${Number(volumeUp?.params?.ratio ?? 1.3)}" min="0" step="0.1"></label>
            `)}
            ${renderStrategyCheckRow('scoreBreakHigh', '突破新高', !!breakHigh, `
                ${weightInput('scoreBreakWeight', breakHigh?.weight ?? 15)}
                <label>回看天数<input id="scoreBreakDays" type="number" value="${Number(breakHigh?.params?.days ?? 20)}" min="1"></label>
            `)}
            ${renderStrategyCheckRow('scoreFormula', '公式因子', !!formulaRule, `
                ${weightInput('scoreFormulaWeight', formulaRule?.weight ?? 30)}
                <label>选择公式
                    <select id="scoreFormulaID">
                        <option value="">按公式名称匹配：${escapeHTML(formulaName || '未选择')}</option>
                        ${formulas.map(item => `<option value="${item.id}" ${selectedFormula?.id === item.id ? 'selected' : ''}>${escapeHTML(item.name)}</option>`).join('')}
                    </select>
                </label>
            `)}
        </div>

        <div class="strategy-section">
            <div class="strategy-section-head">
                <strong>通过规则</strong>
                <span>最后按总分筛选</span>
            </div>
            <div class="strategy-field-grid">
                <label>最低分
                    <input id="strategyMinScore" type="number" value="${Number(cfg.pass?.min_score ?? 60)}" min="0" step="1">
                </label>
                <label>最多保留
                    <input id="strategyTopN" type="number" value="${Number(cfg.pass?.top_n ?? 50)}" min="1" step="1">
                </label>
                <label>计算K线数量
                    <input id="strategyCalcCount" type="number" value="${Number(cfg.calc_count ?? 260)}" min="30" step="10">
                </label>
            </div>
        </div>
    `;
    handleStrategyUniverseChange();
}

function renderStrategyCheckRow(id, title, checked, body) {
    return `
        <div class="strategy-rule-row">
            <label class="check-line strategy-rule-toggle"><input id="${id}" type="checkbox" ${checked ? 'checked' : ''}> ${title}</label>
            <div class="strategy-rule-body">${body}</div>
        </div>
    `;
}

function weightInput(id, value) {
    return `<label>权重<input id="${id}" type="number" value="${Number(value)}" min="0" step="1"></label>`;
}

function handleStrategyUniverseChange() {
    const universe = document.getElementById('strategyUniverse')?.value || 'pool';
    const poolField = document.getElementById('strategyPoolField');
    const marketField = document.getElementById('strategyMarketField');
    const symbols = document.getElementById('strategySymbolsInput');
    if (poolField) poolField.style.display = universe === 'pool' ? 'block' : 'none';
    if (marketField) marketField.style.display = universe === 'market' ? 'block' : 'none';
    if (symbols) symbols.style.display = universe === 'symbols' ? 'block' : 'none';
    updateStrategyPoolPreviews();
}

function updateStrategyPoolPreviews() {
    const universePreview = document.getElementById('strategyUniversePreview');
    const excludePreview = document.getElementById('strategyExcludePreview');
    const universe = document.getElementById('strategyUniverse')?.value || 'pool';
    const poolID = document.getElementById('strategyPoolID')?.value || 'watchlist';
    const marketPoolID = document.getElementById('strategyMarketPoolID')?.value || 'market-all-a';
    const excludePoolID = document.getElementById('filterExcludePoolID')?.value || 'exclude';
    if (universePreview) {
        if (universe === 'symbols') {
            const symbols = (document.getElementById('strategySymbolsInput')?.value || '').split(/[\s,，]+/).filter(Boolean);
            universePreview.innerHTML = `<div class="strategy-pool-preview">手动输入：${symbols.length} 只股票${symbols.length ? `，${escapeHTML(symbols.slice(0, 18).join(', '))}` : ''}</div>`;
        } else if (universe === 'market') {
            universePreview.innerHTML = poolPreviewHTML(marketPoolID);
        } else if (universe === 'all_a') {
            universePreview.innerHTML = '<div class="strategy-pool-preview">全市场A股：首版运行时会限制部分代码，避免一次任务过慢。</div>';
        } else {
            universePreview.innerHTML = poolPreviewHTML(poolID);
        }
    }
    if (excludePreview) excludePreview.innerHTML = poolPreviewHTML(excludePoolID);
}

function renderStrategyVisualFromJSON() {
    try {
        renderStrategyVisualEditor(JSON.parse(document.getElementById('strategyConfig').value || '{}'));
    } catch (error) {
        alert('配置不是有效JSON：' + error.message);
    }
}

function syncStrategyFormToJSON() {
    const cfg = collectStrategyVisualConfig();
    document.getElementById('strategyConfig').value = prettyJSON(cfg);
    return cfg;
}

function collectStrategyVisualConfig() {
    const universe = document.getElementById('strategyUniverse')?.value || 'pool';
    const cfg = {
        universe,
        period: 'day',
        calc_count: numberFromInput('strategyCalcCount', 260),
        batch_size: 50,
        continue_on_error: true,
        filters: [],
        scores: [],
        pass: {
            min_score: numberFromInput('strategyMinScore', 60),
            top_n: numberFromInput('strategyTopN', 50)
        }
    };
    if (universe === 'pool') {
        cfg.pool_id = document.getElementById('strategyPoolID')?.value || 'watchlist';
    }
    if (universe === 'market') {
        cfg.pool_id = document.getElementById('strategyMarketPoolID')?.value || 'market-all-a';
    }
    if (universe === 'symbols') {
        cfg.symbols = (document.getElementById('strategySymbolsInput')?.value || '').split(/[\s,，]+/).filter(Boolean);
    }
    if (checked('filterExcludePool')) {
        cfg.filters.push({ id: 'exclude_pool', factor: 'pool_exclude', params: { pool_id: document.getElementById('filterExcludePoolID')?.value || 'exclude' } });
    }
    if (checked('filterMinAmount')) {
        cfg.filters.push({ id: 'min_amount', factor: 'min_amount', params: { value: numberFromInput('filterMinAmountValue', 100000000) } });
    }
    if (checked('filterPriceRange')) {
        cfg.filters.push({ id: 'price_range', factor: 'price_range', params: { min: numberFromInput('filterPriceMin', 0), max: numberFromInput('filterPriceMax', 9999) } });
    }
    if (checked('filterChangeRange')) {
        cfg.filters.push({ id: 'change_range', factor: 'change_range', params: { min: numberFromInput('filterChangeMin', -10), max: numberFromInput('filterChangeMax', 10) } });
    }
    if (checked('scoreMaTrend')) {
        cfg.scores.push({ id: 'ma_trend', factor: 'ma_trend', weight: numberFromInput('scoreMaTrendWeight', 20), params: { short: numberFromInput('scoreMaShort', 5), mid: numberFromInput('scoreMaMid', 10), long: numberFromInput('scoreMaLong', 20) } });
    }
    if (checked('scoreVolumeUp')) {
        cfg.scores.push({ id: 'volume_up', factor: 'volume_up', weight: numberFromInput('scoreVolumeWeight', 15), params: { days: numberFromInput('scoreVolumeDays', 5), ratio: numberFromInput('scoreVolumeRatio', 1.3) } });
    }
    if (checked('scoreBreakHigh')) {
        cfg.scores.push({ id: 'break_high', factor: 'break_high', weight: numberFromInput('scoreBreakWeight', 15), params: { days: numberFromInput('scoreBreakDays', 20) } });
    }
    if (checked('scoreFormula')) {
        const formulaID = document.getElementById('scoreFormulaID')?.value || '';
        const formula = formulas.find(item => item.id === formulaID);
        cfg.scores.push({ id: 'main_formula', factor: 'formula', weight: numberFromInput('scoreFormulaWeight', 30), params: formulaID ? { formula_id: formulaID } : { formula_name: formula?.name || '主力拉升' } });
    }
    return cfg;
}

function checked(id) {
    return !!document.getElementById(id)?.checked;
}

function numberFromInput(id, fallback) {
    const value = Number(document.getElementById(id)?.value);
    return Number.isFinite(value) ? value : fallback;
}

function newStrategy() {
    if (strategyRunState.running) return;
    selectedStrategyID = '';
    document.getElementById('strategyName').dataset.id = '';
    document.getElementById('strategyName').dataset.readonly = 'false';
    document.getElementById('strategyName').value = '我的选股策略';
    document.getElementById('strategyDescription').value = '';
    document.getElementById('strategyEnabled').checked = true;
    document.getElementById('strategyConfig').value = prettyJSON(defaultStrategyConfig());
    renderStrategyVisualEditor(defaultStrategyConfig());
    document.getElementById('strategyEditorHint').textContent = '新策略保存后可用于自动化任务';
    renderStrategyList();
}

function fillStrategy(id) {
    if (strategyRunState.running) return;
    const item = strategies.find(v => v.id === id);
    if (!item) return;
    selectedStrategyID = id;
    document.getElementById('strategyName').dataset.id = item.id;
    document.getElementById('strategyName').dataset.readonly = item.readonly ? 'true' : 'false';
    document.getElementById('strategyName').value = item.name || '';
    document.getElementById('strategyDescription').value = item.description || '';
    document.getElementById('strategyEnabled').checked = !!item.enabled;
    const cfg = parseStrategyConfig(item.config_json);
    document.getElementById('strategyConfig').value = prettyJSON(cfg);
    renderStrategyVisualEditor(cfg);
    document.getElementById('strategyEditorHint').textContent = item.readonly ? '内置模板只读，请复制后修改' : '可保存后在自动化任务中选择';
    renderStrategyList();
}

function currentStrategyPayload() {
    const id = document.getElementById('strategyName').dataset.id || '';
    const readonly = document.getElementById('strategyName').dataset.readonly === 'true';
    const config = syncStrategyFormToJSON();
    return {
        id,
        name: document.getElementById('strategyName').value,
        description: document.getElementById('strategyDescription').value,
        config_json: JSON.stringify(config),
        enabled: document.getElementById('strategyEnabled').checked,
        readonly
    };
}

function formatStrategyConfig() {
    try {
        document.getElementById('strategyConfig').value = prettyJSON(JSON.parse(document.getElementById('strategyConfig').value || '{}'));
    } catch (error) {
        alert('配置不是有效JSON：' + error.message);
    }
}

async function saveStrategy() {
    if (strategyRunState.running) return;
    try {
        const payload = currentStrategyPayload();
        if (payload.readonly) {
            alert('内置模板不能直接编辑，请先复制');
            return;
        }
        const item = await apiFetch(payload.id ? `/api/strategies/${payload.id}` : '/api/strategies', {
            method: payload.id ? 'PUT' : 'POST',
            body: JSON.stringify(payload)
        });
        selectedStrategyID = item.id;
        await loadStrategies();
        fillStrategy(item.id);
        alert('策略已保存');
    } catch (error) {
        alert(error.message);
    }
}

async function cloneCurrentStrategy() {
    if (strategyRunState.running) return;
    const id = document.getElementById('strategyName').dataset.id || selectedStrategyID;
    if (!id) {
        alert('请先选择一个策略');
        return;
    }
    try {
        const item = await apiFetch(`/api/strategies/${id}/clone`, { method: 'POST' });
        selectedStrategyID = item.id;
        await loadStrategies();
        fillStrategy(item.id);
        alert('策略已复制');
    } catch (error) {
        alert(error.message);
    }
}

async function deleteCurrentStrategy() {
    if (strategyRunState.running) return;
    const id = document.getElementById('strategyName').dataset.id || selectedStrategyID;
    const readonly = document.getElementById('strategyName').dataset.readonly === 'true';
    if (!id) return;
    if (readonly) {
        alert('内置模板不能删除');
        return;
    }
    if (!confirm('确认删除这个策略？')) return;
    try {
        await apiFetch(`/api/strategies/${id}`, { method: 'DELETE' });
        selectedStrategyID = '';
        await loadStrategies();
        if (strategies.length) fillStrategy(strategies[0].id);
    } catch (error) {
        alert(error.message);
    }
}

async function runCurrentStrategy() {
    const id = document.getElementById('strategyName').dataset.id || selectedStrategyID;
    if (!id) {
        alert('请先保存或选择一个策略');
        return;
    }
    const list = document.getElementById('strategyRunResults');
    startStrategyRunUI(id);
    if (list) list.innerHTML = '';
    try {
        const run = await apiFetch(`/api/strategies/${id}/run`, { method: 'POST' });
        const result = JSON.parse(run.result_json || '{}');
        const items = result.items || [];
        finishStrategyRunUI('success', `完成 · 扫描 ${result.total || 0} 只 · 命中 ${run.matched_count || items.length} 只`);
        if (list) {
            list.innerHTML = items.slice(0, 80).map(item => `
                <div class="data-item">
                    <div class="result-symbol">${escapeHTML(item.symbol)}</div>
                    <div class="data-item-meta">评分 <span class="result-latest">${Number(item.score || 0).toFixed(2)}</span> · 最新价 ${Number(item.latest || 0).toFixed(2)}</div>
                    <div class="strategy-reasons">${(item.reasons || []).slice(0, 4).map(reason => `<span>${escapeHTML(reason)}</span>`).join('')}</div>
                    <div class="item-actions">
                        <button class="primary" onclick="openResultChart('${escapeJSString(item.symbol)}')">打开图表</button>
                        <button onclick="addSymbolToPool('watchlist', '${escapeJSString(item.symbol)}')">观察</button>
                    </div>
                </div>
            `).join('') || '<div class="data-item">未命中标的</div>';
        }
        await Promise.all([loadRuns(), loadSelectionResults()]);
    } catch (error) {
        finishStrategyRunUI('failed', error.message);
        alert(error.message);
    }
}

function startStrategyRunUI(strategyID) {
    stopStrategyProgressTimer();
    strategyRunState = {
        running: true,
        strategyID,
        stepIndex: 0,
        timer: null,
        message: '准备数据',
        progress: 8
    };
    setStrategyEditorLocked(true);
    renderStrategyList();
    renderStrategyProgressPanel();
    strategyRunState.timer = setInterval(advanceStrategyRunStep, 1400);
}

function advanceStrategyRunStep() {
    const steps = strategyProgressSteps();
    if (!strategyRunState.running) return;
    strategyRunState.stepIndex = Math.min(strategyRunState.stepIndex + 1, steps.length - 1);
    const current = steps[strategyRunState.stepIndex];
    strategyRunState.message = current.label;
    strategyRunState.progress = current.progress;
    renderStrategyList();
    renderStrategyProgressPanel();
}

function finishStrategyRunUI(status, message) {
    stopStrategyProgressTimer();
    strategyRunState.running = false;
    strategyRunState.message = message;
    strategyRunState.progress = status === 'success' ? 100 : strategyRunState.progress;
    setStrategyEditorLocked(false);
    renderStrategyList();
    const summary = document.getElementById('strategyRunSummary');
    if (summary) {
        summary.className = `data-item strategy-run-summary ${status}`;
        summary.innerHTML = escapeHTML(message || (status === 'success' ? '完成' : '运行失败'));
    }
}

function stopStrategyProgressTimer() {
    if (strategyRunState.timer) {
        clearInterval(strategyRunState.timer);
    }
    strategyRunState.timer = null;
}

function strategyProgressSteps() {
    return [
        { label: '准备数据', progress: 8 },
        { label: '加载K线', progress: 24 },
        { label: '计算公式', progress: 46 },
        { label: '计算因子', progress: 64 },
        { label: '评分排序', progress: 82 },
        { label: '写入结果', progress: 94 }
    ];
}

function renderStrategyProgressPanel() {
    const summary = document.getElementById('strategyRunSummary');
    if (!summary) return;
    const steps = strategyProgressSteps();
    summary.className = 'data-item strategy-run-summary running';
    summary.innerHTML = `
        <div class="strategy-progress-head">
            <strong>运行中：${escapeHTML(strategyRunState.message)}</strong>
            <span>${strategyRunState.progress}%</span>
        </div>
        <div class="strategy-progress-bar"><i style="width:${strategyRunState.progress}%"></i></div>
        <div class="strategy-step-list">
            ${steps.map((step, index) => `<span class="${index <= strategyRunState.stepIndex ? 'active' : ''}">${escapeHTML(step.label)}</span>`).join('')}
        </div>
    `;
}

function setStrategyEditorLocked(locked) {
    const ids = [
        'strategySaveButton',
        'strategyCloneButton',
        'strategyRunButton',
        'strategyDeleteButton',
        'strategyApplyJSONButton',
        'strategySyncJSONButton',
        'strategyFormatJSONButton',
        'strategyName',
        'strategyEnabled',
        'strategyDescription',
        'strategyConfig'
    ];
    ids.forEach(id => {
        const node = document.getElementById(id);
        if (node) node.disabled = locked;
    });
    const runButton = document.getElementById('strategyRunButton');
    if (runButton) runButton.textContent = locked ? '运行中...' : '运行';
    document.querySelectorAll('#strategyVisualEditor input, #strategyVisualEditor select, #strategyVisualEditor textarea, #strategyVisualEditor button, .advanced-json-panel button').forEach(node => {
        node.disabled = locked;
    });
    const editor = document.querySelector('.strategy-editor-card');
    if (editor) editor.classList.toggle('locked', locked);
}

function appendStrategyFactor(factorID) {
    const factor = factors.find(item => item.id === factorID);
    if (!factor) return;
    try {
        const cfg = JSON.parse(document.getElementById('strategyConfig').value || '{}');
        const params = {};
        (factor.params || []).forEach(param => {
            if (param.default !== '') params[param.name] = param.default;
        });
        const rule = {
            id: `${factor.id}_${Date.now().toString(36)}`,
            factor: factor.id,
            params
        };
        if (factor.kind === 'score') rule.weight = 10;
        if (factor.kind === 'filter') {
            cfg.filters = Array.isArray(cfg.filters) ? cfg.filters : [];
            cfg.filters.push(rule);
        } else {
            cfg.scores = Array.isArray(cfg.scores) ? cfg.scores : [];
            cfg.scores.push(rule);
        }
        document.getElementById('strategyConfig').value = prettyJSON(cfg);
    } catch (error) {
        alert('请先修正策略配置JSON：' + error.message);
    }
}

async function loadAutomationData() {
    await Promise.all([loadFormulaList(), loadPools(), loadStrategies(), loadWebhooks(), loadAutomations(), loadRuns(), loadSelectionResults()]);
    updateAutomationPayloadMode();
}

async function loadAutomations() {
    automations = await apiFetch('/api/automations') || [];
    document.getElementById('automationList').innerHTML = automations.map(t => `
        <div class="data-item">
            <div class="data-item-title">${escapeHTML(t.name)}${t.readonly ? ' <span class="tag">系统固定</span>' : ''}</div>
            <div class="data-item-meta">${escapeHTML(t.type)} · ${escapeHTML(t.cron)} · ${t.enabled ? '启用' : '停用'}${t.next_run_at ? ` · 下次：${escapeHTML(t.next_run_at)}` : ''}</div>
            <div class="data-item-meta">上次：${escapeHTML(t.last_status || '--')} ${escapeHTML(t.last_message || '')}</div>
            <div class="item-actions">${automationTaskActions(t)}</div>
        </div>
    `).join('') || '<div class="data-item">暂无任务</div>';
}

function automationTaskActions(task) {
    const id = escapeJSString(task.id);
    if (task.readonly) {
        return `
            <button class="${task.enabled ? '' : 'primary'}" onclick="toggleAutomation('${id}', ${task.enabled ? 'false' : 'true'})">${task.enabled ? '关闭' : '开启'}</button>
            <button onclick="runAutomation('${id}')">立即执行一次</button>
        `;
    }
    return `
        <button onclick="fillAutomation('${id}')">编辑</button>
        <button class="primary" onclick="runAutomation('${id}')">立即运行</button>
        <button onclick="deleteAutomation('${id}')">删除</button>
    `;
}

function fillAutomation(id) {
    const t = automations.find(item => item.id === id);
    if (!t) return;
    if (t.readonly) {
        alert('固定任务只能开启或关闭，不能编辑');
        return;
    }
    const payload = JSON.parse(t.payload_json || '{}');
    document.getElementById('automationName').dataset.id = t.id;
    document.getElementById('automationName').value = t.name;
    document.getElementById('automationType').value = t.type || 'stock_selection';
    document.getElementById('automationFormula').value = payload.formula_id || '';
    document.getElementById('automationPool').value = payload.pool_id || '';
    const strategySelect = document.getElementById('automationStrategy');
    if (strategySelect) strategySelect.value = payload.strategy_id || '';
    document.getElementById('automationPayload').value = JSON.stringify(payload, null, 2);
    document.getElementById('automationCron').value = t.cron;
    document.getElementById('automationEnabled').checked = !!t.enabled;
    const ids = JSON.parse(t.webhook_ids || '[]');
    Array.from(document.getElementById('automationWebhook').options).forEach(opt => {
        opt.selected = ids.includes(opt.value);
    });
    updateAutomationPayloadMode();
}

function updateAutomationPayloadMode() {
    const type = document.getElementById('automationType')?.value || 'stock_selection';
    const stockFields = document.getElementById('stockSelectionFields');
    const strategyFields = document.getElementById('strategySelectionFields');
    const payloadBox = document.getElementById('automationPayload');
    if (stockFields) stockFields.style.display = type === 'stock_selection' ? 'block' : 'none';
    if (strategyFields) strategyFields.style.display = type === 'strategy_selection' ? 'block' : 'none';
    if (!payloadBox) return;
    payloadBox.style.display = (type === 'stock_selection' || type === 'strategy_selection') ? 'none' : 'block';
    if (!payloadBox.value.trim()) {
        payloadBox.value = type === 'system_sync'
            ? JSON.stringify({ scope: 'all', tables: ['day'], limit: 4, max_codes: 200, continue_on_error: true }, null, 2)
            : JSON.stringify({ action: 'noop', data: {} }, null, 2);
    }
}

async function saveAutomation() {
    try {
        const id = document.getElementById('automationName').dataset.id || '';
        const type = document.getElementById('automationType').value || 'stock_selection';
        const webhookIds = Array.from(document.getElementById('automationWebhook').selectedOptions).map(opt => opt.value);
        let payload;
        if (type === 'stock_selection') {
            payload = {
                formula_id: document.getElementById('automationFormula').value,
                pool_id: document.getElementById('automationPool').value,
                calc_count: 240,
                out_count: 1,
                batch_size: 50,
                continue_on_error: true
            };
        } else if (type === 'strategy_selection') {
            payload = {
                strategy_id: document.getElementById('automationStrategy').value
            };
        } else {
            payload = JSON.parse(document.getElementById('automationPayload').value || '{}');
        }
        await apiFetch(id ? `/api/automations/${id}` : '/api/automations', {
            method: id ? 'PUT' : 'POST',
            body: JSON.stringify({
                id,
                name: document.getElementById('automationName').value,
                type,
                cron: document.getElementById('automationCron').value,
                enabled: document.getElementById('automationEnabled').checked,
                payload_json: JSON.stringify(payload),
                webhook_ids: JSON.stringify(webhookIds)
            })
        });
        document.getElementById('automationName').dataset.id = '';
        await loadAutomations();
        alert('任务已保存');
    } catch (error) {
        alert(error.message);
    }
}

async function runAutomation(id) {
    try {
        await apiFetch(`/api/automations/${id}/run`, { method: 'POST' });
        await Promise.all([loadAutomations(), loadRuns(), loadSelectionResults()]);
        alert('任务执行完成');
    } catch (error) {
        alert(error.message);
        await loadRuns();
    }
}

async function toggleAutomation(id, enabled) {
    try {
        await apiFetch(`/api/automations/${id}/enabled`, {
            method: 'PUT',
            body: JSON.stringify({ enabled })
        });
        await loadAutomations();
    } catch (error) {
        alert(error.message);
    }
}

async function createSystemTemplate(template) {
    try {
        await apiFetch('/api/automations/templates', {
            method: 'POST',
            body: JSON.stringify({ template })
        });
        await loadAutomations();
        alert('系统任务模板已创建');
    } catch (error) {
        alert(error.message);
    }
}

async function deleteAutomation(id) {
    if (!confirm('确认删除这个任务？')) return;
    await apiFetch(`/api/automations/${id}`, { method: 'DELETE' });
    await loadAutomations();
}

async function loadRuns() {
    const runs = await apiFetch('/api/automations/runs?limit=30') || [];
    document.getElementById('runList').innerHTML = runs.map(r => `
        <div class="data-item">
            <div class="data-item-title">${escapeHTML(r.task_name)} · ${escapeHTML(r.status)}</div>
            <div class="data-item-meta">${escapeHTML(r.started_at)} · 命中 ${r.matched_count}</div>
            <div class="data-item-meta">${escapeHTML(r.log || (r.result_json || '').slice(0, 240))}</div>
        </div>
    `).join('') || '<div class="data-item">暂无运行记录</div>';
}

async function loadSelectionResults() {
    const list = document.getElementById('selectionResultList');
    if (!list) return;
    const formulaID = document.getElementById('resultFormulaFilter')?.value || '';
    const symbol = document.getElementById('resultSymbolFilter')?.value || '';
    const latest = document.getElementById('resultLatestFilter')?.value || '';
    const params = new URLSearchParams({ limit: '200' });
    if (formulaID) params.set('formula_id', formulaID);
    if (symbol) params.set('symbol', symbol);
    if (latest) params.set('latest', latest);
    selectionResults = await apiFetch(`/api/selection-results?${params.toString()}`) || [];
    list.innerHTML = selectionResults.map(item => `
        <div class="data-item">
            <div class="result-symbol">${escapeHTML(item.symbol)}</div>
            <div class="data-item-meta">${escapeHTML(item.formula_name)} · ${escapeHTML(item.task_name)}</div>
            <div class="data-item-meta">${escapeHTML(item.created_at)} · 最新值 <span class="result-latest">${Number(item.latest || 0).toFixed(4)}</span></div>
            <div class="item-actions">
                <button class="primary" onclick="openResultChart('${item.symbol}')">打开图表</button>
                <button onclick="showResultDetail('${item.id}')">详情</button>
                <button onclick="addSymbolToPool('watchlist', '${item.symbol}')">观察</button>
                <button onclick="addSymbolToPool('exclude', '${item.symbol}')">排除</button>
            </div>
        </div>
    `).join('') || '<div class="data-item">暂无命中结果</div>';
}

async function refreshDecisionDesk() {
    const hitSummary = document.getElementById('decisionHitSummary');
    const hitList = document.getElementById('decisionHits');
    if (hitSummary) hitSummary.textContent = '加载中';
    if (hitList) hitList.innerHTML = '<div class="data-item">正在加载命中结果...</div>';
    try {
        if (!pools.length) {
            await loadPools();
        }
        await loadDecisionResults();
        renderDecisionPools();
        if (!selectedDecisionResult && decisionResults.length > 0) {
            await selectDecisionResult(decisionResults[0].id);
        } else {
            updateDecisionActionState();
        }
    } catch (error) {
        if (hitSummary) hitSummary.textContent = '加载失败';
        if (hitList) hitList.innerHTML = `<div class="data-item">${escapeHTML(error.message || error)}</div>`;
    }
}

async function loadDecisionResults() {
    const today = localDateString();
    dailyReview = await apiFetch('/api/daily-review?limit=200') || {};
    reviewItems = dailyReview.items || [];
    const allResults = reviewItems.map(item => item.result).filter(Boolean);
    const todayItems = allResults.filter(item => String(item.created_at || '').startsWith(today));
    decisionResults = todayItems.length ? todayItems : allResults;
    decisionShowingToday = todayItems.length > 0;
    if (selectedDecisionResult) {
        selectedReviewItem = reviewItemBySymbol(selectedDecisionResult.symbol);
        const latestSelected = decisionResults.find(item => normalizeSymbol(item.symbol) === normalizeSymbol(selectedDecisionResult.symbol));
        if (latestSelected) selectedDecisionResult = latestSelected;
    }
    renderDecisionHits(todayItems.length > 0, allResults.length);
}

function renderDecisionHits(isToday, latestCount) {
    const summary = document.getElementById('decisionHitSummary');
    const list = document.getElementById('decisionHits');
    if (!list) return;
    if (summary) {
        if (decisionResults.length === 0) {
            summary.textContent = '暂无选股命中，去自动化页运行一次选股任务';
        } else {
            summary.textContent = isToday
                ? `今日命中 ${decisionResults.length} 只`
                : `今日暂无命中，显示最近一次运行的 ${latestCount} 只`;
        }
    }
    list.innerHTML = decisionResults.map(item => {
        const review = reviewItemBySymbol(item.symbol);
        const statusInfo = decisionStatusInfo(item.symbol, review);
        const total = review?.score?.total || 0;
        const track = review?.track?.summary || '待跟踪';
        return `
            <button class="decision-hit ${selectedDecisionResult?.id === item.id ? 'active' : ''}" onclick="selectDecisionResult('${item.id}')">
                <span class="decision-hit-top">
                    <span class="decision-symbol">${escapeHTML(item.symbol)}</span>
                    <span class="score-total">${total ? total + '分' : '--'}</span>
                </span>
                <span class="decision-hit-meta">${escapeHTML(item.formula_name)} · ${escapeHTML(item.task_name || '--')}</span>
                <span class="decision-hit-foot">
                    <span>${escapeHTML(formatDecisionTime(item.created_at))}</span>
                    <strong>${Number(item.latest || 0).toFixed(4)}</strong>
                    <em class="${statusInfo.className}">${statusInfo.label}</em>
                </span>
                <span class="decision-hit-meta">${escapeHTML(track)}</span>
            </button>
        `;
    }).join('') || '<div class="data-item">暂无命中结果</div>';
}

function renderDecisionPools() {
    renderPoolChips('watchlist', 'watchPoolList', 'watchPoolSummary');
    renderPoolChips('exclude', 'excludePoolList', 'excludePoolSummary');
}

function renderPoolChips(poolID, listID, summaryID) {
    const pool = poolByID(poolID);
    const list = document.getElementById(listID);
    const summary = document.getElementById(summaryID);
    const symbols = pool?.symbols || [];
    if (summary) summary.textContent = `${symbols.length} 只`;
    if (!list) return;
    list.innerHTML = symbols.slice(0, 80).map(symbol => `
        <button class="pool-chip" onclick="openDecisionSymbol('${normalizeSymbol(symbol)}')">
            ${escapeHTML(symbol)}
        </button>
    `).join('') || '<div class="data-item compact-empty">暂无股票</div>';
}

async function selectDecisionResult(id) {
    const item = decisionResults.find(v => v.id === id);
    if (!item) return;
    selectedDecisionResult = item;
    selectedReviewItem = reviewItems.find(v => v.result?.id === id) || reviewItemBySymbol(item.symbol);
    currentStock = item.symbol;
    const input = document.getElementById('stockCode');
    if (input) input.value = item.symbol;
    renderDecisionHits(decisionShowingToday, decisionResults.length);
    renderDecisionDetail(item);
    await loadDecisionChart(item.symbol);
}

async function openDecisionSymbol(symbol) {
    const normalized = normalizeSymbol(symbol);
    const existing = decisionResults.find(item => normalizeSymbol(item.symbol) === normalized);
    if (existing) {
        await selectDecisionResult(existing.id);
        return;
    }
    selectedDecisionResult = { id: '', symbol: normalized, detail_json: '{}', formula_name: '手动查看', task_name: '股票池' };
    selectedReviewItem = reviewItemBySymbol(normalized) || { result: selectedDecisionResult, note: {}, score: {}, track: {} };
    currentStock = normalized;
    renderDecisionDetail(selectedDecisionResult);
    await loadDecisionChart(normalized);
}

function renderDecisionDetail(item) {
    const symbol = normalizeSymbol(item.symbol);
    const review = selectedReviewItem || reviewItemBySymbol(symbol) || {};
    document.getElementById('decisionDetailTitle').textContent = `${symbol} 命中详情`;
    document.getElementById('decisionDetailMeta').textContent = `${item.formula_name || '--'} · ${item.task_name || '--'} · ${formatDecisionTime(item.created_at)}`;
    document.getElementById('decisionScore').innerHTML = renderScoreGrid(review.score);
    document.getElementById('decisionTrack').innerHTML = renderTrackPanel(review.track);
    document.getElementById('decisionReason').innerHTML = renderDecisionReason(item);
    fillDecisionNoteForm(review.note || { symbol });
    updateDecisionActionState();
}

function renderDecisionReason(item) {
    let detail = {};
    try {
        detail = JSON.parse(item.detail_json || '{}');
    } catch {
        return `<pre class="json-output">${escapeHTML(item.detail_json || '')}</pre>`;
    }
    const rows = extractReasonRows(detail);
    if (!rows.length) {
        return `<pre class="json-output">${escapeHTML(prettyJSON(detail))}</pre>`;
    }
    return rows.map(row => `
        <div class="reason-item">
            <span>${escapeHTML(row.label)}</span>
            <strong>${escapeHTML(compactValue(row.value))}</strong>
        </div>
    `).join('');
}

function extractReasonRows(detail) {
    const rows = [];
    const push = (label, value) => {
        if (value !== undefined && value !== null && value !== '') rows.push({ label, value });
    };
    push('最新输出', detail.latest ?? detail.Latest ?? detail.value ?? detail.Value);
    push('是否命中', detail.hit ?? detail.Hit ?? detail.signal ?? detail.Signal);
    push('公式引擎', detail.engine ?? detail.Engine);
    push('耗时', detail.tick_ms !== undefined ? `${detail.tick_ms} ms` : undefined);
    const data = detail.data ?? detail.Data;
    if (Array.isArray(data)) {
        data.slice(-6).forEach((value, index) => push(`输出 ${data.length - 6 + index + 1}`, value));
    } else if (data && typeof data === 'object') {
        Object.entries(data).slice(0, 16).forEach(([key, value]) => push(key, value));
    }
    Object.entries(detail).forEach(([key, value]) => {
        if (rows.length >= 18) return;
        if (['latest', 'Latest', 'value', 'Value', 'hit', 'Hit', 'signal', 'Signal', 'data', 'Data'].includes(key)) return;
        if (typeof value !== 'object') push(key, value);
    });
    return rows;
}

async function loadDecisionChart(symbol) {
    const chartDom = document.getElementById('decisionChart');
    if (!chartDom) return;
    chartDom.innerHTML = '';
    try {
        renderHQKLine('decisionChart', symbol, 'day', {
            count: 260,
            pageSize: 80,
            windows: [
                { Index: 'MA' },
                { Index: 'VOL' }
            ]
        });
    } catch (error) {
        chartDom.innerHTML = `<div class="data-item">${escapeHTML(error.message || error)}</div>`;
    }
}

async function addDecisionSymbolToPool(poolID) {
    if (!selectedDecisionResult?.symbol) return;
    await addSymbolToPool(poolID, selectedDecisionResult.symbol);
}

async function removeDecisionSymbolFromPools() {
    if (!selectedDecisionResult?.symbol) return;
    await Promise.allSettled([
        removeSymbolFromPool('watchlist', selectedDecisionResult.symbol, false),
        removeSymbolFromPool('exclude', selectedDecisionResult.symbol, false)
    ]);
    await upsertDecisionNote({ symbol: selectedDecisionResult.symbol, status: '' });
    await loadPools();
    await loadDecisionResults();
    renderDecisionPools();
    renderDecisionHits(decisionShowingToday, decisionResults.length);
    if (selectedDecisionResult) {
        selectedReviewItem = reviewItemBySymbol(selectedDecisionResult.symbol);
        renderDecisionDetail(selectedDecisionResult);
    }
    updateDecisionActionState();
}

async function addSymbolToPool(poolID, symbol) {
    const normalized = normalizeSymbol(symbol);
    if (!normalized) return;
    const opposite = poolID === 'watchlist' ? 'exclude' : 'watchlist';
    await removeSymbolFromPool(opposite, normalized, false);
    await apiFetch(`/api/stock-pools/${poolID}/symbols/${encodeURIComponent(normalized)}`, { method: 'POST' });
    await loadPools();
    await loadDecisionResults();
    renderDecisionPools();
    renderDecisionHits(decisionShowingToday, decisionResults.length);
    if (selectedDecisionResult && normalizeSymbol(selectedDecisionResult.symbol) === normalized) {
        selectedReviewItem = reviewItemBySymbol(normalized);
        renderDecisionDetail(selectedDecisionResult);
    }
    updateDecisionActionState();
}

async function removeSymbolFromPool(poolID, symbol, refresh = true) {
    const normalized = normalizeSymbol(symbol);
    if (!normalized) return;
    await apiFetch(`/api/stock-pools/${poolID}/symbols/${encodeURIComponent(normalized)}`, { method: 'DELETE' });
    if (refresh) {
        await loadPools();
        await loadDecisionResults();
        renderDecisionPools();
        renderDecisionHits(decisionShowingToday, decisionResults.length);
        updateDecisionActionState();
    }
}

function updateDecisionActionState() {
    const symbol = selectedDecisionResult?.symbol || '';
    const watchButton = document.getElementById('decisionWatchButton');
    const excludeButton = document.getElementById('decisionExcludeButton');
    const removeButton = document.getElementById('decisionRemoveButton');
    const inWatch = symbolInPool(symbol, 'watchlist');
    const inExclude = symbolInPool(symbol, 'exclude');
    if (watchButton) {
        watchButton.disabled = !symbol || inWatch;
        watchButton.textContent = inWatch ? '已在观察' : '加入观察';
    }
    if (excludeButton) {
        excludeButton.disabled = !symbol || inExclude;
        excludeButton.textContent = inExclude ? '已排除' : '排除';
    }
    if (removeButton) {
        removeButton.disabled = !symbol || (!inWatch && !inExclude);
    }
}

function reviewItemBySymbol(symbol) {
    const normalized = normalizeSymbol(symbol);
    return reviewItems.find(item => normalizeSymbol(item.result?.symbol || item.note?.symbol) === normalized);
}

function decisionStatusInfo(symbol, review) {
    const status = review?.status || review?.note?.status || '';
    const inWatch = review?.watch || symbolInPool(symbol, 'watchlist') || status === 'watch';
    const inExclude = review?.excluded || symbolInPool(symbol, 'exclude') || status === 'exclude';
    if (inExclude) return { label: '已排除', className: 'excluded' };
    if (inWatch) return { label: '观察中', className: 'watched' };
    return { label: '待处理', className: 'pending' };
}

function renderScoreGrid(score = {}) {
    const items = [
        ['总分', score.total],
        ['趋势', score.trend],
        ['量能', score.volume],
        ['位置', score.place],
        ['风险', score.risk]
    ];
    return items.map(([label, value]) => `
        <div class="score-item ${label === '总分' ? 'score-item-total' : ''}">
            <span>${escapeHTML(label)}</span>
            <strong>${value ? escapeHTML(value) : '--'}</strong>
        </div>
    `).join('');
}

function renderTrackPanel(track = {}) {
    if (!track || !track.available) {
        return `<div class="track-empty">${escapeHTML(track?.summary || '暂无次日跟踪')}</div>`;
    }
    const rows = [
        ['日期', track.date],
        ['开盘', formatPercent(track.open_change)],
        ['最高', formatPercent(track.max_gain)],
        ['回撤', formatPercent(track.drawdown)],
        ['收盘', formatPercent(track.close_change)]
    ];
    return `
        <div class="track-title">${escapeHTML(track.summary || '次日跟踪')}</div>
        <div class="track-grid">
            ${rows.map(([label, value]) => `
                <div>
                    <span>${escapeHTML(label)}</span>
                    <strong>${escapeHTML(value)}</strong>
                </div>
            `).join('')}
        </div>
    `;
}

function fillDecisionNoteForm(note = {}) {
    const fields = {
        decisionAddedPrice: note.added_price,
        decisionPlanBuy: note.plan_buy,
        decisionStopLoss: note.stop_loss,
        decisionAddReason: note.add_reason || '',
        decisionReviewNote: note.review_note || '',
        decisionExcludeCategory: note.exclude_category || '',
        decisionExcludeReason: note.exclude_reason || ''
    };
    Object.entries(fields).forEach(([id, value]) => {
        const node = document.getElementById(id);
        if (!node) return;
        node.value = value && value !== 0 ? value : '';
    });
}

function numberInputValue(id) {
    const value = document.getElementById(id)?.value;
    if (value === undefined || value === null || value === '') return 0;
    const num = Number(value);
    return Number.isFinite(num) ? num : 0;
}

async function upsertDecisionNote(notePatch) {
    const symbol = normalizeSymbol(notePatch.symbol);
    if (!symbol) return null;
    const current = reviewItemBySymbol(symbol)?.note || selectedReviewItem?.note || {};
    const payload = {
        ...current,
        ...notePatch,
        symbol
    };
    return apiFetch(`/api/decision-notes/${encodeURIComponent(symbol)}`, {
        method: 'PUT',
        body: JSON.stringify(payload)
    });
}

async function saveDecisionNoteFromForm() {
    if (!selectedDecisionResult?.symbol) return;
    try {
        const symbol = normalizeSymbol(selectedDecisionResult.symbol);
        const statusInfo = decisionStatusInfo(symbol, selectedReviewItem);
        const status = statusInfo.className === 'watched' ? 'watch' : (statusInfo.className === 'excluded' ? 'exclude' : (selectedReviewItem?.note?.status || ''));
        await upsertDecisionNote({
            symbol,
            status,
            added_price: numberInputValue('decisionAddedPrice'),
            plan_buy: numberInputValue('decisionPlanBuy'),
            stop_loss: numberInputValue('decisionStopLoss'),
            add_reason: document.getElementById('decisionAddReason')?.value || '',
            review_note: document.getElementById('decisionReviewNote')?.value || '',
            exclude_category: document.getElementById('decisionExcludeCategory')?.value || '',
            exclude_reason: document.getElementById('decisionExcludeReason')?.value || ''
        });
        await loadDecisionResults();
        selectedReviewItem = reviewItemBySymbol(symbol);
        renderDecisionDetail(selectedDecisionResult);
        renderDecisionHits(decisionShowingToday, decisionResults.length);
        renderDailyReview(dailyReview);
        alert('记录已保存');
    } catch (error) {
        alert(error.message || error);
    }
}

async function loadDailyReview() {
    const summary = document.getElementById('dailyReviewSummary');
    if (summary) summary.textContent = '加载中';
    setLoadingText('dailyReviewList', '正在加载复盘数据...');
    try {
        if (!pools.length) await loadPools();
        dailyReview = await apiFetch('/api/daily-review?limit=200') || {};
        reviewItems = dailyReview.items || [];
        renderDailyReview(dailyReview);
    } catch (error) {
        if (summary) summary.textContent = '加载失败';
        setErrorText('dailyReviewList', error);
    }
}

function renderDailyReview(data = {}) {
    const summary = document.getElementById('dailyReviewSummary');
    const list = document.getElementById('dailyReviewList');
    if (!list) return;
    const items = data.items || [];
    const info = data.summary || {};
    if (summary) {
        summary.textContent = `${escapeHTML(data.date || localDateString())} · 命中 ${info.hits || 0} 只 · 已处理 ${info.handled_count || 0} 只`;
    }
    renderMetricCards('dailyReviewMetrics', [
        { label: '今日命中', value: info.hits || 0, note: '最近一次选股结果' },
        { label: '已处理', value: info.handled_count || 0, note: '观察、排除或已记录' },
        { label: '观察池', value: info.watch_count || 0, note: '当前跟踪标的' },
        { label: '排除池', value: info.exclude_count || 0, note: '今日不再跟踪' },
        { label: '平均评分', value: info.avg_score || '--', note: '趋势/量能/位置/风险' },
        { label: '次日胜率', value: info.tracked_count ? formatPercent(info.win_rate) : '--', note: `${info.tracked_count || 0} 只已跟踪` },
        { label: '平均收盘', value: info.tracked_count ? formatPercent(info.avg_close_change) : '--', note: '次日收盘表现' }
    ]);
    list.innerHTML = items.map(item => renderReviewCard(item)).join('') || '<div class="data-item">暂无复盘数据</div>';
}

function renderReviewCard(item) {
    const result = item.result || {};
    const statusInfo = decisionStatusInfo(result.symbol, item);
    const note = item.note || {};
    const reason = note.exclude_reason || note.add_reason || note.review_note || '暂无记录';
    return `
        <div class="data-item daily-review-card">
            <div class="review-card-head">
                <div>
                    <div class="result-symbol">${escapeHTML(result.symbol || '--')}</div>
                    <div class="data-item-meta">${escapeHTML(result.formula_name || '--')} · ${escapeHTML(result.task_name || '--')}</div>
                </div>
                <span class="review-status ${statusInfo.className}">${escapeHTML(statusInfo.label)}</span>
            </div>
            <div class="score-grid">${renderScoreGrid(item.score)}</div>
            <div class="track-panel">${renderTrackPanel(item.track)}</div>
            <div class="data-item-meta">记录：${escapeHTML(reason)}</div>
            <div class="item-actions">
                <button class="primary" onclick="selectReviewItem('${escapeJSString(result.id || '')}')">查看</button>
                <button onclick="addSymbolToPool('watchlist', '${escapeJSString(result.symbol || '')}')">观察</button>
                <button onclick="addSymbolToPool('exclude', '${escapeJSString(result.symbol || '')}')">排除</button>
            </div>
        </div>
    `;
}

async function selectReviewItem(id) {
    const review = reviewItems.find(item => item.result?.id === id);
    if (!review?.result) return;
    switchWorkspace('market', document.querySelectorAll('.workspace-tab')[0]);
    await selectDecisionResult(review.result.id);
}

function formatPercent(value) {
    if (value === null || value === undefined || value === '' || Number.isNaN(Number(value))) return '--';
    const num = Number(value);
    return `${num > 0 ? '+' : ''}${num.toFixed(2)}%`;
}

function formatDecisionTime(value) {
    if (!value) return '--';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value);
    return date.toLocaleString('zh-CN', { hour12: false });
}

function openResultChart(symbol) {
    document.getElementById('hqSymbol').value = symbol;
    switchWorkspace('proChart', document.querySelectorAll('.workspace-tab')[1]);
    loadHQChart();
}

function showResultDetail(id) {
    const item = selectionResults.find(v => v.id === id);
    if (!item) return;
    try {
        document.getElementById('formulaTestOutput').textContent = prettyJSON(JSON.parse(item.detail_json || '{}'));
    } catch {
        document.getElementById('formulaTestOutput').textContent = item.detail_json || '';
    }
    openResultChart(item.symbol);
}

async function loadWebhooks() {
    webhooks = await apiFetch('/api/webhooks') || [];
    const select = document.getElementById('automationWebhook');
    if (select) {
        select.innerHTML = webhooks.map(h => `<option value="${h.id}">${escapeHTML(h.name)}</option>`).join('');
    }
    const list = document.getElementById('webhookList');
    if (!list) return;
    list.innerHTML = webhooks.map(h => `
        <div class="data-item">
            <div class="data-item-title">${escapeHTML(h.name)}</div>
            <div class="data-item-meta">${escapeHTML(h.url)} · ${h.enabled ? '启用' : '停用'}</div>
            <div class="item-actions">
                <button onclick="fillWebhook('${h.id}')">编辑</button>
                <button class="primary" onclick="testWebhook('${h.id}')">测试</button>
                <button onclick="deleteWebhook('${h.id}')">删除</button>
            </div>
        </div>
    `).join('') || '<div class="data-item">暂无 Webhook</div>';
}

function fillWebhook(id) {
    const h = webhooks.find(item => item.id === id);
    if (!h) return;
    document.getElementById('webhookName').dataset.id = h.id;
    document.getElementById('webhookName').value = h.name;
    document.getElementById('webhookURL').value = h.url;
    document.getElementById('webhookHeaders').value = h.headers_json || '{}';
    document.getElementById('webhookEvents').value = h.events || '[]';
    document.getElementById('webhookEnabled').checked = !!h.enabled;
}

async function saveWebhook() {
    try {
        const id = document.getElementById('webhookName').dataset.id || '';
        JSON.parse(document.getElementById('webhookHeaders').value || '{}');
        JSON.parse(document.getElementById('webhookEvents').value || '[]');
        await apiFetch(id ? `/api/webhooks/${id}` : '/api/webhooks', {
            method: id ? 'PUT' : 'POST',
            body: JSON.stringify({
                id,
                name: document.getElementById('webhookName').value,
                url: document.getElementById('webhookURL').value,
                method: 'POST',
                headers_json: document.getElementById('webhookHeaders').value || '{}',
                events: document.getElementById('webhookEvents').value || '[]',
                enabled: document.getElementById('webhookEnabled').checked
            })
        });
        document.getElementById('webhookName').dataset.id = '';
        await loadWebhooks();
        alert('Webhook 已保存');
    } catch (error) {
        alert(error.message);
    }
}

async function testWebhook(id) {
    try {
        const data = await apiFetch(`/api/webhooks/${id}/test`, { method: 'POST' });
        alert(data.join('\n') || '已发送');
    } catch (error) {
        alert(error.message);
    }
}

async function deleteWebhook(id) {
    if (!confirm('确认删除这个 Webhook？')) return;
    await apiFetch(`/api/webhooks/${id}`, { method: 'DELETE' });
    await loadWebhooks();
}

let dataCenterLoaded = false;

async function loadDataCenter() {
    if (dataCenterLoaded) return;
    dataCenterLoaded = true;
    await Promise.allSettled([
        loadMarketOverview(),
        loadStockProfile(),
        loadHistoryData('tdx'),
        loadBlockData()
    ]);
}

async function loadMarketOverview() {
    setLoadingText('marketOverviewStats');
    try {
        const [status, stats, count] = await Promise.all([
            apiFetch('/api/server-status'),
            apiFetch('/api/market-stats'),
            apiFetch('/api/market-count')
        ]);
        renderMetricCards('marketOverviewStats', [
            { label: '服务状态', value: status.status || '--', note: status.connected ? '通达信已连接' : '连接异常' },
            { label: '市场证券数', value: count.total || 0, note: (count.exchanges || []).map(v => `${v.exchange}:${v.count}`).join(' · ') },
            { label: '沪市股票', value: stats.sh?.total || 0, note: `涨 ${stats.sh?.up || 0} · 跌 ${stats.sh?.down || 0} · 平 ${stats.sh?.flat || 0}` },
            { label: '深市股票', value: stats.sz?.total || 0, note: `涨 ${stats.sz?.up || 0} · 跌 ${stats.sz?.down || 0} · 平 ${stats.sz?.flat || 0}` },
            { label: '北交所', value: stats.bj?.total || 0, note: `涨 ${stats.bj?.up || 0} · 跌 ${stats.bj?.down || 0} · 平 ${stats.bj?.flat || 0}` }
        ]);
        await loadCodeDirectory();
    } catch (error) {
        setErrorText('marketOverviewStats', error);
    }
}

async function loadCodeDirectory() {
    setLoadingText('marketDirectoryOutput');
    try {
        const exchange = document.getElementById('marketCodeExchange')?.value || 'all';
        const limit = document.getElementById('marketCodeLimit')?.value || '80';
        const data = await apiFetch(`/api/codes?exchange=${encodeURIComponent(exchange)}`);
        const rows = (data.codes || []).slice(0, Number(limit) || 80);
        renderTable('marketDirectoryOutput', rows, [
            { key: 'code', label: '代码' },
            { key: 'name', label: '名称' },
            { key: 'exchange', label: '市场' }
        ]);
    } catch (error) {
        setErrorText('marketDirectoryOutput', error);
    }
}

async function loadETFDirectory() {
    setLoadingText('marketDirectoryOutput');
    try {
        const exchange = document.getElementById('marketCodeExchange')?.value || 'all';
        const limit = document.getElementById('marketCodeLimit')?.value || '80';
        const data = await apiFetch(`/api/etf?exchange=${encodeURIComponent(exchange)}&limit=${encodeURIComponent(limit)}`);
        renderTable('marketDirectoryOutput', data.list || [], [
            { key: 'code', label: '代码' },
            { key: 'name', label: '名称' },
            { key: 'exchange', label: '市场' },
            { key: 'last_price', label: '最新价' }
        ]);
    } catch (error) {
        setErrorText('marketDirectoryOutput', error);
    }
}

function syncDataStockInputs() {
    const code = document.getElementById('dataStockCode')?.value || currentStock || '000001';
    const historyCode = document.getElementById('historyCode');
    if (historyCode && !historyCode.value.trim()) historyCode.value = code;
    return code.trim();
}

async function loadStockProfile() {
    setLoadingText('stockProfileOutput');
    try {
        const code = syncDataStockInputs();
        const [finance, categories, gbbq, auction] = await Promise.allSettled([
            apiFetch(`/api/finance?code=${encodeURIComponent(code)}`),
            apiFetch(`/api/company/categories?code=${encodeURIComponent(code)}`),
            apiFetch(`/api/gbbq?code=${encodeURIComponent(code)}`),
            apiFetch(`/api/call-auction?code=${encodeURIComponent(code)}`)
        ]);
        const categoryList = categories.status === 'fulfilled' ? categories.value || [] : [];
        const panels = [
            finance.status === 'fulfilled' ? renderKeyValuePanel('财务信息', finance.value) : renderJsonPanel('财务信息', finance.reason?.message || '加载失败'),
            renderJsonPanel('F10目录', categoryList.slice(0, 20)),
            renderJsonPanel('股本变迁', (gbbq.status === 'fulfilled' ? gbbq.value || [] : []).slice?.(0, 20) || gbbq.value || []),
            renderJsonPanel('集合竞价', auction.status === 'fulfilled' ? auction.value : auction.reason?.message || '加载失败')
        ];
        document.getElementById('stockProfileOutput').innerHTML = panels.join('');
    } catch (error) {
        setErrorText('stockProfileOutput', error);
    }
}

async function loadIncomeReport() {
    const output = document.getElementById('stockProfileOutput');
    if (!output) return;
    setLoadingText('stockProfileOutput');
    try {
        const code = syncDataStockInputs();
        const startDate = document.getElementById('incomeStartDate')?.value || '';
        const days = document.getElementById('incomeDays')?.value || '';
        const data = await apiFetch(`/api/income?code=${encodeURIComponent(code)}&start_date=${encodeURIComponent(startDate)}&days=${encodeURIComponent(days)}`);
        output.innerHTML = renderJsonPanel('收益测算', data);
    } catch (error) {
        setErrorText('stockProfileOutput', error);
    }
}

async function loadHistoryData(mode) {
    setLoadingText('historyOutput');
    try {
        const code = document.getElementById('historyCode')?.value || document.getElementById('dataStockCode')?.value || '000001';
        const type = document.getElementById('historyType')?.value || 'day';
        const limit = document.getElementById('historyLimit')?.value || '120';
        const date = document.getElementById('historyDate')?.value || '';
        const startDate = document.getElementById('historyStartDate')?.value || '';
        const endDate = document.getElementById('historyEndDate')?.value || '';
        let data;
        if (mode === 'ths') {
            data = await apiFetch(`/api/kline-all/ths?code=${encodeURIComponent(code)}&type=${encodeURIComponent(type)}&limit=${encodeURIComponent(limit)}`);
            renderKlineRows(data);
        } else if (mode === 'history') {
            data = await apiFetch(`/api/kline-history?code=${encodeURIComponent(code)}&type=${encodeURIComponent(type)}&limit=${encodeURIComponent(limit)}`);
            renderKlineRows(data);
        } else if (mode === 'trade') {
            data = await apiFetch(`/api/trade-history/full?code=${encodeURIComponent(code)}&start_date=${encodeURIComponent(startDate)}&end_date=${encodeURIComponent(endDate)}&limit=${encodeURIComponent(limit)}`);
            renderTradeRows(data.list || []);
        } else if (mode === 'minuteTrade') {
            data = await apiFetch(`/api/minute-trade-all?code=${encodeURIComponent(code)}&date=${encodeURIComponent(date)}`);
            renderTradeRows(data.List || data.list || []);
        } else {
            data = await apiFetch(`/api/kline-all/tdx?code=${encodeURIComponent(code)}&type=${encodeURIComponent(type)}&limit=${encodeURIComponent(limit)}`);
            renderKlineRows(data);
        }
    } catch (error) {
        setErrorText('historyOutput', error);
    }
}

function renderKlineRows(data) {
    const rows = data.list || data.List || [];
    renderTable('historyOutput', rows, [
        { key: 'time', label: '时间', value: row => row.time || row.Time },
        { key: 'open', label: '开盘', value: row => row.open ?? row.Open },
        { key: 'high', label: '最高', value: row => row.high ?? row.High },
        { key: 'low', label: '最低', value: row => row.low ?? row.Low },
        { key: 'close', label: '收盘', value: row => row.close ?? row.Close },
        { key: 'volume', label: '成交量', value: row => row.volume ?? row.Volume },
        { key: 'amount', label: '成交额', value: row => row.amount ?? row.Amount }
    ]);
}

function renderTradeRows(rows) {
    renderTable('historyOutput', rows, [
        { key: 'time', label: '时间', value: row => row.time || row.Time },
        { key: 'price', label: '价格', value: row => row.price ?? row.Price },
        { key: 'volume', label: '成交量', value: row => row.volume ?? row.Volume },
        { key: 'number', label: '笔数', value: row => row.number ?? row.Number },
        { key: 'status', label: '性质', value: row => row.status ?? row.Status }
    ]);
}

async function loadWorkdayRange() {
    setLoadingText('historyOutput');
    try {
        const startDate = document.getElementById('historyStartDate')?.value || '';
        const endDate = document.getElementById('historyEndDate')?.value || '';
        const data = await apiFetch(`/api/workday/range?start=${encodeURIComponent(startDate)}&end=${encodeURIComponent(endDate)}`);
        renderTable('historyOutput', data.list || [], [
            { key: 'iso', label: '日期' },
            { key: 'numeric', label: '数字日期' }
        ]);
    } catch (error) {
        setErrorText('historyOutput', error);
    }
}

async function loadBlockData() {
    setLoadingText('blockIndustryOutput');
    try {
        const file = document.getElementById('blockFile')?.value || 'gn';
        const withIndex = document.getElementById('blockWithIndex')?.checked ? 'true' : 'false';
        const data = await apiFetch(`/api/block?file=${encodeURIComponent(file)}&with_index=${withIndex}`);
        renderTable('blockIndustryOutput', data.list || [], [
            { key: 'Name', label: '板块' },
            { key: 'Index', label: '指数代码' },
            { key: 'Type', label: '类型' },
            { key: 'Codes', label: '成分', value: row => row.Codes || row.codes || [] }
        ]);
    } catch (error) {
        setErrorText('blockIndustryOutput', error);
    }
}

async function loadIndustryData(kind) {
    setLoadingText('blockIndustryOutput');
    try {
        const endpoints = {
            hy: '/api/tdx-hy',
            stat: '/api/tdx-stat',
            stat2: '/api/tdx-stat2',
            xgsg: '/api/xgsg'
        };
        const data = await apiFetch(endpoints[kind] || endpoints.hy);
        const rows = data.list || data.List || [];
        if (kind === 'hy') {
            renderTable('blockIndustryOutput', rows, [
                { key: 'Code', label: '代码' },
                { key: 'TdxHy', label: '通达信行业' },
                { key: 'SwHy', label: '申万行业' }
            ]);
        } else {
            const sample = rows[0] || {};
            const columns = Object.keys(sample).slice(0, 8).map(key => ({ key, label: key }));
            renderTable('blockIndustryOutput', rows, columns.length ? columns : [{ key: 'value', label: '数据', value: row => row }]);
        }
    } catch (error) {
        setErrorText('blockIndustryOutput', error);
    }
}

async function loadHQChart() {
    try {
        const symbol = document.getElementById('hqSymbol').value || '000001';
        const period = document.getElementById('hqPeriod').value;
        currentHQOverlay = null;
        updateHQOverlayStatus('未叠加公式');
        renderHQKLine('hqChart', symbol, period, { count: 800, pageSize: 80 });
        refreshHQFormulaWindowOptions();
    } catch (error) {
        alert(error.message || error);
    }
}

function updateHQOverlayStatus(text) {
    const node = document.getElementById('hqOverlayStatus');
    if (node) node.textContent = text;
}

window.updateHQOverlayStatus = updateHQOverlayStatus;

function formulaResultForSymbol(resp, symbol) {
    const data = resp?.data || {};
    const normalized = normalizeSymbol(symbol);
    return data[normalized] || data[normalized.toUpperCase()] || Object.values(data)[0] || {};
}

function getHQChartInstance() {
    const container = document.getElementById('hqChart');
    if (!window.TDXHQChart || !container) return null;
    return window.TDXHQChart.getChart ? window.TDXHQChart.getChart(container) : null;
}

function getHQChartFrameCount(chart) {
    return chart?.JSChartContainer?.Frame?.SubFrame?.length
        || chart?.Frame?.SubFrame?.length
        || 2;
}

function refreshHQFormulaWindowOptions() {
    const select = document.getElementById('hqFormulaWindow');
    if (!select) return;
    const currentValue = select.value || '0';
    const chart = getHQChartInstance();
    const frameCount = getHQChartFrameCount(chart);
    const count = Math.max(2, frameCount);
    select.innerHTML = Array.from({ length: count }, (_, index) => {
        const label = index === 0 ? '主图' : `副图${index}`;
        return `<option value="${index}">${label}</option>`;
    }).join('');
    select.value = Number(currentValue) < count ? currentValue : '0';
}

function buildHQFormulaIndexInfo(formula) {
    if (!formula?.script) {
        throw new Error('公式脚本为空，无法应用');
    }
    return {
        Name: formula.name || '自定义公式',
        Script: formula.script,
        Args: parseHQFormulaArgs(formula),
        YAxis: {
            ExcludeValue: !!document.getElementById('hqFormulaExcludeY')?.checked
        }
    };
}

function currentHQSymbolPeriod() {
    const symbol = document.getElementById('hqSymbol').value || '000001';
    const period = document.getElementById('hqPeriod').value;
    return { symbol, period };
}

async function applySelectedFormulaToHQChart() {
    const formula = selectedHQFormula();
    if (!formula) {
        alert('请先选择一个公式');
        return;
    }
    try {
        updateHQOverlayStatus('正在应用 HQChart 公式...');
        const { symbol, period } = currentHQSymbolPeriod();
        let chart = getHQChartInstance();
        if (!chart) {
            renderHQKLine('hqChart', symbol, period, { count: 800, pageSize: 80 });
            refreshHQFormulaWindowOptions();
            chart = getHQChartInstance();
        }
        if (!chart) throw new Error('HQChart 图表实例未就绪');

        const mode = document.getElementById('hqFormulaApplyMode')?.value || 'overlay';
        const windowIndex = Number(document.getElementById('hqFormulaWindow')?.value || 0);
        const indexInfo = buildHQFormulaIndexInfo(formula);
        const args = indexInfo.Args || [];
        let appliedWindowIndex = windowIndex;

        if (mode === 'change') {
            if (typeof chart.ChangeScriptIndex !== 'function') throw new Error('当前 HQChart 版本不支持 ChangeScriptIndex');
            chart.ChangeScriptIndex(windowIndex, indexInfo);
        } else if (mode === 'new-window') {
            if (typeof chart.AddScriptIndexWindow !== 'function') throw new Error('当前 HQChart 版本不支持 AddScriptIndexWindow');
            chart.AddScriptIndexWindow(indexInfo, { Draw: true });
            refreshHQFormulaWindowOptions();
            appliedWindowIndex = Math.max(1, getHQChartFrameCount(chart) - 1);
        } else {
            if (typeof chart.AddOverlayIndex !== 'function') throw new Error('当前 HQChart 版本不支持 AddOverlayIndex');
            const independentY = !!document.getElementById('hqFormulaIndependentY')?.checked;
            const option = {
                Script: indexInfo.Script,
                WindowIndex: windowIndex,
                Name: indexInfo.Name,
                Args: args,
                IsShareY: !independentY
            };
            if (option.IsShareY) option.YAxis = indexInfo.YAxis;
            chart.AddOverlayIndex(option);
        }

        const resp = await apiFetch(`/api/formulas/${formula.id}/test`, {
            method: 'POST',
            body: JSON.stringify({
                symbol,
                period,
                calc_count: 500,
                out_count: 120
            })
        });
        const result = formulaResultForSymbol(resp, symbol);
        currentHQOverlay = {
            formulaID: formula.id,
            name: formula?.name || '公式',
            mode,
            windowIndex: appliedWindowIndex,
            engine: resp.engine || '',
            tickMS: resp.tick_ms || 0
        };
        document.getElementById('formulaTestOutput').textContent = prettyJSON(resp);
        const modeLabel = mode === 'change' ? '切换窗口' : (mode === 'new-window' ? '新建副图' : '叠加指标');
        const windowLabel = appliedWindowIndex === 0 ? '主图' : `副图${appliedWindowIndex}`;
        updateHQOverlayStatus(`已用 HQChart ${modeLabel}：${currentHQOverlay.name} · ${windowLabel} · ${result.hit ? '命中' : '未命中'} · ${currentHQOverlay.engine || 'engine'} ${currentHQOverlay.tickMS}ms`);
    } catch (error) {
        updateHQOverlayStatus(`应用失败：${error.message || error}`);
        alert(error.message || error);
    }
}

async function overlaySelectedFormulaOnHQChart() {
    await applySelectedFormulaToHQChart();
}

function clearHQFormulaOverlay() {
    currentHQOverlay = null;
    const { symbol, period } = currentHQSymbolPeriod();
    renderHQKLine('hqChart', symbol, period, { count: 800, pageSize: 80 });
    updateHQOverlayStatus('未叠加公式');
}

document.addEventListener('DOMContentLoaded', async () => {
    try {
        await refreshSystemStatus();
        setInterval(refreshSystemStatus, 30000);
        if (!currentStock) {
            loadQuickStock('000001');
        }
        await Promise.all([loadFormulaList(), loadPools(), loadWebhooks()]);
        await loadAutomations();
        await loadRuns();
        await loadSelectionResults();
        await refreshDecisionDesk();
        await loadHQChart();
    } catch (error) {
        console.warn('初始化自动化页面失败:', error);
    }
});
