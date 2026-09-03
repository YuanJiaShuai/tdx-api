import copy
import ast
import json
import os
import subprocess
import threading
import time
import uuid
import hashlib
import sqlite3
from configparser import ConfigParser
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from fastapi import FastAPI, HTTPException, Query
from pydantic import BaseModel, Field
from zoneinfo import ZoneInfo


APP_NAME = "hikyuu-data-service"
TZ_NAME = os.getenv("TZ", "Asia/Shanghai")
TZ = ZoneInfo(TZ_NAME)
STOCKS_DIR = Path(os.getenv("HIKYUU_STOCKS_DIR", "/root/stocks"))
CONFIG_DIR = Path(os.getenv("HIKYUU_CONFIG_DIR", "/root/.hikyuu"))
LOG_DIR = Path(os.getenv("HIKYUU_LOG_DIR", "/app/logs"))
PYTHON_BIN = os.getenv("HIKYUU_PYTHON", "python3")
IMPORT_SCRIPT = os.getenv("HIKYUU_IMPORT_SCRIPT", "/app/importdata_runner.py")
QUERY_SCRIPT = os.getenv("HIKYUU_QUERY_SCRIPT", "/app/query_runner.py")
QUERY_TIMEOUT_SECONDS = int(os.getenv("HIKYUU_QUERY_TIMEOUT_SECONDS", "120"))
SCHEDULER_ENABLED = os.getenv("HIKYUU_SCHEDULER_ENABLED", "true").lower() in ("1", "true", "yes", "on")
AFTER_CLOSE_CRON = os.getenv("HIKYUU_AFTER_CLOSE_CRON", "30 16 * * 1-5")
MAX_TASKS = int(os.getenv("HIKYUU_MAX_TASKS", "200"))
DATA_REVISION_FILE = STOCKS_DIR / ".data-revision"
INDICATOR_CACHE_SECONDS = int(os.getenv("HIKYUU_INDICATOR_CACHE_SECONDS", "60"))
indicator_cache: Dict[str, Any] = {}
indicator_cache_lock = threading.Lock()


class SyncRequest(BaseModel):
    day: bool = True
    min: bool = True
    min5: bool = True
    trans: bool = False
    time: bool = False
    stock: bool = True
    fund: bool = True
    weight: bool = True
    finance: bool = True
    block: bool = True
    day_start_date: Optional[str] = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    min_start_date: Optional[str] = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    min5_start_date: Optional[str] = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    trans_start_date: Optional[str] = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    time_start_date: Optional[str] = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    use_tdx_number: int = Field(default=10, ge=1, le=32)


class TaskRecord(BaseModel):
    id: str
    type: str
    status: str
    started_at: str
    ended_at: Optional[str] = None
    exit_code: Optional[int] = None
    error: Optional[str] = None
    log_file: str
    request: Dict[str, Any]


app = FastAPI(title=APP_NAME)
tasks: Dict[str, Dict[str, Any]] = {}
task_order: List[str] = []
task_lock = threading.Lock()
active_task_id: Optional[str] = None
scheduler: Optional[BackgroundScheduler] = None


def now_text() -> str:
    return datetime.now(TZ).isoformat()


def bool_text(value: bool) -> str:
    return "True" if value else "False"


def ensure_dirs() -> None:
    STOCKS_DIR.mkdir(parents=True, exist_ok=True)
    (STOCKS_DIR / "tmp").mkdir(parents=True, exist_ok=True)
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    LOG_DIR.mkdir(parents=True, exist_ok=True)


def hikyuu_version() -> str:
    try:
        from importlib.metadata import version
        return version("hikyuu")
    except Exception:
        return "unknown"


def data_revision() -> str:
    if DATA_REVISION_FILE.exists():
        try:
            value = DATA_REVISION_FILE.read_text(encoding="utf-8").strip()
            if value:
                return value
        except OSError:
            pass
    try:
        latest = max((item.stat().st_mtime_ns for item in STOCKS_DIR.glob("*")), default=0)
    except OSError:
        latest = 0
    return hashlib.sha1(f"{STOCKS_DIR}:{latest}".encode()).hexdigest()[:12]


