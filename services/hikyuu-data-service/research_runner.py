"""Research helpers shared by the Hikyuu HTTP service.

Hikyuu remains the source of historical bars and, when its Python indicator
symbols are available, the source of indicator values.  The small native
fallback is intentionally explicit: it keeps the API useful while a dataset
is being bootstrapped and reports the fallback in response metadata.
"""

from __future__ import annotations

import math
import os
from datetime import datetime
from typing import Any, Iterable

from query_runner import CONFIG_FILE, PERIODS, RECOVER_TYPES, load_records, normalize_symbol, parse_date, _hikyuu_runtime, _runtime_lock


INDICATORS = {"ma", "ema", "macd", "boll", "atr"}


def _finite(value: Any) -> float | None:
    try:
        result = float(value)
    except (TypeError, ValueError):
        return None
    return result if math.isfinite(result) else None


def _closes(rows: list[dict[str, Any]]) -> list[float]:
    return [float(row.get("close") or 0) for row in rows]


def _ema(values: list[float], period: int) -> list[float | None]:
    if period <= 0:
        raise ValueError("period must be positive")
    result: list[float | None] = [None] * len(values)
    if len(values) < period:
        return result
    current = sum(values[:period]) / period
    result[period - 1] = current
    alpha = 2 / (period + 1)
    for index in range(period, len(values)):
        current = (values[index] - current) * alpha + current
        result[index] = current
    return result


def _sma(values: list[float], period: int) -> list[float | None]:
    if period <= 0:
        raise ValueError("period must be positive")
    result: list[float | None] = [None] * len(values)
    for index in range(period - 1, len(values)):
        result[index] = sum(values[index - period + 1 : index + 1]) / period
    return result


def _boll(values: list[float], period: int, width: float) -> tuple[list[float | None], list[float | None], list[float | None]]:
    middle = _sma(values, period)
    upper: list[float | None] = [None] * len(values)
    lower: list[float | None] = [None] * len(values)
    for index, mid in enumerate(middle):
        if mid is None:
            continue
        window = values[index - period + 1 : index + 1]
        deviation = math.sqrt(sum((value - mid) ** 2 for value in window) / period)
        upper[index] = mid + width * deviation
        lower[index] = mid - width * deviation
    return middle, upper, lower


def _atr(rows: list[dict[str, Any]], period: int) -> list[float | None]:
    if period <= 0:
        raise ValueError("period must be positive")
    true_ranges: list[float] = []
    previous_close = None
    for row in rows:
        high = float(row.get("high") or 0)
        low = float(row.get("low") or 0)
        close = float(row.get("close") or 0)
        true_ranges.append(max(high - low, abs(high - previous_close), abs(low - previous_close)) if previous_close else high - low)
        previous_close = close
    return _sma(true_ranges, period)


