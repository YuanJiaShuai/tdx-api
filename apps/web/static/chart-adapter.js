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
            case 'minute60':
            case 'hour':
            case '60m':
                return 8;
            default:
                return 0;
        }
    }

    function isKLineHistoryRequest(name) {
        return [
            'KLineChartContainer::RequestHistoryData',
            'KLineChartContainer::ReqeustHistoryMinuteData',
            'KLineChartContainer::RequestHistoryMinuteData',
            'KLineChartContainer::RequestHistoryPageData',
            'KLineChartContainer::RequestDragDayData',
            'KLineChartContainer::RequestDragMinuteData',
            'KLineChartContainer::RequestZoomDayData',
            'KLineChartContainer::RequestZoomMinuteData'
        ].includes(name);
    }

    function isMinuteRequest(name) {
        return [
            'MinuteChartContainer::RequestMinuteData',
            'MinuteChartContainer::RequestHistoryMinuteData'
        ].includes(name);
    }

    function resolveHistoryLimit(req, state) {
        const requested = Number(req && req.count) || 0;
        const configured = Number(state && state.count) || 0;
        const period = String(state && state.period || '').toLowerCase();
        if (period.startsWith('minute') || period === 'hour') {
            return Math.max(requested, configured, 240);
        }
        return requested || configured || 800;
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

    async function fetchMinute(symbol) {
        const code = normalizeCode(symbol);
        const response = await fetch(`/api/minute?code=${encodeURIComponent(code)}`);
        const result = await response.json();
        if (result.code !== 0) {
            throw new Error(result.message || '分时数据请求失败');
        }
        return result.data || {};
    }

    async function fetchQuote(symbol) {
        const code = normalizeCode(symbol);
        const response = await fetch(`/api/quote?code=${encodeURIComponent(code)}`);
        const result = await response.json();
        if (result.code !== 0) {
            throw new Error(result.message || '行情数据请求失败');
        }
        return Array.isArray(result.data) ? result.data[0] : null;
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

    function scaledPrice(value) {
        const number = Number(value || 0);
        if (!Number.isFinite(number) || number <= 0) return 0;
        return number > 1000 ? number / 1000 : number;
    }

    function minuteTimeToNumber(value) {
        const text = String(value || '').trim();
        const match = text.match(/^(\d{1,2}):(\d{2})(?::(\d{2}))?$/);
        if (match) {
            return Number(match[1]) * 10000 + Number(match[2]) * 100 + Number(match[3] || 0);
        }
        const number = Number(text.replace(/\D/g, ''));
        if (!Number.isFinite(number)) return 0;
        return number < 10000 ? number * 100 : number;
    }

    function minuteDateToNumber(value) {
        const number = Number(String(value || '').replace(/\D/g, ''));
        if (Number.isFinite(number) && number > 0) return number;
        const now = new Date();
        return Number(`${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}`);
    }

    function buildMinutePayload(symbol, minutePayload, quote) {
        const hqSymbol = toHQSymbol(symbol);
        const rows = Array.isArray(minutePayload?.List) ? minutePayload.List : [];
        const date = minuteDateToNumber(minutePayload?.date);
        const quoteK = quote?.K || {};
        const yclose = scaledPrice(quoteK.Last) || scaledPrice(rows[0]?.Price);
        let totalVol = 0;
        let totalAmount = 0;
        let high = scaledPrice(quoteK.High) || 0;
        let low = scaledPrice(quoteK.Low) || 0;
        let amountForAverage = 0;
        let volumeForAverage = 0;

        const minute = rows.map(item => {
            const price = scaledPrice(item?.Price);
            const vol = Number(item?.Number || 0);
            const amount = price > 0 && vol > 0 ? price * vol : 0;
            totalVol += Number.isFinite(vol) ? vol : 0;
            totalAmount += amount;
            if (price > 0) {
                high = high > 0 ? Math.max(high, price) : price;
                low = low > 0 ? Math.min(low, price) : price;
                amountForAverage += amount;
                volumeForAverage += vol;
            }
            const average = volumeForAverage > 0 ? amountForAverage / volumeForAverage : price;
            return {
                date,
                time: minuteTimeToNumber(item?.Time),
                open: price,
                high: price,
                low: price,
                price,
                vol,
                amount,
                avprice: average || price
            };
        }).filter(item => item.time > 0 && item.price > 0);

        const first = minute[0] || {};
        const last = minute[minute.length - 1] || {};
        const open = scaledPrice(quoteK.Open) || first.price || 0;
        const close = scaledPrice(quoteK.Close) || last.price || 0;
        const name = quote?.Name || quote?.name || hqSymbol;
        return {
            stock: [{
                name,
                symbol: hqSymbol,
                yclose: yclose || open || close || 0,
                open,
                price: close,
                high: high || close || open || 0,
                low: low || close || open || 0,
                vol: Number(quote?.TotalHand || 0) * 100 || totalVol,
                amount: Number(quote?.Amount || 0) || totalAmount,
                date,
                time: last.time || 150000,
                minute,
                minutecount: minute.length || Number(minutePayload?.Count || 0) || 240
            }],
            AutoUpdate: false
        };
    }

    async function fetchMinuteChartData(symbol) {
        const [minuteResult, quoteResult] = await Promise.allSettled([
            fetchMinute(symbol),
            fetchQuote(symbol)
        ]);
        if (minuteResult.status === 'rejected') throw minuteResult.reason;
        return buildMinutePayload(symbol, minuteResult.value, quoteResult.status === 'fulfilled' ? quoteResult.value : null);
    }

    function createNetworkFilter(key) {
        return function networkFilter(data, callback) {
            const name = data && data.Name;
            const state = states.get(key) || {};
            if (isMinuteRequest(name)) {
                data.PreventDefault = true;
                const req = data.Request && data.Request.Data ? data.Request.Data : {};
                const reqSymbol = Array.isArray(req.symbol) ? req.symbol[0] : req.symbol;
                const symbol = reqSymbol || state.symbol;
                fetchMinuteChartData(symbol)
                    .then(payload => callback(payload))
                    .catch(error => {
                        console.warn('HQChart minute failed:', error);
                        callback(buildMinutePayload(symbol, { Count: 0, List: [] }, null));
                    });
                if (name) {
                    console.debug('[HQChart NetworkFilter handled]', name);
                }
                return;
            }
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
            if (!isKLineHistoryRequest(name)) return;

            data.PreventDefault = true;
            const req = data.Request && data.Request.Data ? data.Request.Data : {};
            const symbol = req.symbol || state.symbol;
            const count = resolveHistoryLimit(req, state);
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

    function renderMinute(container, options = {}) {
        if (!container || !hasHQChart()) return false;

        const key = chartKey(container);
        const symbol = normalizeCode(options.symbol);
        states.set(key, { symbol, period: 'minute', count: 240 });
        destroyChart(container);
        states.set(key, { symbol, period: 'minute', count: 240 });

        if (window.MARKET_SUFFIX_NAME && typeof window.MARKET_SUFFIX_NAME.GetMarketStatus === 'function') {
            window.MARKET_SUFFIX_NAME.GetMarketStatus = function () { return 2; };
        }

        const chart = window.JSChart.Init(container, false, true);
        charts.set(key, chart);
        chart.SetOption({
            Type: '分钟走势图',
            Symbol: toHQSymbol(symbol),
            IsAutoUpdate: false,
            DayCount: 1,
            NetworkFilter: createNetworkFilter(key),
            Windows: options.windows || [],
            Minute: {
                IsShowTooltip: true
            },
            MinuteLine: {
                IsDrawAreaPrice: options.isDrawAreaPrice === false ? 0 : 1,
                IsShowAveragePrice: options.isShowAveragePrice === false ? 0 : 1
            },
            MinuteTitle: {
                IsShowName: true,
                IsShowDate: false,
                IsShowTime: true,
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

    window.TDXHQChart = { renderKLine, renderMinute, resize, destroy: destroyChart, getChart, isAvailable: hasHQChart };
})();