def dataset_metadata() -> Dict[str, Any]:
    files: List[Dict[str, Any]] = []
    try:
        for item in sorted(STOCKS_DIR.glob("*.h5")) + sorted(STOCKS_DIR.glob("*.db")):
            stat = item.stat()
            files.append({"name": item.name, "bytes": stat.st_size, "modified_at": datetime.fromtimestamp(stat.st_mtime, TZ).isoformat()})
    except OSError:
        pass
    symbols = None
    stock_db = STOCKS_DIR / "stock.db"
    if stock_db.exists():
        try:
            with sqlite3.connect(stock_db, timeout=2) as conn:
                for table in ("stock", "stock_basic", "stocks"):
                    try:
                        symbols = int(conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0])
                        break
                    except sqlite3.Error:
                        continue
        except sqlite3.Error:
            symbols = None
    return {"data_revision": data_revision(), "stocks": {"path": str(STOCKS_DIR), "symbols": symbols, "files": files}, "hikyuu_version": hikyuu_version(), "checked_at": now_text()}


def dataset_quality(code: str = "", period: str = "day") -> Dict[str, Any]:
    metadata = dataset_metadata()
    files = metadata["stocks"]["files"]
    h5_files = [item for item in files if str(item["name"]).endswith(".h5")]
    total_bytes = sum(int(item["bytes"]) for item in files)
    checks = [
        {"id": "directory", "label": "数据目录", "status": "pass" if STOCKS_DIR.exists() else "fail", "detail": str(STOCKS_DIR)},
        {"id": "hdf5", "label": "HDF5 文件", "status": "pass" if h5_files else "warn", "detail": f"{len(h5_files)} 个文件"},
        {"id": "stock_db", "label": "基础信息库", "status": "pass" if (STOCKS_DIR / "stock.db").exists() else "warn", "detail": f"{metadata['stocks']['symbols'] or 0} 个证券"},
        {"id": "size", "label": "数据体积", "status": "pass" if total_bytes > 0 else "warn", "detail": total_bytes},
        {"id": "revision", "label": "修订标识", "status": "pass" if metadata["data_revision"] else "warn", "detail": metadata["data_revision"]},
    ]
    sample = None
    if code:
        try:
            from query_runner import load_records
            sample = load_records(code, period, "", "", 2000, "none")["list"]
            times = [str(row.get("time")) for row in sample]
            closes = [float(row.get("close") or 0) for row in sample]
            duplicate_count = len(times) - len(set(times))
            invalid_count = sum(1 for row in sample if float(row.get("high") or 0) < float(row.get("low") or 0) or float(row.get("close") or 0) <= 0)
            checks.extend([
                {"id": "sample_count", "label": "样本记录", "status": "pass" if sample else "warn", "detail": len(sample)},
                {"id": "duplicates", "label": "重复时间", "status": "pass" if duplicate_count == 0 else "fail", "detail": duplicate_count},
                {"id": "ohlc", "label": "OHLC 合法性", "status": "pass" if invalid_count == 0 else "fail", "detail": invalid_count},
                {"id": "monotonic", "label": "时间序列", "status": "pass" if times == sorted(times) and len(closes) == len(sample) else "warn", "detail": "升序" if times == sorted(times) else "需检查"},
            ])
        except Exception as exc:
            checks.append({"id": "sample_count", "label": "样本记录", "status": "warn", "detail": str(exc)})
    status = "pass" if all(item["status"] == "pass" for item in checks) else "warn"
    return {"status": status, "checks": checks, "data_revision": metadata["data_revision"], "checked_at": now_text()}


