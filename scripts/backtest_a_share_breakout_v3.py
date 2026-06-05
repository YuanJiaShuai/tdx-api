#!/usr/bin/env python3
"""Backtest a 1-2 month A-share breakout-pullback strategy."""

from __future__ import annotations

import argparse
import json
import urllib.request
from dataclasses import dataclass

import pandas as pd


URL = "http://localhost:8080/api/kline?code={code}&type=day"


@dataclass(frozen=True)
class V3Params:
    market_code: str = "510300"
    market_short: int = 20
    market_long: int = 60
    ret20_min: float = 0.08
    ret60_min: float = 0.15
    max_dist_high60: float = 0.15
    pullback_min: float = 0.02
    pullback_max: float = 0.14
    vol_breakout: float = 0.80
    stop_loss: float = 0.08
    profit_trigger: float = 0.12
    trailing_stop: float = 0.10
    max_hold: int = 40


def fetch(code: str) -> pd.DataFrame:
    with urllib.request.urlopen(URL.format(code=code), timeout=20) as resp:
        payload = json.loads(resp.read())
    if payload.get("code") != 0:
        raise RuntimeError(payload)

    df = pd.DataFrame(payload["data"]["List"])
    df["date"] = pd.to_datetime(df["Time"]).dt.tz_localize(None)
    for col in ["Open", "High", "Low", "Close"]:
        df[col.lower()] = df[col].astype(float)
    df["volume"] = df["Volume"].astype(float)
    return df[["date", "open", "high", "low", "close", "volume"]].sort_values("date").reset_index(drop=True)


def prepare(stock: pd.DataFrame, market: pd.DataFrame, p: V3Params) -> pd.DataFrame:
    s = stock.copy()
    s["ma5"] = s.close.rolling(5).mean()
    s["ma20"] = s.close.rolling(20).mean()
    s["ma60"] = s.close.rolling(60).mean()
    s["ma120"] = s.close.rolling(120).mean()
    s["vol20"] = s.volume.rolling(20).mean()
    s["vol60"] = s.volume.rolling(60).mean()
    s["hh60"] = s.high.rolling(60).max()
    s["recent5_high"] = s.high.rolling(5).max()
    s["recent5_low"] = s.low.rolling(5).min()
    s["recent5_hh60"] = s.high.rolling(5).max() >= s.hh60.shift(1)
    s["ret20"] = s.close / s.close.shift(20) - 1
    s["ret60"] = s.close / s.close.shift(60) - 1
    s["pullback_from_recent5_high"] = s.close / s.recent5_high - 1

    m = market[["date", "close"]].copy()
    m["m_ma20"] = m.close.rolling(p.market_short).mean()
    m["m_ma60"] = m.close.rolling(p.market_long).mean()
    m["market_ok"] = (m.close > m.m_ma60) & (m.m_ma20 > m.m_ma60)

    s = s.merge(m[["date", "market_ok"]], on="date", how="left")
    s["market_ok"] = s.market_ok.ffill().fillna(False)
    return s


def entry_signal(df: pd.DataFrame, i: int, p: V3Params) -> bool:
    r = df.iloc[i]
    prev = df.iloc[i - 1]
    trend = r.close > r.ma60 and r.ma20 > r.ma60 > r.ma120
    strength = r.ret20 >= p.ret20_min and r.ret60 >= p.ret60_min
    near_high = r.close >= r.hh60 * (1 - p.max_dist_high60)
    pullback = -p.pullback_max <= r.pullback_from_recent5_high <= -p.pullback_min
    did_not_break_ma20 = r.close >= r.ma20 and r.recent5_low >= r.ma20 * 0.98
    turn_up = r.close > r.open and r.close > prev.close and r.close > r.ma5
    volume_ok = r.volume >= r.vol20 * p.vol_breakout and r.vol20 > r.vol60
    return bool(
        r.market_ok
        and trend
        and strength
        and near_high
        and r.recent5_hh60
        and pullback
        and did_not_break_ma20
        and turn_up
        and volume_ok
    )


def backtest(df: pd.DataFrame, p: V3Params, start: str | None = None, initial_cash: float = 100_000) -> tuple[dict, pd.DataFrame]:
    if start:
        df = df[df.date >= pd.Timestamp(start)].reset_index(drop=True)

    cash = initial_cash
    shares = 0.0
    entry = 0.0
    entry_date = None
    entry_i = 0
    high_since_entry = 0.0
    trades = []
    equity = []
    start_i = 125

    for i in range(start_i, len(df) - 1):
        r = df.iloc[i]
        nxt = df.iloc[i + 1]
        equity.append(cash + shares * r.close)

        if shares <= 0:
            if entry_signal(df, i, p):
                entry = nxt.open
                shares = cash * (1 - 0.0005) / entry
                cash = 0.0
                entry_date = nxt.date
                entry_i = i + 1
                high_since_entry = entry
            continue

        high_since_entry = max(high_since_entry, r.high)
        hold = i - entry_i
        gain = r.close / entry - 1
        draw_from_high = r.close / high_since_entry - 1
        exit_reason = None

        if gain <= -p.stop_loss:
            exit_reason = "stop"
        elif gain >= p.profit_trigger and draw_from_high <= -p.trailing_stop:
            exit_reason = "trail"
        elif r.close < r.ma20:
            exit_reason = "ma20"
        elif hold >= p.max_hold:
            exit_reason = "time"

        if exit_reason:
            exit_price = nxt.open
            proceeds = shares * exit_price * (1 - 0.001)
            trades.append(
                {
                    "entry_date": entry_date,
                    "exit_date": nxt.date,
                    "entry": entry,
                    "exit": exit_price,
                    "return": proceeds / (shares * entry) - 1,
                    "hold_days": hold + 1,
                    "reason": exit_reason,
                }
            )
            cash = proceeds
            shares = 0.0

    if shares > 0:
        r = df.iloc[-1]
        proceeds = shares * r.close * (1 - 0.001)
        trades.append(
            {
                "entry_date": entry_date,
                "exit_date": r.date,
                "entry": entry,
                "exit": r.close,
                "return": proceeds / (shares * entry) - 1,
                "hold_days": len(df) - 1 - entry_i,
                "reason": "final",
            }
        )
        cash = proceeds
        equity.append(cash)

    trades_df = pd.DataFrame(trades)
    metrics = calc_metrics(pd.Series(equity, dtype=float), trades_df, initial_cash, df, start_i)
    return metrics, trades_df


