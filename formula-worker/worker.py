#!/usr/bin/env python3
import json
import math
import os
import re
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HQCHARTPY2 = None
ENGINE_STATUS = {
    "engine": "fallback",
    "hqchartpy2_available": False,
    "message": "HQChartPy2 not installed; using fallback evaluator",
    "error": "",
}

try:
    import HQChartPy2
    HQCHARTPY2 = HQChartPy2
    try:
        HQCHARTPY2.SetLog(0)
    except Exception:
        pass
    ENGINE_STATUS = {
        "engine": "hqchartpy2",
        "hqchartpy2_available": True,
        "message": "HQChartPy2 module detected; using C++ evaluator with fallback",
        "version": str(HQCHARTPY2.GetVersion()) if hasattr(HQCHARTPY2, "GetVersion") else "",
        "py_version": str(HQCHARTPY2.GetPyVersion()) if hasattr(HQCHARTPY2, "GetPyVersion") else "",
        "error": "",
    }
except Exception as exc:
    ENGINE_STATUS["error"] = str(exc)


class Series:
    def __init__(self, values):
        self.values = [float(v) if v is not None else 0.0 for v in values]

    def __len__(self):
        return len(self.values)

    def _binary(self, other, op):
        if isinstance(other, Series):
            return Series([op(a, b) for a, b in zip(self.values, other.values)])
        return Series([op(a, float(other)) for a in self.values])

    def __add__(self, other):
        return self._binary(other, lambda a, b: a + b)

    def __radd__(self, other):
        return self.__add__(other)

    def __sub__(self, other):
        return self._binary(other, lambda a, b: a - b)

    def __rsub__(self, other):
        return Series([float(other) - a for a in self.values])

    def __mul__(self, other):
        return self._binary(other, lambda a, b: a * b)

    def __rmul__(self, other):
        return self.__mul__(other)

    def __truediv__(self, other):
        return self._binary(other, lambda a, b: a / b if b else 0.0)

    def __rtruediv__(self, other):
        return Series([float(other) / a if a else 0.0 for a in self.values])

    def __neg__(self):
        return Series([-a for a in self.values])

    def __gt__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a > b else 0.0)

    def __ge__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a >= b else 0.0)

    def __lt__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a < b else 0.0)

    def __le__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a <= b else 0.0)

    def __eq__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a == b else 0.0)

    def __and__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a and b else 0.0)

    def __or__(self, other):
        return self._binary(other, lambda a, b: 1.0 if a or b else 0.0)

    def last(self):
        return self.values[-1] if self.values else 0.0

    def tail(self, count):
        if count is None or count < 0:
            return self.values
        return self.values[-count:]


def as_series(value, length):
    if isinstance(value, Series):
        return value
    return Series([value] * length)