def write_hikyuu_ini() -> None:
    ensure_dirs()
    datadir = str(STOCKS_DIR)
    content = f"""
[hikyuu]
tmpdir = {datadir}/tmp
datadir = {datadir}
reload_time = 00:00
quotation_server = ipc:///tmp/hikyuu_real.ipc
lazy_preload = False

[block]
type = sqlite3
db = {datadir}/stock.db

[preload]
day = True
week = True
month = True
quarter = False
halfyear = False
year = False
min = False
min5 = False
min15 = False
min30 = False
min60 = False
hour2 = False
timeline = False
trans = False
day_max = 100000
week_max = 100000
month_max = 100000
quarter_max = 100000
halfyear_max = 100000
year_max = 100000
min_max = 100000
min5_max = 100000
min15_max = 100000
min30_max = 100000
min60_max = 100000
hour2_max = 100000
timeline_max = 100000
trans_max = 100000

[baseinfo]
type = sqlite3
db = {datadir}/stock.db

[kdata]
type = hdf5
sh_day = {datadir}/sh_day.h5
sh_min = {datadir}/sh_1min.h5
sh_min5 = {datadir}/sh_5min.h5
sz_day = {datadir}/sz_day.h5
sz_min = {datadir}/sz_1min.h5
sz_min5 = {datadir}/sz_5min.h5
bj_day = {datadir}/bj_day.h5
bj_min = {datadir}/bj_1min.h5
bj_min5 = {datadir}/bj_5min.h5
sh_time = {datadir}/sh_time.h5
sz_time = {datadir}/sz_time.h5
bj_time = {datadir}/bj_time.h5
sh_trans = {datadir}/sh_trans.h5
sz_trans = {datadir}/sz_trans.h5
bj_trans = {datadir}/bj_trans.h5
""".strip()
    (CONFIG_DIR / "hikyuu.ini").write_text(content + "\n", encoding="utf-8")


def build_import_config(request: SyncRequest) -> None:
    ensure_dirs()
    parser = ConfigParser()
    parser["quotation"] = {
        "stock": bool_text(request.stock),
        "fund": bool_text(request.fund),
        "future": "False",
    }
    parser["ktype"] = {
        "day": bool_text(request.day),
        "min": bool_text(request.min),
        "min5": bool_text(request.min5),
        "trans": bool_text(request.trans),
        "time": bool_text(request.time),
        "day_start_date": request.day_start_date or "1990-12-19",
        "min_start_date": request.min_start_date or "2023-09-19",
        "min5_start_date": request.min5_start_date or "2023-09-19",
        "trans_start_date": request.trans_start_date or "2023-12-11",
        "time_start_date": request.time_start_date or "2023-12-11",
    }
    parser["weight"] = {"enable": bool_text(request.weight)}
    parser["finance"] = {"enable": bool_text(request.finance)}
    parser["block"] = {"enable": bool_text(request.block)}
    parser["tdx"] = {"enable": "False"}
    parser["pytdx"] = {"enable": "True", "use_tdx_number": str(request.use_tdx_number)}
    parser["hdf5"] = {"enable": "True", "dir": str(STOCKS_DIR)}
    parser["mysql"] = {"enable": "False"}
    parser["clickhouse"] = {"enable": "False"}
    parser["sched"] = {"time": "16:30:00"}
    parser["collect"] = {
        "quotation_server": "ipc:///tmp/hikyuu_real.ipc",
        "interval": "305",
        "source": "qq",
        "use_zhima_proxy": "False",
        "phase1_start": "00:00:00",
        "phase1_end": "11:35:00",
        "phase2_start": "12:00:00",
        "phase2_end": "15:05:00",
    }
    parser["lazy_preload"] = {"enable": "False"}
    parser["preload"] = {
        "day": "True",
        "week": "True",
        "month": "True",
        "quarter": "False",
        "halfyear": "False",
        "year": "False",
        "min": "False",
        "min5": "False",
        "min15": "False",
        "min30": "False",
        "min60": "False",
        "hour2": "False",
        "timeline": "False",
        "trans": "False",
        "day_max": "100000",
        "week_max": "100000",
        "month_max": "100000",
        "quarter_max": "100000",
        "halfyear_max": "100000",
        "year_max": "100000",
        "min_max": "100000",
        "min5_max": "100000",
        "min15_max": "100000",
        "min30_max": "100000",
        "min60_max": "100000",
        "hour2_max": "100000",
        "timeline_max": "100000",
        "trans_max": "100000",
    }
    with (CONFIG_DIR / "importdata-gui.ini").open("w", encoding="utf-8") as fh:
        parser.write(fh)
    write_hikyuu_ini()


def trim_tasks() -> None:
    while len(task_order) > MAX_TASKS:
        old_id = task_order.pop(0)
        tasks.pop(old_id, None)


def request_dict(request: SyncRequest) -> Dict[str, Any]:
    if hasattr(request, "model_dump"):
        return request.model_dump()
    return request.dict()


