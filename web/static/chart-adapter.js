(function () {
    let chart = null;
    let current = { symbol: '000001', period: 'day' };

    function hasHQChart() {
        return typeof window.JSChart !== 'undefined' && window.JSChart && typeof window.JSChart.Init === 'function';
    }

    function normalizeCode(symbol) {
        const value = String(symbol || '').trim().toLowerCase();
        if (!value) return '000001';
        return value.split('.')[0];
    }

    function toHQSymbol(symbol) {
        const code = normalizeCode(symbol);
        if (code.startsWith('6') || code.startsWith('9')) return `${code}.sh`;
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

    async function fetchHistory(symbol, period, count) {
        const url = `/api/hqchart/history?symbol=${encodeURIComponent(normalizeCode(symbol))}&period=${encodeURIComponent(period || 'day')}&limit=${encodeURIComponent(count || 800)}`;
        const response = await fetch(url);
        const result = await response.json();
        if (result.code !== 0) {
            throw new Error(result.message || 'HQChart 数据请求失败');
        }
        return result.data;
    }

    function networkFilter(data, callback) {
        const name = data && data.Name;
        if (name === 'KLineChartContainer::RequestFlowCapitalData') {
            data.PreventDefault = true;
            const req = data.Request && data.Request.Data ? data.Request.Data : {};
            const symbol = req.symbol || current.symbol;
            callback({ symbol: toHQSymbol(symbol), name: toHQSymbol(symbol), stock: [] });
            return;
        }
        if (name !== 'KLineChartContainer::RequestHistoryData') return;

        data.PreventDefault = true;
        const req = data.Request && data.Request.Data ? data.Request.Data : {};
        const symbol = req.symbol || current.symbol;
        const count = req.count || 800;
        fetchHistory(symbol, current.period, count)
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
    }

    function destroyChart(container) {
        if (chart && typeof chart.ChartDestroy === 'function') {
            chart.ChartDestroy();
        } else if (container && container.JSChart && typeof container.JSChart.ChartDestroy === 'function') {
            container.JSChart.ChartDestroy();
        }
        chart = null;
        if (container) container.innerHTML = '';
    }

    function renderKLine(container, options) {
        if (!hasHQChart()) return false;

        const symbol = normalizeCode(options.symbol);
        const period = options.period || 'day';
        current = { symbol, period };
        destroyChart(container);

        if (window.MARKET_SUFFIX_NAME && typeof window.MARKET_SUFFIX_NAME.GetMarketStatus === 'function') {
            window.MARKET_SUFFIX_NAME.GetMarketStatus = function () { return 2; };
        }

        chart = window.JSChart.Init(container, false, true);
        const chartOption = {
            Type: '历史K线图',
            Symbol: toHQSymbol(symbol),
            IsAutoUpdate: false,
            NetworkFilter: networkFilter,
            Windows: [
                { Index: 'MA' },
                { Index: 'VOL' },
                { Index: 'MACD' }
            ],
            KLine: {
                Right: 1,
                Period: periodToHQChart(period),
                MaxRequestDataCount: 800,
                PageSize: 80,
                IsShowTooltip: true,
                DrawType: 0,
                RightSpaceCount: 2,
                DataWidth: 10
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
        };

        chart.SetOption(chartOption);
        return true;
    }

    function resize() {
        if (chart && typeof chart.OnSize === 'function') chart.OnSize();
    }

    window.TDXHQChart = { renderKLine, resize, destroy: destroyChart, isAvailable: hasHQChart };
})();