def _indicator_from_hikyuu(rows: list[dict[str, Any]], name: str, params: dict[str, Any]) -> tuple[list[dict[str, Any]], str] | None:
    """Try Hikyuu's native indicator functions without making them mandatory.

    Hikyuu has changed the Python wrapper's conversion helpers between minor
    releases.  This adapter probes the supported conversion methods and falls
    back cleanly when a deployment exposes an older wrapper.
    """
    try:
        import hikyuu as hq  # type: ignore
        with _runtime_lock:
            runtime = _hikyuu_runtime()
        Datetime, Query, StockManager = runtime["Datetime"], runtime["Query"], runtime["StockManager"]

        ATR, CLOSE, EMA, MA, MACD = (getattr(hq, name, None) for name in ("ATR", "CLOSE", "EMA", "MA", "MACD"))

        symbol = normalize_symbol(str(params.get("symbol") or ""))
        stock = StockManager.instance()[symbol]
        if stock.is_null():
            return None
        period_name = str(params.get("period_type") or "day").lower()
        ktype = getattr(Query, PERIODS.get(period_name, "DAY"))
        start = parse_date(str(params.get("start") or ""))
        end = parse_date(str(params.get("end") or ""), end=True)
        limit = max(0, int(params.get("limit") or 0))
        if start or end:
            query = Query(Datetime(start or datetime(1990, 1, 1)), Datetime(end) if end else None, ktype=ktype, recover_type=getattr(Query, RECOVER_TYPES.get(str(params.get("recover") or "none"), "NO_RECOVER")))
        else:
            query = Query(-limit if limit else 0, ktype=ktype, recover_type=getattr(Query, RECOVER_TYPES.get(str(params.get("recover") or "none"), "NO_RECOVER")))
        kdata = stock.get_kdata(query)
        close = CLOSE(kdata)
        period = int(params.get("period") or 20)
        raw: Any
        if name == "boll" or any(item is None for item in (ATR, CLOSE, EMA, MA, MACD)):
            return None
        if name == "ma":
            raw = (MA(close, period),)
            fields = ("value",)
        elif name == "ema":
            raw = (EMA(close, period),)
            fields = ("value",)
        elif name == "macd":
            values = MACD(close, int(params.get("fast") or 12), int(params.get("slow") or 26), int(params.get("signal") or 9))
            raw = tuple(values) if isinstance(values, (list, tuple)) else (values,)
            fields = ("dif", "dea", "hist")
        elif name == "boll":
            values = BOLL(close, period, float(params.get("width") or 2))
            raw = tuple(values) if isinstance(values, (list, tuple)) else (values,)
            fields = ("mid", "upper", "lower")
        elif name == "atr":
            raw = (ATR(kdata, period),)
            fields = ("value",)
        else:
            return None

        def values(indicator: Any, result_index: int = 0) -> list[float | None]:
            if result_index and hasattr(indicator, "get_result"):
                indicator = indicator.get_result(result_index)
            if hasattr(indicator, "to_np"):
                array = indicator.to_np()
                return [_finite(value) for value in array]
            if hasattr(indicator, "get_result_as_price_list"):
                array = indicator.get_result_as_price_list(0)
                return [_finite(value) for value in array]
            return [_finite(indicator[index]) for index in range(len(kdata))]

        if name == "macd":
            columns = [values(raw[0], 1), values(raw[0], 2), values(raw[0], 0)]
        else:
            columns = [values(item) for item in raw]
        output = []
        for index, row in enumerate(rows[-len(kdata) :] if len(rows) >= len(kdata) else rows):
            item = {"time": row["time"]}
            for field, column in zip(fields, columns):
                item[field] = column[index] if index < len(column) else None
            output.append(item)
        return output, "hikyuu"
    except Exception:
        return None


def calculate_indicator(payload: dict[str, Any]) -> dict[str, Any]:
    name = str(payload.get("indicator") or payload.get("name") or "").strip().lower()
    if name not in INDICATORS:
        raise ValueError(f"unsupported indicator: {name}")
    symbol = str(payload.get("code") or payload.get("symbol") or "").strip()
    period_type = str(payload.get("type") or payload.get("period_type") or "day").lower()
    limit = max(1, min(100000, int(payload.get("limit") or 800)))
    recover = str(payload.get("recover") or "none").lower()
    bars = load_records(symbol, period_type, str(payload.get("start") or ""), str(payload.get("end") or ""), limit, recover)["list"]
    params = dict(payload.get("params") or {})
    params.update({"symbol": symbol, "period_type": period_type, "start": payload.get("start"), "end": payload.get("end"), "limit": limit, "recover": recover})
    native = _indicator_from_hikyuu(bars, name, params)
    if native is not None:
        values, engine = native
    else:
        closes = _closes(bars)
        period = max(1, int(params.get("period") or 20))
        if name == "ma":
            columns, fields = (_sma(closes, period),), ("value",)
        elif name == "ema":
            columns, fields = (_ema(closes, period),), ("value",)
        elif name == "macd":
            fast, slow, signal = max(1, int(params.get("fast") or 12)), max(2, int(params.get("slow") or 26)), max(1, int(params.get("signal") or 9))
            fast_values, slow_values = _ema(closes, fast), _ema(closes, slow)
            dif = [a - b if a is not None and b is not None else None for a, b in zip(fast_values, slow_values)]
            dea_input = [value or 0 for value in dif]
            dea = _ema(dea_input, signal)
            hist = [((a - b) * 2) if a is not None and b is not None else None for a, b in zip(dif, dea)]
            columns, fields = (dif, dea, hist), ("dif", "dea", "hist")
        elif name == "boll":
            columns, fields = _boll(closes, period, float(params.get("width") or 2)), ("mid", "upper", "lower")
        else:
            columns, fields = (_atr(bars, period),), ("value",)
        values = []
        for index, row in enumerate(bars):
            item = {"time": row["time"]}
            for field, column in zip(fields, columns):
                item[field] = column[index]
            values.append(item)
        engine = "native-fallback"
    return {
        "symbol": symbol,
        "period": period_type,
        "indicator": name,
        "params": params,
        "count": len(values),
        "list": values,
        "meta": {"source": "hikyuu", "calculation_engine": engine, "data_revision": os.getenv("HIKYUU_DATA_REVISION", "runtime")},
    }