def task_snapshot(task: Dict[str, Any]) -> Dict[str, Any]:
    item = copy.deepcopy(task)
    log_file = Path(item["log_file"])
    if log_file.exists():
        try:
            lines = log_file.read_text(encoding="utf-8", errors="replace").splitlines()
            item["log_tail"] = lines[-80:]
            item.update(parse_task_progress(item["log_tail"]))
        except OSError:
            item["log_tail"] = []
            item.update({"progress": None, "stage": None, "message": None})
    else:
        item["log_tail"] = []
        item.update({"progress": None, "stage": None, "message": None})
    return item


def parse_task_progress(lines: List[str]) -> Dict[str, Any]:
    result: Dict[str, Any] = {"progress": None, "stage": None, "message": None}
    for raw_line in reversed(lines):
        text = raw_line.strip()
        if not text:
            continue
        if text.startswith("["):
            try:
                event = ast.literal_eval(text)
            except (ValueError, SyntaxError):
                event = None
            if isinstance(event, list) and event and event[0] == "HDF5_IMPORT":
                kind = str(event[1]) if len(event) > 1 else ""
                if kind == "THREAD":
                    status = str(event[2]) if len(event) > 2 else ""
                    if status == "FINISHED":
                        return {"progress": 100, "stage": "完成", "message": "同步完成"}
                    if status == "FAILURE":
                        message = str(event[3]) if len(event) > 3 else "同步失败"
                        return {"progress": 100, "stage": "失败", "message": message}
                if kind in {"IMPORT_KDATA", "IMPORT_TRANS", "IMPORT_TIME"}:
                    sub_stage = str(event[2]) if len(event) > 2 else kind
                    progress = event[3] if len(event) > 3 else None
                    if isinstance(progress, (int, float)):
                        return {
                            "progress": max(0, min(100, int(progress))),
                            "stage": sub_stage,
                            "message": f"{sub_stage} {int(progress)}%",
                        }
                    return {"progress": None, "stage": sub_stage, "message": sub_stage}
                if kind == "IMPORT_FINANCE":
                    progress = event[2] if len(event) > 2 else None
                    if isinstance(progress, (int, float)):
                        return {
                            "progress": max(0, min(100, int(progress))),
                            "stage": "财务",
                            "message": f"财务 {int(progress)}%",
                        }
                    return {"progress": None, "stage": "财务", "message": "财务同步中"}
                if kind == "IMPORT_WEIGHT":
                    label = str(event[1]) if len(event) > 1 else "权息"
                    total = event[2] if len(event) > 2 else None
                    message = f"{label} 同步中"
                    if total is not None:
                        message = f"{label} {total}"
                    return {"progress": None, "stage": label, "message": message}
                if kind == "IMPORT_BLOCKINFO":
                    label = str(event[2]) if len(event) > 2 else "板块"
                    return {"progress": None, "stage": label, "message": label}
                if kind == "INFO":
                    message = str(event[2]) if len(event) > 2 else "同步中"
                    return {"progress": None, "stage": "信息", "message": message}
        result = {"progress": None, "stage": "信息", "message": text}
        if "导入完毕" in text:
            return {"progress": 100, "stage": "完成", "message": "同步完成"}
    return result


def run_task(task_id: str) -> None:
    global active_task_id
    with task_lock:
        task = tasks[task_id]
        task["status"] = "running"

    request = SyncRequest(**task["request"])
    log_path = Path(task["log_file"])
    env = os.environ.copy()
    env["HOME"] = str(CONFIG_DIR.parent if CONFIG_DIR.name == ".hikyuu" else Path("/root"))
    env["HIKYUU_STOCKS_DIR"] = str(STOCKS_DIR)
    env["HIKYUU_CONFIG_DIR"] = str(CONFIG_DIR)
    env["TZ"] = TZ_NAME

    try:
        build_import_config(request)
        with log_path.open("a", encoding="utf-8") as log:
            log.write(f"[{now_text()}] task {task_id} started: {task['type']}\n")
            log.flush()
            proc = subprocess.Popen(
                [PYTHON_BIN, IMPORT_SCRIPT],
                stdout=log,
                stderr=subprocess.STDOUT,
                env=env,
                text=True,
            )
            exit_code = proc.wait()
            with task_lock:
                task["exit_code"] = exit_code
                task["ended_at"] = now_text()
                if exit_code == 0:
                    task["status"] = "success"
                    try:
                        DATA_REVISION_FILE.write_text(f"{uuid.uuid4().hex[:12]}\n", encoding="utf-8")
                    except OSError:
                        pass
                else:
                    task["status"] = "failed"
                    task["error"] = f"import process exited with code {exit_code}"
            log.write(f"[{now_text()}] task {task_id} ended: exit_code={exit_code}\n")
    except Exception as exc:
        with task_lock:
            task["status"] = "failed"
            task["error"] = str(exc)
            task["ended_at"] = now_text()
    finally:
        with task_lock:
            if active_task_id == task_id:
                active_task_id = None


