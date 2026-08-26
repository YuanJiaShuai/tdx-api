#!/usr/bin/env python3
"""Update local A-share daily kline cache from closing quotes."""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any
from zoneinfo import ZoneInfo


DEFAULT_BASE_URL = "http://localhost:8080"
DEFAULT_DB_PATH = Path("data/market/a_share_daily.db")
DEFAULT_BATCH_SIZE = 80
SOURCE = "quote_close"


def shanghai_today() -> str:
    return datetime.now(ZoneInfo("Asia/Shanghai")).strftime("%Y-%m-%d")


def request_json(base_url: str, path: str, timeout: int = 30) -> dict[str, Any]:
    url = urllib.parse.urljoin(base_url.rstrip("/") + "/", path.lstrip("/"))
    with urllib.request.urlopen(url, timeout=timeout) as response:
        raw = response.read().decode("utf-8")
    payload = json.loads(raw)
    if not isinstance(payload, dict):
        raise RuntimeError(f"unexpected JSON payload from {url}: {type(payload).__name__}")
    if payload.get("code") not in (None, 0):
        raise RuntimeError(f"API error from {url}: {payload}")
    return payload


def fetch_codes_for_exchange(base_url: str, exchange: str) -> list[str]:
    payload = request_json(base_url, f"/api/codes?exchange={urllib.parse.quote(exchange)}", timeout=45)
    data = payload.get("data", [])
    if isinstance(data, dict):
        for key in ("list", "codes", "items", "data"):
            if isinstance(data.get(key), list):
                data = data[key]
                break
    codes: list[str] = []
    if isinstance(data, list):
        for item in data:
            code: Any
            if isinstance(item, str):
                code = item
            elif isinstance(item, dict):
                code = item.get("code") or item.get("Code") or item.get("symbol") or item.get("Symbol")
            else:
                continue
            code_str = str(code or "").strip()
            digits = "".join(ch for ch in code_str if ch.isdigit())
            if len(digits) >= 6:
                codes.append(digits[-6:])
    return codes


def fetch_all_codes(base_url: str, limit: int | None = None) -> tuple[list[str], str, list[str]]:
    errors: list[str] = []
    ordered: list[str] = []
    seen: set[str] = set()
    for exchange in ("sh", "sz", "bj"):
        try:
            codes = fetch_codes_for_exchange(base_url, exchange)
        except Exception as exc:  # noqa: BLE001 - batch job should report all exchange failures.
            errors.append(f"/api/codes?exchange={exchange}: {exc}")
            continue
        for code in codes:
            if code not in seen:
                ordered.append(code)
                seen.add(code)
    if "510300" not in seen:
        ordered.append("510300")
        seen.add("510300")
    if limit is not None:
        limited = ordered[:limit]
        if "510300" not in limited:
            limited.append("510300")
        ordered = limited
    return ordered, "api", errors


def fetch_quotes(base_url: str, codes: list[str], timeout: int = 45) -> list[dict[str, Any]]:
    quoted = urllib.parse.quote(",".join(codes))
    payload = request_json(base_url, f"/api/quote?code={quoted}", timeout=timeout)
    data = payload.get("data", [])
    if isinstance(data, dict):
        data = [data]
    if not isinstance(data, list):
        raise RuntimeError(f"unexpected quote data: {type(data).__name__}")
    return [item for item in data if isinstance(item, dict)]


def fetch_quotes_resilient(base_url: str, codes: list[str], batch_size: int, errors: list[str]) -> list[dict[str, Any]]:
    quotes: list[dict[str, Any]] = []
    for start in range(0, len(codes), batch_size):
        batch = codes[start : start + batch_size]
        try:
            quotes.extend(fetch_quotes(base_url, batch))
            continue
        except Exception as exc:  # noqa: BLE001 - retry individually to salvage valid rows.
            errors.append(f"batch {batch[0]}..{batch[-1]}: {exc}")
        for code in batch:
            try:
                quotes.extend(fetch_quotes(base_url, [code], timeout=20))
            except Exception as exc:  # noqa: BLE001
                errors.append(f"{code}: {exc}")
    return quotes


def to_float(value: Any) -> float:
    try:
        return float(value or 0)
    except (TypeError, ValueError):
        return 0.0