def run_reference_backtest(payload: dict[str, Any]) -> dict[str, Any]:
    """Run a reproducible MA cross reference strategy on Hikyuu bars."""
    symbols = [str(value) for value in (payload.get("symbols") or []) if str(value).strip()]
    if not symbols:
        raise ValueError("symbols must not be empty")
    fast = max(1, int(payload.get("fast") or 5))
    slow = max(fast + 1, int(payload.get("slow") or 20))
    initial_cash = max(1.0, float(payload.get("initial_cash") or 100000))
    buy_cost = max(0.0, float(payload.get("buy_cost") or 0.0005))
    sell_cost = max(0.0, float(payload.get("sell_cost") or 0.001))
    history_count = max(slow + 2, min(2000, int(payload.get("history_count") or 520)))
    trades: list[dict[str, Any]] = []
    equity_points: dict[str, float] = {}
    per_symbol: dict[str, dict[str, Any]] = {}
    warnings: list[str] = []
    for symbol in symbols:
        try:
            bars = load_records(symbol, str(payload.get("type") or "day"), str(payload.get("start") or ""), str(payload.get("end") or ""), history_count, str(payload.get("recover") or "none"))["list"]
        except Exception as exc:
            warnings.append(f"{symbol}: {exc}")
            continue
        closes = _closes(bars)
        short, long = _sma(closes, fast), _sma(closes, slow)
        cash, shares, entry, entry_index = initial_cash, 0.0, 0.0, -1
        trade_start = len(trades)
        peak_equity, max_drawdown = initial_cash, 0.0
        for index in range(1, len(bars) - 1):
            if short[index] is None or long[index] is None or short[index - 1] is None or long[index - 1] is None:
                continue
            next_bar = bars[index + 1]
            if shares == 0 and short[index - 1] <= long[index - 1] and short[index] > long[index]:
                price = float(next_bar.get("open") or next_bar.get("close") or 0)
                if price > 0:
                    shares = cash * (1 - buy_cost) / price
                    cash = 0
                    entry, entry_index = price, index + 1
            elif shares > 0 and short[index - 1] >= long[index - 1] and short[index] < long[index]:
                price = float(next_bar.get("open") or next_bar.get("close") or 0)
                if price > 0:
                    cash = shares * price * (1 - sell_cost)
                    trades.append({"symbol": symbol, "entry_date": bars[entry_index]["time"], "exit_date": next_bar["time"], "entry_price": entry, "exit_price": price, "return": price / entry * (1 - buy_cost) * (1 - sell_cost) - 1, "reason": "ma_cross"})
                    shares, entry, entry_index = 0.0, 0.0, -1
            equity = cash + shares * closes[index]
            equity_points[bars[index]["time"]] = equity_points.get(bars[index]["time"], 0.0) + equity
            peak_equity = max(peak_equity, equity)
            if peak_equity > 0:
                max_drawdown = max(max_drawdown, (peak_equity - equity) / peak_equity)
        if shares > 0 and bars:
            final = float(bars[-1].get("close") or 0)
            cash = shares * final * (1 - sell_cost)
            trades.append({"symbol": symbol, "entry_date": bars[entry_index]["time"], "exit_date": bars[-1]["time"], "entry_price": entry, "exit_price": final, "return": final / entry * (1 - buy_cost) * (1 - sell_cost) - 1, "reason": "final"})
        symbol_trades = trades[trade_start:]
        symbol_returns = [float(item["return"]) for item in symbol_trades]
        per_symbol[symbol] = {
            "sample_count": len(symbol_trades),
            "win_rate": sum(1 for value in symbol_returns if value > 0) / len(symbol_returns) if symbol_returns else 0,
            "average_return": sum(symbol_returns) / len(symbol_returns) if symbol_returns else 0,
            "total_return": cash / initial_cash - 1,
            "max_drawdown": max_drawdown,
        }
    curve = [{"date": key, "equity": value} for key, value in sorted(equity_points.items())]
    total_return = 0.0
    if curve:
        total_return = curve[-1]["equity"] / (initial_cash * len(symbols)) - 1
    wins = [trade for trade in trades if trade["return"] > 0]
    return {"engine": "hikyuu", "calculation_engine": "hikyuu-data-reference", "symbols": len(symbols), "signals": len(trades), "trades": trades, "equity_curve": curve, "per_symbol": per_symbol, "metrics": {"symbols": len(symbols), "trades": len(trades), "win_rate": len(wins) / len(trades) if trades else 0, "total_return": total_return}, "warnings": warnings, "meta": {"source": "hikyuu", "data_revision": os.getenv("HIKYUU_DATA_REVISION", "runtime"), "strategy": "ma_cross_reference", "params": {"fast": fast, "slow": slow}}}
