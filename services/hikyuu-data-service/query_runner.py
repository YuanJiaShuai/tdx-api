import argparse
import contextlib
import io
import json
import os
import sys
from datetime import datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo


TZ_NAME = os.getenv("TZ", "Asia/Shanghai")
TZ = ZoneInfo(TZ_NAME)
CONFIG_FILE = Path(os.getenv("HIKYUU_CONFIG_DIR", "/root/.hikyuu")) / "hikyuu.ini"

PERIODS = {
    "day": "DAY",
    "minute1": "MIN",
    "min": "MIN",
    "minute5": "MIN5",
    "min5": "MIN5",
    "week": "WEEK",
    "month": "MONTH",
}

RECOVER_TYPES = {
    "none": "NO_RECOVER",
    "no": "NO_RECOVER",
    "forward": "FORWARD",
    "qfq": "FORWARD",
    "backward": "BACKWARD",
    "hfq": "BACKWARD",
    "equal_forward": "EQUAL_FORWARD",
    "equal_backward": "EQUAL_BACKWARD",
}


def parse_date(value: str, end: bool = False):
    if not value:
        return None
    for layout in ("%Y%m%d", "%Y-%m-%d"):
        try:
            parsed = datetime.strptime(value, layout)
            if end:
                parsed += timedelta(days=1)
            return parsed
        except ValueError:
            continue
    raise ValueError("date must be YYYYMMDD or YYYY-MM-DD")


def normalize_symbol(value: str) -> str:
    value = value.strip().upper().replace("_", ".").replace("-", ".").replace(":", ".")
    if "." in value:
        code, market = value.split(".", 1)
    elif len(value) == 8 and value[:2] in ("SH", "SZ", "BJ"):
        market, code = value[:2], value[2:]
    else:
        raise ValueError("symbol must be like 000001.SZ")
    if market not in ("SH", "SZ", "BJ") or len(code) != 6 or not code.isdigit():
        raise ValueError("invalid symbol")
    return f"{market.lower()}{code}"


def datetime_text(value) -> str:
    result = value.datetime()
    if result.tzinfo is None:
        result = result.replace(tzinfo=TZ)
    return result.isoformat()


def record_json(record, last_close: float) -> dict:
    return {
        "time": datetime_text(record.datetime),
        "last": last_close,
        "open": float(record.open),
        "high": float(record.high),
        "low": float(record.low),
        "close": float(record.close),
        "volume": float(record.volume),
        "amount": float(record.amount),
    }


def load_records(symbol: str, period: str, start: str, end: str, limit: int, recover: str) -> dict:
    if period not in PERIODS:
        raise ValueError("unsupported period")
    if recover not in RECOVER_TYPES:
        raise ValueError("unsupported recover type")

    native_output = io.StringIO()
    with contextlib.redirect_stdout(native_output), contextlib.redirect_stderr(native_output):
        from hikyuu import Datetime, Query, StockManager, hikyuu_init

        if not CONFIG_FILE.exists():
            raise FileNotFoundError(f"missing hikyuu config: {CONFIG_FILE}")
        hikyuu_init(str(CONFIG_FILE), ignore_preload=True)
        stock = StockManager.instance()[normalize_symbol(symbol)]
        if stock.is_null():
            raise LookupError(f"symbol not found: {symbol}")

        ktype = getattr(Query, PERIODS[period])
        recover_type = getattr(Query, RECOVER_TYPES[recover])
        start_date = parse_date(start)
        end_date = parse_date(end, end=True)

        if start_date is not None or end_date is not None:
            query = Query(Datetime(start_date or datetime(1990, 1, 1)), Datetime(end_date) if end_date else None, ktype=ktype, recover_type=recover_type)
        elif limit > 0:
            query = Query(-limit, ktype=ktype, recover_type=recover_type)
        else:
            query = Query(0, ktype=ktype, recover_type=recover_type)

        records = stock.get_kdata(query)
        values = []
        previous_close = 0.0
        for record in records:
            values.append(record_json(record, previous_close))
            previous_close = float(record.close)

    if not values:
        raise LookupError("no kline data")
    if limit > 0 and not (start or end) and len(values) > limit:
        values = values[-limit:]

    return {
        "symbol": symbol,
        "period": period,
        "recover": recover,
        "count": len(values),
        "list": values,
        "meta": {"source": "hikyuu"},
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--symbol", required=True)
    parser.add_argument("--period", default="day")
    parser.add_argument("--start", default="")
    parser.add_argument("--end", default="")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--recover", default="none")
    args = parser.parse_args()

    try:
        result = load_records(
            args.symbol,
            args.period.lower(),
            args.start,
            args.end,
            max(0, args.limit),
            args.recover.lower(),
        )
    except Exception as exc:
        print(str(exc), file=sys.stderr, flush=True)
        return 1

    print(json.dumps(result, ensure_ascii=False), flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
