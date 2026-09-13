import os
import sys
import time
from concurrent import futures
from configparser import ConfigParser
from pathlib import Path


DEFAULT_TDX_HOSTS = (
    ("124.71.187.122", 7709),
    ("218.106.92.182", 7709),
    ("220.178.55.71", 7709),
    ("117.34.114.15", 7709),
)


def configured_hosts():
    raw = os.getenv("HIKYUU_TDX_HOSTS", "").strip()
    if not raw:
        return list(DEFAULT_TDX_HOSTS)
    result = []
    for item in raw.split(","):
        host, _, port = item.strip().partition(":")
        if host:
            result.append((host, int(port or "7709")))
    return result or list(DEFAULT_TDX_HOSTS)


def search_reliable_tdx_hosts():
    from pytdx.hq import TdxHq_API

    def probe(endpoint):
        host, port = endpoint
        api = TdxHq_API(multithread=False, heartbeat=True, auto_retry=True, raise_exception=False)
        started = time.monotonic()
        try:
            if api.connect(host, port, time_out=3):
                bars = api.get_security_bars(9, 0, "159915", 0, 1)
                if bars:
                    return True, time.monotonic() - started, host, port
        except Exception:
            pass
        finally:
            try:
                api.disconnect()
            except Exception:
                pass
        return False, time.monotonic() - started, host, port

    endpoints = configured_hosts()
    with futures.ThreadPoolExecutor(max_workers=len(endpoints)) as executor:
        results = list(executor.map(probe, endpoints))
    return sorted((item for item in results if item[0]), key=lambda item: item[1])


def tdx_security_list(api, market):
    from hikyuu.data.common_pytdx import to_pytdx_market

    pytdx_market = to_pytdx_market(market)
    total = api.get_security_count(pytdx_market) or 0
    result = []
    for offset in range(0, total, 1000):
        try:
            rows = api.get_security_list(pytdx_market, offset) or []
        except Exception as exc:
            print(f"TDX security list page skipped: {market} offset={offset}: {exc}", flush=True)
            continue
        result.extend({"code": str(row["code"]), "name": str(row["name"])} for row in rows)
    return result


def install_import_fallbacks(import_module):
    from hikyuu.data.common_sqlite3 import get_codepre_list, get_marketid

    original_import_stock_name = import_module.sqlite_import_stock_name

    def import_stock_name(connect, api, market, quotations=None):
        count = original_import_stock_name(connect, api, market, quotations)
        marketid = get_marketid(connect, market)
        existing = connect.execute(
            "select count(*) from Stock where marketid=? and type<>?",
            (marketid, import_module.STOCKTYPE.INDEX),
        ).fetchone()[0]
        if count > 0 or existing > 100:
            return count

        rows = tdx_security_list(api, market)
        code_rules = get_codepre_list(connect, marketid, quotations)
        today = int(time.strftime("%Y%m%d"))
        inserted = 0
        for row in rows:
            code = row["code"]
            stock_type = next((kind for prefix, kind in code_rules if code.startswith(prefix)), None)
            if stock_type is None:
                continue
            cursor = connect.execute(
                "insert or ignore into Stock(marketid, code, name, type, valid, startDate, endDate) values (?, ?, ?, ?, 1, ?, 99999999)",
                (marketid, code, row["name"], stock_type, today),
            )
            inserted += cursor.rowcount
        connect.commit()
        print(f"TDX security list fallback: {market} inserted={inserted}", flush=True)
        return inserted

    import_module.search_best_tdx = search_reliable_tdx_hosts
    import_module.sqlite_import_stock_name = import_stock_name


def main() -> int:
    config_dir = Path(os.getenv("HIKYUU_CONFIG_DIR", "/root/.hikyuu"))
    config_file = config_dir / "importdata-gui.ini"
    if not config_file.exists():
        print(f"missing config file: {config_file}", flush=True)
        return 2

    try:
        import hikyuu.gui.data.UsePytdxImportToH5Thread as import_module
        from hikyuu.gui.data.UsePytdxImportToH5Thread import UsePytdxImportToH5Thread
    except Exception as exc:
        print(f"failed to import hikyuu: {exc}", flush=True)
        return 3

    config = ConfigParser()
    config.read(config_file, encoding="utf-8")

    install_import_fallbacks(import_module)

    runner = UsePytdxImportToH5Thread(None, config)

    def on_message(msg):
        print(msg, flush=True)

    runner.message.connect(on_message)
    runner.run()
    return 0


if __name__ == "__main__":
    sys.exit(main())
