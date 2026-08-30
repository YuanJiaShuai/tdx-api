import os
import sys
from configparser import ConfigParser
from pathlib import Path


def main() -> int:
    config_dir = Path(os.getenv("HIKYUU_CONFIG_DIR", "/root/.hikyuu"))
    config_file = config_dir / "importdata-gui.ini"
    if not config_file.exists():
        print(f"missing config file: {config_file}", flush=True)
        return 2

    try:
        from hikyuu.gui.data.UsePytdxImportToH5Thread import UsePytdxImportToH5Thread
    except Exception as exc:
        print(f"failed to import hikyuu: {exc}", flush=True)
        return 3

    config = ConfigParser()
    config.read(config_file, encoding="utf-8")

    runner = UsePytdxImportToH5Thread(None, config)

    def on_message(msg):
        print(msg, flush=True)

    runner.message.connect(on_message)
    runner.run()
    return 0


if __name__ == "__main__":
    sys.exit(main())