def price_scale(k: dict[str, Any]) -> float:
    values = [abs(to_float(k.get(name))) for name in ("Open", "High", "Low", "Close", "Last")]
    values = sorted(v for v in values if v > 0)
    if not values:
        return 1.0
    return 1000.0 if values[len(values) // 2] > 1000 else 1.0


def quote_to_row(quote: dict[str, Any], run_date: str) -> tuple[tuple[Any, ...] | None, str | None]:
    code = str(quote.get("Code") or quote.get("code") or "").zfill(6)[-6:]
    k = quote.get("K") or {}
    if not code.isdigit() or not isinstance(k, dict):
        return None, f"{code or 'unknown'}: missing code/K"
    scale = price_scale(k)
    open_price = to_float(k.get("Open")) / scale
    high = to_float(k.get("High")) / scale
    low = to_float(k.get("Low")) / scale
    close = to_float(k.get("Close")) / scale
    volume = to_float(quote.get("TotalHand"))
    amount = to_float(quote.get("Amount"))
    if amount <= 0 and close > 0 and volume > 0:
        amount = close * volume * 100
    if min(open_price, high, low, close) <= 0:
        return None, f"{code}: invalid OHLC open={open_price} high={high} low={low} close={close}"
    return (code, run_date, open_price, high, low, close, volume, amount, SOURCE), None


def ensure_schema(conn: sqlite3.Connection) -> None:
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS daily_kline (
            code TEXT NOT NULL,
            date TEXT NOT NULL,
            open REAL NOT NULL,
            high REAL NOT NULL,
            low REAL NOT NULL,
            close REAL NOT NULL,
            volume REAL NOT NULL,
            amount REAL,
            source TEXT NOT NULL DEFAULT 'api',
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (code, date)
        )
        """
    )
    conn.execute("CREATE INDEX IF NOT EXISTS idx_daily_kline_code_date ON daily_kline (code, date)")
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS update_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            run_date TEXT NOT NULL,
            source TEXT NOT NULL,
            status TEXT NOT NULL,
            rows INTEGER NOT NULL DEFAULT 0,
            message TEXT,
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        )
        """
    )


def write_update_log(db_path: Path, run_date: str, status: str, rows: int, message: str) -> None:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(db_path) as conn:
        ensure_schema(conn)
        conn.execute(
            "INSERT INTO update_log (run_date, source, status, rows, message) VALUES (?, ?, ?, ?, ?)",
            (run_date, SOURCE, status, rows, message),
        )


def upsert_rows(db_path: Path, rows: list[tuple[Any, ...]]) -> int:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(db_path) as conn:
        ensure_schema(conn)
        conn.executemany(
            """
            INSERT INTO daily_kline (code, date, open, high, low, close, volume, amount, source, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(code, date) DO UPDATE SET
                open = excluded.open,
                high = excluded.high,
                low = excluded.low,
                close = excluded.close,
                volume = excluded.volume,
                amount = excluded.amount,
                source = excluded.source,
                updated_at = CURRENT_TIMESTAMP
            """,
            rows,
        )
        return conn.execute("SELECT changes()").fetchone()[0]


def write_log(path: Path, lines: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL)
    parser.add_argument("--date", default=shanghai_today())
    parser.add_argument("--db-path", default=str(DEFAULT_DB_PATH))
    parser.add_argument("--batch-size", type=int, default=DEFAULT_BATCH_SIZE)
    parser.add_argument("--limit", type=int, default=None, help="Optional code limit for smoke tests.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    started = time.monotonic()
    db_path = Path(args.db_path)
    log_path = Path("reports/logs") / f"update_daily_quotes_{args.date}.log"
    errors: list[str] = []
    requested_codes = 0
    success_quotes = 0
    written_rows = 0
    skipped_rows = 0
    code_source = "api"

    try:
        request_json(args.base_url, "/api/quote?code=000001", timeout=8)
        codes, code_source, code_errors = fetch_all_codes(args.base_url, args.limit)
        errors.extend(code_errors)
        requested_codes = len(codes)
        if not codes:
            raise RuntimeError("no codes loaded from /api/codes")
        quotes = fetch_quotes_resilient(args.base_url, codes, max(1, args.batch_size), errors)
        success_quotes = len(quotes)
        rows: list[tuple[Any, ...]] = []
        for quote in quotes:
            row, skip_reason = quote_to_row(quote, args.date)
            if row is None:
                skipped_rows += 1
                if skip_reason:
                    errors.append(skip_reason)
                continue
            rows.append(row)
        written_rows = upsert_rows(db_path, rows) if rows else 0
        status = "success" if rows else "failed"
        message = (
            f"date={args.date}, code_source={code_source}, requested_codes={requested_codes}, "
            f"success_quotes={success_quotes}, written_rows={written_rows}, skipped_rows={skipped_rows}, "
            f"errors={len(errors)}, db={db_path}, log={log_path}, elapsed={time.monotonic() - started:.1f}s"
        )
        write_update_log(db_path, args.date, status, written_rows, message)
        lines = [message, "errors:"] + errors[:300] if errors else [message]
        write_log(log_path, lines)
        print(message)
        return 0 if rows else 1
    except Exception as exc:  # noqa: BLE001 - top-level batch failure should be recorded.
        errors.append(str(exc))
        message = (
            f"date={args.date}, code_source={code_source if requested_codes else 'not_loaded_api_unavailable'}, "
            f"requested_codes={requested_codes}, success_quotes={success_quotes}, written_rows={written_rows}, "
            f"skipped_rows={skipped_rows}, errors={len(errors)}, db={db_path}, log={log_path}, "
            f"elapsed={time.monotonic() - started:.1f}s"
        )
        write_log(log_path, [message, "errors:"] + errors[:300])
        write_update_log(db_path, args.date, "failed", 0, message)
        print(message, file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
