(function () {
    const charts = new Map();
    const states = new Map();
    let nextID = 1;

    function hasHQChart() {
        return typeof window.JSChart !== 'undefined' && window.JSChart && typeof window.JSChart.Init === 'function';
    }

    function normalizeCode(symbol) {
        const value = String(symbol || '').trim().toLowerCase();
        if (!value) return '000001';
        return value.split('.')[0];
    }

    function toHQSymbol(symbol) {
        const raw = String(symbol || '').trim().toLowerCase();
        if (/^\d{6}\.(sh|sz|bj)$/.test(raw)) return raw;
        const code = normalizeCode(symbol);
        if (code.startsWith('6') || code.startsWith('9')) return `${code}.sh`;
        if (code.startsWith('8') || code.startsWith('4')) return `${code}.bj`;
        return `${code}.sz`;
    }

    function periodToHQChart(period) {
        switch (String(period || '').toLowerCase()) {
            case 'week':
                return 1;
            case 'month':
                return 2;
            case 'minute1':
            case '1m':
                return 4;
            case 'minute5':
            case '5m':
                return 5;
            case 'minute15':
            case '15m':
                return 6;
            case 'minute30':
            case '30m':
                return 7;
            case 'hour':
            case '60m':
                return 8;
            default:
                return 0;
        }
    }

    function chartKey(container) {
        if (!container) return '';
        if (!container.dataset.hqChartKey) {
            container.dataset.hqChartKey = container.id || `hq-chart-${nextID++}`;
        }
        return container.dataset.hqChartKey;
    }

    async function fetchHistory(symbol, period, count) {
        const querySymbol = String(symbol || '').trim() || normalizeCode(symbol);
        const url = `/api/hqchart/history?symbol=${encodeURIComponent(querySymbol)}&period=${encodeURIComponent(period || 'day')}&limit=${encodeURIComponent(count || 800)}`;
        const response = await fetch(url);
        const result = await response.json();
        if (result.code !== 0) {
            throw new Error(result.message || 'HQChart 数据请求失败');
        }
        return result.data;
    }

    async function fetchIndexHistory(symbol, period, count) {
        const url = `/api/hqchart/history?symbol=${encodeURIComponent(symbol)}&period=${encodeURIComponent(period || 'day')}&limit=${encodeURIComponent(count || 800)}&index=1`;
        const response = await fetch(url);
        const result = await response.json();
        if (result.code !== 0) {
            throw new Error(result.message || 'HQChart 大盘数据请求失败');
        }
        return result.data;
    }

    async function fetchIndexHistoryWithFallback(symbol, period, count) {
        try {
            return await fetchIndexHistory(symbol, period, count);
        } catch (error) {
            if (String(symbol || '').toLowerCase() !== '399001.sz') {
                console.warn('HQChart primary index failed, retry with 399001.sz:', error);
                return fetchIndexHistory('399001.sz', period, count);
            }
            throw error;
        }
    }

    function defaultIndexSymbol(symbol) {
        const hqSymbol = toHQSymbol(symbol).toLowerCase();
        if (hqSymbol.endsWith('.sh')) return '000001.sh';
        return '399001.sz';
    }

    function latestStockFromHistory(payload, symbol) {
        const rows = payload && Array.isArray(payload.data) ? payload.data : [];
        const latest = rows[rows.length - 1] || [];
        return {
            name: payload?.name || symbol,
            symbol: payload?.symbol || symbol,
            yclose: latest[1] || latest[5] || 0,
            open: latest[2] || 0,
            price: latest[5] || 0,
            high: latest[3] || 0,
            low: latest[4] || 0,
            vol: latest[6] || 0,
            amount: latest[7] || 0,
            date: latest[0] || 0,
            time: latest[8] || 150000,
            increase: latest[1] ? ((latest[5] - latest[1]) * 100 / latest[1]) : 0,
            amplitude: latest[1] ? ((latest[3] - latest[4]) * 100 / latest[1]) : 0
        };
    }

    function createNetworkFilter(key) {
        return function networkFilter(data, callback) {
            const name = data && data.Name;
            const state = states.get(key) || {};
            if (name === 'KLineChartContainer::RequestFlowCapitalData') {
                data.PreventDefault = true;
                const req = data.Request && data.Request.Data ? data.Request.Data : {};
                const symbol = req.symbol || state.symbol;
                callback({ symbol: toHQSymbol(symbol), name: toHQSymbol(symbol), stock: [] });
                return;
            }
            if (name === 'ScriptIndex::RequestAuthorization') {
                data.PreventDefault = true;
                const req = data.Request && data.Request.Data ? data.Request.Data : {};
                callback({
                    code: 0,
                    indexName: req.IndexName || '',
                    indexID: req.IndexID || '',
                    Lock: { IsLocked: false }
                });
                return;
            }
            if (name === 'JSSymbolData::GetIndexData') {
                data.PreventDefault = true;
                const req = data.Request && data.Request.Data ? data.Request.Data : {};
                const sourceSymbol = req.symbol || state.symbol;
                const indexSymbol = sourceSymbol ? defaultIndexSymbol(sourceSymbol) : (req.indexSymbol || '399001.sz');
                const count = req.count || state.count || 800;
                fetchIndexHistoryWithFallback(indexSymbol, state.period, count)
                    .then(payload => callback({
                        symbol: payload.symbol || toHQSymbol(indexSymbol),
                        name: payload.name || payload.symbol || toHQSymbol(indexSymbol),
                        data: payload.data || [],
                        ver: payload.ver || 2
                    }))
                    .catch(error => {
                        console.warn('HQChart index history failed:', error);
                        fetchHistory(state.symbol, state.period, count)
                            .then(payload => callback({
                                symbol: payload.symbol || toHQSymbol(state.symbol),
                                name: payload.name || payload.symbol || toHQSymbol(state.symbol),
                                data: payload.data || [],
                                ver: payload.ver || 2
                            }))
                            .catch(() => callback({ symbol: toHQSymbol(indexSymbol), name: toHQSymbol(indexSymbol), data: [], ver: 2 }));
                    });
                return;
            }
            if (name === 'JSSymbolData::GetLatestIndexData') {
                data.PreventDefault = true;
                const req = data.Request && data.Request.Data ? data.Request.Data : {};
                const indexSymbol = defaultIndexSymbol(state.symbol);
                fetchIndexHistoryWithFallback(indexSymbol, state.period, 2)
                    .then(payload => callback({ stock: [latestStockFromHistory(payload, toHQSymbol(indexSymbol))] }))
                    .catch(error => {
                        console.warn('HQChart latest index failed:', error);
                        callback({ stock: [] });
                    });
                return;
            }
            if (name !== 'KLineChartContainer::RequestHistoryData') return;

            data.PreventDefault = true;
            const req = data.Request && data.Request.Data ? data.Request.Data : {};
            const symbol = req.symbol || state.symbol;
            const count = req.count || state.count || 800;
            fetchHistory(symbol, state.period, count)
                .then(payload => callback({
                    symbol: payload.symbol || toHQSymbol(symbol),
                    name: payload.name || payload.symbol || toHQSymbol(symbol),
                    data: payload.data || [],
                    ver: payload.ver || 2
                }))
                .catch(error => {
                    console.warn('HQChart history failed:', error);
                    callback({ symbol: toHQSymbol(symbol), name: toHQSymbol(symbol), data: [], ver: 2 });
                });
            if (name) {
                console.debug('[HQChart NetworkFilter handled]', name);
            }
        };
    }

    function destroyChart(container) {
        const key = chartKey(container);
        const chart = charts.get(key);
        if (chart && typeof chart.ChartDestroy === 'function') {
            chart.ChartDestroy();
        } else if (container && container.JSChart && typeof container.JSChart.ChartDestroy === 'function') {
            container.JSChart.ChartDestroy();
        }
        charts.delete(key);
        states.delete(key);
        if (container) container.innerHTML = '';
    }

    function getChart(container) {
        if (!container) return null;
        return charts.get(chartKey(container)) || null;
    }

    function renderKLine(container, options = {}) {
        if (!container || !hasHQChart()) return false;

        const key = chartKey(container);
        const symbol = normalizeCode(options.symbol);
        const period = options.period || 'day';
        const count = options.count || 800;
        const windows = options.windows || [
            { Index: 'MA' },
            { Index: 'VOL' },
            { Index: 'MACD' }
        ];
        states.set(key, { symbol, period, count });
        destroyChart(container);
        states.set(key, { symbol, period, count });

        if (window.MARKET_SUFFIX_NAME && typeof window.MARKET_SUFFIX_NAME.GetMarketStatus === 'function') {
            window.MARKET_SUFFIX_NAME.GetMarketStatus = function () { return 2; };
        }

        const chart = window.JSChart.Init(container, false, true);
        charts.set(key, chart);
        chart.SetOption({
            Type: '历史K线图',
            Symbol: toHQSymbol(symbol),
            IsAutoUpdate: false,
            NetworkFilter: createNetworkFilter(key),
            Windows: windows,
            KLine: {
                Right: 1,
                Period: periodToHQChart(period),
                MaxRequestDataCount: count,
                PageSize: options.pageSize || 80,
                IsShowTooltip: true,
                DrawType: 0,
                RightSpaceCount: 2,
                DataWidth: options.dataWidth || 10
            },
            KLineTitle: {
                IsShowName: true,
                IsShowSettingInfo: true,
                IsTitleShowLatestData: true
            },
            Border: {
                Left: 54,
                Right: 76,
                Top: 24,
                Bottom: 24,
                AutoLeft: { Blank: 10, MinWidth: 54 },
                AutoRight: { Blank: 8, MinWidth: 60 }
            },
            CorssCursorInfo: {
                Right: 2,
                DateFormatType: 3,
                IsShowCorss: true
            },
            EnableYDrag: { Right: true, Left: false },
            EnableZoomIndexWindow: true,
            FloatTooltip: { Enable: true }
        });
        chart.ScriptErrorCallback = function (error) {
            const detail = typeof error === 'string'
                ? error
                : (error && (error.Description || error.message || JSON.stringify(error))) || '未知错误';
            if (typeof window.updateHQOverlayStatus === 'function') {
                window.updateHQOverlayStatus(`HQChart 公式错误：${detail}`);
            }
            console.warn('HQChart script error:', error);
        };
        return true;
    }

    function resize(container) {
        if (container) {
            const chart = charts.get(chartKey(container));
            if (chart && typeof chart.OnSize === 'function') chart.OnSize();
            return;
        }
        charts.forEach(chart => {
            if (chart && typeof chart.OnSize === 'function') chart.OnSize();
        });
    }

    window.TDXHQChart = { renderKLine, resize, destroy: destroyChart, getChart, isAvailable: hasHQChart };
})();