def calc_metrics(equity: pd.Series, trades: pd.DataFrame, initial_cash: float, df: pd.DataFrame, start_i: int) -> dict:
    if equity.empty:
        return {
            "trades": 0,
            "win_rate": 0.0,
            "total_return": 0.0,
            "cagr": 0.0,
            "max_drawdown": 0.0,
            "profit_factor": 0.0,
            "avg_trade": 0.0,
            "exposure": 0.0,
        }

    wins = trades.loc[trades["return"] > 0, "return"] if not trades.empty else pd.Series(dtype=float)
    losses = trades.loc[trades["return"] <= 0, "return"] if not trades.empty else pd.Series(dtype=float)
    peak = equity.cummax()
    years = max((df.date.iloc[-1] - df.date.iloc[min(start_i, len(df) - 1)]).days / 365.25, 1e-9)
    profit_factor = wins.sum() / abs(losses.sum()) if abs(losses.sum()) > 0 else float("inf")

    return {
        "trades": int(len(trades)),
        "win_rate": float(len(wins) / len(trades)) if len(trades) else 0.0,
        "total_return": float(equity.iloc[-1] / initial_cash - 1),
        "cagr": float((equity.iloc[-1] / initial_cash) ** (1 / years) - 1),
        "max_drawdown": float((equity / peak - 1).min()),
        "profit_factor": float(profit_factor),
        "avg_trade": float(trades["return"].mean()) if not trades.empty else 0.0,
        "exposure": float(trades["hold_days"].sum() / max(len(df), 1)) if not trades.empty else 0.0,
    }


def pct(x: float) -> str:
    return f"{x * 100:.2f}%"


def print_metrics(title: str, metrics: dict) -> None:
    print(f"\n{title}")
    print("-" * len(title))
    print(f"trades        : {metrics['trades']}")
    print(f"win_rate      : {pct(metrics['win_rate'])}")
    print(f"total_return  : {pct(metrics['total_return'])}")
    print(f"cagr          : {pct(metrics['cagr'])}")
    print(f"max_drawdown  : {pct(metrics['max_drawdown'])}")
    print(f"profit_factor : {metrics['profit_factor']:.2f}")
    print(f"avg_trade     : {pct(metrics['avg_trade'])}")
    print(f"exposure      : {pct(metrics['exposure'])}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("codes", nargs="+")
    parser.add_argument("--market", default="510300")
    args = parser.parse_args()

    p = V3Params(market_code=args.market)
    market = fetch(p.market_code)
    print(f"market filter: {p.market_code}, close > MA60 and MA20 > MA60")
    print("strategy: 1-2 month breakout-pullback, next-day open execution")
    print("costs: buy 0.05%, sell 0.10%")
    print(f"params: {p}")

    for code in args.codes:
        stock = fetch(code)
        df = prepare(stock, market, p)
        full_m, full_t = backtest(df, p)
        test_m, test_t = backtest(df, p, start="2021-01-01")

        print(f"\n===== {code} =====")
        print(f"bars: {stock.date.iloc[0].date()} -> {stock.date.iloc[-1].date()}, count={len(stock)}")
        print_metrics("full sample", full_m)
        print_metrics("since 2021-01-01", test_m)

        if not full_t.empty:
            out = full_t.tail(10).copy()
            out["entry_date"] = out.entry_date.dt.strftime("%Y-%m-%d")
            out["exit_date"] = out.exit_date.dt.strftime("%Y-%m-%d")
            out["return"] = out["return"].map(pct)
            print("\nlatest trades")
            print(out.to_string(index=False))

        r = df.iloc[-1]
        signal = entry_signal(df, len(df) - 1, p)
        print(
            "\nlatest bar: "
            f"{r.date.date()} close={r.close:.0f}, ma5={r.ma5:.0f}, ma20={r.ma20:.0f}, "
            f"ma60={r.ma60:.0f}, ma120={r.ma120:.0f}, market_ok={bool(r.market_ok)}, "
            f"entry_signal={signal}"
        )


if __name__ == "__main__":
    main()