def MA(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    out = []
    for i in range(len(x.values)):
        start = max(0, i - n + 1)
        window = x.values[start : i + 1]
        out.append(sum(window) / len(window))
    return Series(out)


def EMA(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    alpha = 2 / (n + 1)
    out = []
    prev = x.values[0] if x.values else 0.0
    for v in x.values:
        prev = alpha * v + (1 - alpha) * prev
        out.append(prev)
    return Series(out)


def SMA(x, n, m=1):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    m = float(m)
    out = []
    prev = x.values[0] if x.values else 0.0
    for v in x.values:
        prev = (m * v + (n - m) * prev) / n
        out.append(prev)
    return Series(out)


def REF(x, n=1):
    x = as_series(x, 0)
    n = max(int(float(n)), 0)
    out = []
    for i in range(len(x.values)):
        out.append(x.values[i - n] if i >= n else 0.0)
    return Series(out)


def LLV(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    out = []
    for i in range(len(x.values)):
        start = max(0, i - n + 1)
        out.append(min(x.values[start : i + 1]))
    return Series(out)


def HHV(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    out = []
    for i in range(len(x.values)):
        start = max(0, i - n + 1)
        out.append(max(x.values[start : i + 1]))
    return Series(out)


def CROSS(a, b):
    a = as_series(a, 0)
    b = as_series(b, len(a))
    out = [0.0]
    for i in range(1, min(len(a.values), len(b.values))):
        out.append(1.0 if a.values[i - 1] <= b.values[i - 1] and a.values[i] > b.values[i] else 0.0)
    return Series(out)


def ABS(x):
    x = as_series(x, 0)
    return Series([abs(v) for v in x.values])


def MAX(a, b):
    a = as_series(a, 0)
    b = as_series(b, len(a))
    return Series([max(x, y) for x, y in zip(a.values, b.values)])


def MIN(a, b):
    a = as_series(a, 0)
    b = as_series(b, len(a))
    return Series([min(x, y) for x, y in zip(a.values, b.values)])


def IF(cond, a, b):
    cond = as_series(cond, 0)
    a = as_series(a, len(cond))
    b = as_series(b, len(cond))
    return Series([x if c else y for c, x, y in zip(cond.values, a.values, b.values)])


def SUM(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    out = []
    for i in range(len(x.values)):
        start = max(0, i - n + 1)
        out.append(sum(x.values[start : i + 1]))
    return Series(out)


def STD(x, n):
    x = as_series(x, 0)
    n = max(int(float(n)), 1)
    out = []
    for i in range(len(x.values)):
        start = max(0, i - n + 1)
        window = x.values[start : i + 1]
        avg = sum(window) / len(window)
        out.append(math.sqrt(sum((v - avg) ** 2 for v in window) / len(window)))
    return Series(out)


def period_to_id(period):
    value = str(period or "").strip().lower()
    if value in ("1", "week", "w"):
        return 1
    if value in ("2", "month", "m"):
        return 2
    if value in ("4", "minute1", "1m", "min1"):
        return 4
    if value in ("5", "minute5", "5m", "min5"):
        return 5
    if value in ("6", "minute15", "15m", "min15"):
        return 6
    if value in ("7", "minute30", "30m", "min30"):
        return 7
    if value in ("8", "hour", "60m", "min60"):
        return 8
    return 0


def to_hq_symbol(symbol):
    value = str(symbol or "").strip().lower()
    if "." in value:
        return value
    if value.startswith(("6", "9")):
        return value + ".sh"
    return value + ".sz"


def normalize_args(args):
    if isinstance(args, str):
        try:
            args = json.loads(args)
        except Exception:
            args = []
    return args if isinstance(args, list) else []


class InMemoryHQData:
    def __init__(self, data):
        self.data = data or {}

    def _rows_for(self, symbol):
        candidates = [
            symbol,
            str(symbol).upper(),
            str(symbol).lower(),
            str(symbol).split(".")[0],
            str(symbol).split(".")[0].upper(),
        ]
        for item in candidates:
            if item in self.data:
                return self.data[item]
        return []

    def GetKLineData(self, symbol, period, right, jobID):
        return self._kline_payload(symbol, period, right)

    def GetKLineData2(self, symbol, period, right, callInfo, kdataInfo, jobID):
        return self._kline_payload(symbol, period, right)

    def _kline_payload(self, symbol, period, right):
        rows = self._rows_for(symbol)
        payload = {
            "symbol": to_hq_symbol(symbol),
            "name": to_hq_symbol(symbol),
            "period": int(period or 0),
            "right": int(right or 0),
            "count": len(rows),
            "date": [],
            "time": [],
            "yclose": [],
            "open": [],
            "high": [],
            "low": [],
            "close": [],
            "vol": [],
            "amount": [],
        }
        for row in rows:
            payload["date"].append(int(row.get("date", 0) or 0))
            payload["time"].append(int(row.get("time", 0) or 0))
            payload["yclose"].append(float(row.get("yclose", 0) or 0))
            payload["open"].append(float(row.get("open", 0) or 0))
            payload["high"].append(float(row.get("high", 0) or 0))
            payload["low"].append(float(row.get("low", 0) or 0))
            payload["close"].append(float(row.get("close", 0) or 0))
            payload["vol"].append(float(row.get("vol", 0) or 0))
            payload["amount"].append(float(row.get("amount", 0) or 0))
        return payload

    def GetFinance(self, symbol, id, period, right, kcount, jobID):
        return {"type": 1, "data": [0] * int(kcount or 0)}

    def GetDynainfo(self, symbol, id, period, right, kcount, jobID):
        return {"type": 0, "data": 0}

    def GetCapital(self, symbol, period, right, kcount, jobID):
        return {"type": 0, "data": 0}

    def GetTotalCapital(self, symbol, period, right, kcount, jobID):
        return {"type": 0, "data": 0}

    def GetHisCapital(self, symbol, period, right, kcount, jobID):
        return {"type": 1, "data": [0] * int(kcount or 0)}

    def GetDataByNumber(self, symbol, funcName, id, period, right, kcount, jobID):
        if funcName == "FINANCE":
            return self.GetFinance(symbol, id, period, right, kcount, jobID)
        if funcName == "DYNAINFO":
            return self.GetDynainfo(symbol, id, period, right, kcount, jobID)
        return False

    def GetDataByNumbers(self, symbol, funcName, args, period, right, kcount, jobID, aryDate=None, aryTime=None):
        return False

    def GetDataByName(self, symbol, funcName, period, right, kcount, jobID):
        if funcName == "CAPITAL":
            return self.GetCapital(symbol, period, right, kcount, jobID)
        if funcName == "TOTALCAPITAL":
            return self.GetTotalCapital(symbol, period, right, kcount, jobID)
        if funcName == "GetHisCapital":
            return self.GetHisCapital(symbol, period, right, kcount, jobID)
        return False

    def GetDataByString(self, symbol, funcName, period, right, kcount, jobID):
        return False

    def GetIndexScript(self, name, callInfo, jobID):
        return None


class HQChartPy2Result:
    def __init__(self):
        self.data = {}
        self.errors = []

    def success(self, symbol, js_data, job_id):
        self.data[str(symbol)] = js_data

    def failed(self, code, symbol, error, job_id):
        self.errors.append({"code": code, "symbol": symbol, "error": error, "job_id": job_id})


def parse_hqchartpy2_output(raw, out_count):
    if isinstance(raw, str):
        payload = json.loads(raw)
    else:
        payload = raw or {}

    outputs = {}
    if isinstance(payload, dict):
        if isinstance(payload.get("OutVar"), list):
            for item in payload["OutVar"]:
                name = str(item.get("Name") or item.get("name") or item.get("ID") or "RESULT").upper()
                values = item.get("Data") or item.get("data") or []
                outputs[name] = values[-out_count:] if out_count and out_count > 0 else values
        elif isinstance(payload.get("Data"), list):
            outputs["RESULT"] = payload["Data"][-out_count:] if out_count and out_count > 0 else payload["Data"]
        else:
            for key, value in payload.items():
                if isinstance(value, list):
                    outputs[str(key).upper()] = value[-out_count:] if out_count and out_count > 0 else value

    last_values = {}
    for name, values in outputs.items():
        last_values[name] = next((v for v in reversed(values) if v is not None), 0)
    latest = next(reversed(last_values.values())) if last_values else 0
    try:
        latest_float = float(latest or 0)
    except Exception:
        latest_float = 0.0

    return {
        "hit": bool(latest_float),
        "latest": latest_float,
        "last_values": last_values,
        "series": outputs,
        "raw": payload,
    }


def try_eval_hqchartpy2(script, data, args=None, period="day", right=0, out_count=1):
    if HQCHARTPY2 is None:
        raise RuntimeError("HQChartPy2 not available")

    symbols = [to_hq_symbol(symbol) for symbol in data.keys()]
    result = HQChartPy2Result()
    hq_data = InMemoryHQData(data)
    callback_config = {
        "GetKLineData": hq_data.GetKLineData,
        "GetKLineData2": hq_data.GetKLineData2,
        "GetDataByNumber": hq_data.GetDataByNumber,
        "GetDataByNumbers": hq_data.GetDataByNumbers,
        "GetDataByName": hq_data.GetDataByName,
        "GetDataByString": hq_data.GetDataByString,
        "GetIndexScript": hq_data.GetIndexScript,
        "Success": result.success,
        "Failed": result.failed,
    }
    run_config = {
        "Script": script,
        "Args": normalize_args(args),
        "Period": period_to_id(period),
        "Right": int(right or 0),
        "Symbol": symbols[0] if len(symbols) == 1 else symbols,
        "OutCount": int(out_count or 1),
        "JobID": str(uuid.uuid4()),
    }
    ok = HQCHARTPY2.Run(json.dumps(run_config, ensure_ascii=False), callback_config)
    if not ok or result.errors:
        raise RuntimeError(json.dumps(result.errors, ensure_ascii=False) if result.errors else "HQChartPy2 run failed")

    parsed = {}
    for symbol, raw in result.data.items():
        parsed[symbol.split(".")[0].upper()] = parse_hqchartpy2_output(raw, int(out_count or 1))
    return parsed


SAFE_FUNCS = {
    "MA": MA,
    "EMA": EMA,
    "SMA": SMA,
    "REF": REF,
    "LLV": LLV,
    "HHV": HHV,
    "CROSS": CROSS,
    "ABS": ABS,
    "MAX": MAX,
    "MIN": MIN,
    "IF": IF,
    "SUM": SUM,
    "STD": STD,
}


def split_script(script):
    cleaned = []
    for line in script.replace("\r", "\n").split("\n"):
        line = line.strip()
        if not line or line.startswith("#") or line.startswith("//"):
            continue
        cleaned.append(line)
    return [part.strip() for part in ";".join(cleaned).split(";") if part.strip()]


def normalize_expr(expr):
    expr = expr.strip()
    expr = re.sub(r"\bAND\b", "&", expr, flags=re.IGNORECASE)
    expr = re.sub(r"\bOR\b", "|", expr, flags=re.IGNORECASE)
    return expr


def eval_script(script, rows, args=None, out_count=1):
    length = len(rows)
    env = {
        "DATE": Series([r.get("date", 0) for r in rows]),
        "TIME": Series([r.get("time", 0) for r in rows]),
        "C": Series([r.get("close", 0) for r in rows]),
        "CLOSE": Series([r.get("close", 0) for r in rows]),
        "O": Series([r.get("open", 0) for r in rows]),
        "OPEN": Series([r.get("open", 0) for r in rows]),
        "H": Series([r.get("high", 0) for r in rows]),
        "HIGH": Series([r.get("high", 0) for r in rows]),
        "L": Series([r.get("low", 0) for r in rows]),
        "LOW": Series([r.get("low", 0) for r in rows]),
        "V": Series([r.get("vol", 0) for r in rows]),
        "VOL": Series([r.get("vol", 0) for r in rows]),
        "AMOUNT": Series([r.get("amount", 0) for r in rows]),
    }
    env.update(SAFE_FUNCS)
    for arg in args or []:
        if isinstance(arg, dict) and arg.get("Name"):
            env[str(arg["Name"]).upper()] = float(arg.get("Value", 0))

    outputs = {}
    final_name = "RESULT"
    for stmt in split_script(script):
        name = None
        expr = stmt
        visible = True
        if ":=" in stmt:
            name, expr = stmt.split(":=", 1)
            visible = False
        elif ":" in stmt:
            name, expr = stmt.split(":", 1)
            visible = True
        if name:
            name = name.strip().upper()
        expr = normalize_expr(expr)
        value = eval(expr, {"__builtins__": {}}, env)
        if not isinstance(value, Series):
            value = Series([value] * length)
        if name:
            env[name] = value
            if visible:
                outputs[name] = value
                final_name = name
        else:
            outputs[final_name] = value
    if not outputs and final_name in env:
        outputs[final_name] = env[final_name]

    serial = {}
    last_values = {}
    for key, value in outputs.items():
        serial[key] = value.tail(out_count)
        last_values[key] = value.last()
    latest = next(reversed(last_values.values())) if last_values else 0.0
    return {
        "hit": bool(latest),
        "latest": latest,
        "last_values": last_values,
        "series": serial,
    }


class WorkerHandler(BaseHTTPRequestHandler):
    def _send(self, payload, status=200):
        raw = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if self.path == "/health":
            self._send({"code": 0, "message": "ok", **ENGINE_STATUS})
            return
        self._send({"code": -1, "message": "not found"}, 404)

    def do_POST(self):
        if self.path != "/api/formula/run":
            self._send({"code": -1, "message": "not found"}, 404)
            return
        start = time.time()
        try:
            length = int(self.headers.get("Content-Length", "0"))
            body = self.rfile.read(length)
            req = json.loads(body.decode("utf-8"))
            script = req.get("script") or req.get("Script")
            if not script:
                raise ValueError("script不能为空")
            args = req.get("args") or []
            out_count = int(req.get("out_count") or req.get("OutCount") or 1)
            data = req.get("data") or {}
            if not data:
                raise ValueError("data不能为空")
            engine = "fallback"
            fallback_error = ""
            if HQCHARTPY2 is not None:
                try:
                    result = try_eval_hqchartpy2(
                        script,
                        data,
                        args=args,
                        period=req.get("period") or req.get("Period") or "day",
                        right=req.get("right") or req.get("Right") or 0,
                        out_count=out_count,
                    )
                    engine = "hqchartpy2"
                except Exception as exc:
                    fallback_error = str(exc)
                    result = {}
                    for symbol, rows in data.items():
                        result[symbol] = eval_script(script, rows, args=args, out_count=out_count)
            else:
                result = {}
                for symbol, rows in data.items():
                    result[symbol] = eval_script(script, rows, args=args, out_count=out_count)
            self._send({
                "code": 0,
                "message": "success",
                "engine": engine,
                "hqchartpy2_available": ENGINE_STATUS["hqchartpy2_available"],
                "fallback_error": fallback_error,
                "tick_ms": int((time.time() - start) * 1000),
                "data": result,
            })
        except Exception as exc:
            self._send({
                "code": -1,
                "message": str(exc),
                "engine": ENGINE_STATUS["engine"],
                "hqchartpy2_available": ENGINE_STATUS["hqchartpy2_available"],
                "tick_ms": int((time.time() - start) * 1000),
                "data": None,
            }, 400)

    def log_message(self, fmt, *args):
        if os.getenv("FORMULA_WORKER_LOG", "1") != "0":
            super().log_message(fmt, *args)


def main():
    host = os.getenv("FORMULA_WORKER_HOST", "127.0.0.1")
    port = int(os.getenv("FORMULA_WORKER_PORT", "8712"))
    server = ThreadingHTTPServer((host, port), WorkerHandler)
    print(f"formula-worker listening on http://{host}:{port} engine={ENGINE_STATUS['engine']}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