def create_task(task_type: str, request: SyncRequest) -> Dict[str, Any]:
    global active_task_id
    with task_lock:
        if active_task_id is not None:
            raise HTTPException(status_code=409, detail=f"task already running: {active_task_id}")

        task_id = str(uuid.uuid4())
        log_file = LOG_DIR / f"{task_id}.log"
        task = {
            "id": task_id,
            "type": task_type,
            "status": "pending",
            "started_at": now_text(),
            "ended_at": None,
            "exit_code": None,
            "error": None,
            "log_file": str(log_file),
            "request": request_dict(request),
        }
        tasks[task_id] = task
        task_order.append(task_id)
        active_task_id = task_id
        trim_tasks()

    thread = threading.Thread(target=run_task, args=(task_id,), daemon=True)
    thread.start()
    return task


def scheduled_after_close_sync() -> None:
    request = SyncRequest()
    try:
        create_task("after_close_sync", request)
    except HTTPException:
        return


@app.on_event("startup")
def startup() -> None:
    global scheduler
    ensure_dirs()
    write_hikyuu_ini()
    if SCHEDULER_ENABLED:
        scheduler = BackgroundScheduler(timezone=TZ)
        scheduler.add_job(
            scheduled_after_close_sync,
            CronTrigger.from_crontab(AFTER_CLOSE_CRON, timezone=TZ),
            id="after-close-sync",
            name="after-close-sync",
            replace_existing=True,
        )
        scheduler.start()


@app.on_event("shutdown")
def shutdown() -> None:
    if scheduler is not None:
        scheduler.shutdown(wait=False)


@app.get("/api/hikyuu/kline")
def query_kline(
    code: str = Query(..., min_length=1),
    kline_type: str = Query("day", alias="type"),
    start: str = "",
    end: str = "",
    limit: int = Query(0, ge=0, le=100000),
    recover: str = "none",
) -> Dict[str, Any]:
    ensure_dirs()
    if not (CONFIG_DIR / "hikyuu.ini").exists():
        write_hikyuu_ini()
    try:
        from query_runner import load_records
        data = load_records(code.strip(), kline_type.strip().lower() or "day", start.strip(), end.strip(), limit, recover.strip().lower() or "none")
        return {"code": 0, "message": "success", "data": data}
    except Exception as in_process_error:
        # Keep the subprocess path as an isolation fallback for incompatible
        # Hikyuu builds or a damaged native runtime.
        fallback_error = in_process_error
    env = os.environ.copy()
    env["HOME"] = str(CONFIG_DIR.parent if CONFIG_DIR.name == ".hikyuu" else Path("/root"))
    env["HIKYUU_STOCKS_DIR"] = str(STOCKS_DIR)
    env["HIKYUU_CONFIG_DIR"] = str(CONFIG_DIR)
    env["TZ"] = TZ_NAME

    command = [
        PYTHON_BIN,
        QUERY_SCRIPT,
        "--symbol",
        code.strip(),
        "--period",
        kline_type.strip().lower() or "day",
        "--start",
        start.strip(),
        "--end",
        end.strip(),
        "--limit",
        str(limit),
        "--recover",
        recover.strip().lower() or "none",
    ]
    try:
        completed = subprocess.run(
            command,
            capture_output=True,
            text=True,
            env=env,
            timeout=QUERY_TIMEOUT_SECONDS,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise HTTPException(status_code=504, detail="hikyuu 查询超时") from exc
    except OSError as exc:
        raise HTTPException(status_code=503, detail=f"hikyuu 查询进程启动失败: {exc}") from exc

    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip() or str(fallback_error) or "hikyuu 查询失败"
        raise HTTPException(status_code=503, detail=detail[-1000:])

    output_lines = [line.strip() for line in completed.stdout.splitlines() if line.strip()]
    if not output_lines:
        raise HTTPException(status_code=503, detail="hikyuu 查询没有返回数据")
    try:
        data = json.loads(output_lines[-1])
    except json.JSONDecodeError as exc:
        raise HTTPException(status_code=503, detail="hikyuu 查询返回格式错误") from exc

    return {"code": 0, "message": "success", "data": data}


@app.get("/api/hikyuu/metadata")
def metadata() -> Dict[str, Any]:
    ensure_dirs()
    return {"code": 0, "message": "success", "data": dataset_metadata()}


@app.get("/api/hikyuu/quality")
def quality(code: str = Query(""), period: str = Query("day")) -> Dict[str, Any]:
    ensure_dirs()
    return {"code": 0, "message": "success", "data": dataset_quality(code.strip(), period.strip().lower() or "day")}


@app.post("/api/hikyuu/indicators")
def indicators(payload: Dict[str, Any]) -> Dict[str, Any]:
    cache_key = json.dumps(payload, sort_keys=True, ensure_ascii=False, default=str)
    current_time = time.monotonic()
    with indicator_cache_lock:
        cached = indicator_cache.get(cache_key)
        if cached and current_time - cached["created_at"] < INDICATOR_CACHE_SECONDS:
            return {"code": 0, "message": "success", "data": cached["data"]}
    try:
        from research_runner import calculate_indicator
        result = calculate_indicator(payload)
    except Exception as exc:
        raise HTTPException(status_code=503, detail=f"指标计算失败: {exc}") from exc
    with indicator_cache_lock:
        indicator_cache[cache_key] = {"created_at": current_time, "data": result}
        if len(indicator_cache) > 128:
            oldest = min(indicator_cache, key=lambda key: indicator_cache[key]["created_at"])
            indicator_cache.pop(oldest, None)
    return {"code": 0, "message": "success", "data": result}


@app.post("/api/hikyuu/backtest")
def backtest(payload: Dict[str, Any]) -> Dict[str, Any]:
    try:
        from research_runner import run_reference_backtest
        result = run_reference_backtest(payload)
    except Exception as exc:
        raise HTTPException(status_code=503, detail=f"Hikyuu 回测失败: {exc}") from exc
    return {"code": 0, "message": "success", "data": result}


@app.get("/api/hikyuu/health")
def health() -> Dict[str, Any]:
    return {
        "code": 0,
        "message": "success",
        "data": {
            "service": APP_NAME,
            "time": now_text(),
            "stocks_dir": str(STOCKS_DIR),
            "config_dir": str(CONFIG_DIR),
            "log_dir": str(LOG_DIR),
            "scheduler_enabled": SCHEDULER_ENABLED,
            "after_close_cron": AFTER_CLOSE_CRON,
            "active_task_id": active_task_id,
            "hikyuu_version": hikyuu_version(),
            "data_revision": data_revision(),
            "quality_status": dataset_quality()["status"],
        },
    }


@app.post("/api/hikyuu/tasks/full-sync")
def full_sync(request: SyncRequest = SyncRequest()) -> Dict[str, Any]:
    task = create_task("full_sync", request)
    return {"code": 0, "message": "success", "data": task_snapshot(task)}


@app.post("/api/hikyuu/tasks/after-close-sync")
def after_close_sync(request: SyncRequest = SyncRequest()) -> Dict[str, Any]:
    task = create_task("after_close_sync", request)
    return {"code": 0, "message": "success", "data": task_snapshot(task)}


@app.get("/api/hikyuu/tasks")
def list_tasks() -> Dict[str, Any]:
    with task_lock:
        items = [task_snapshot(tasks[task_id]) for task_id in reversed(task_order)]
    return {"code": 0, "message": "success", "data": items}


@app.get("/api/hikyuu/tasks/{task_id}")
def get_task(task_id: str) -> Dict[str, Any]:
    with task_lock:
        task = tasks.get(task_id)
        if task is None:
            raise HTTPException(status_code=404, detail="task not found")
        item = task_snapshot(task)
    return {"code": 0, "message": "success", "data": item}


@app.get("/api/hikyuu/config")
def get_config() -> Dict[str, Any]:
    paths = {
        "hikyuu_ini": str(CONFIG_DIR / "hikyuu.ini"),
        "importdata_gui_ini": str(CONFIG_DIR / "importdata-gui.ini"),
    }
    return {"code": 0, "message": "success", "data": paths}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", "8091")))
