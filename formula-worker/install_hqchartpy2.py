#!/usr/bin/env python3
import os
import shutil
import subprocess
import sys
import sysconfig
from pathlib import Path


def main():
    root = Path(__file__).resolve().parent
    src = root / "hqchartpy2-src"
    module_dir = src / "hqchart.py2.free"
    if not module_dir.exists():
        print("HQChartPy2 source not found, skip")
        return 0

    include_dir = sysconfig.get_paths().get("include")
    sources = [
        *module_dir.glob("*.cpp"),
        *(src / "HQChart.Complier.Source").glob("*.cpp"),
        *(src / "HQChart.Complier.ToJson").glob("*.cpp"),
    ]
    cmd = [
        "g++",
        "-shared",
        "-std=c++11",
        "-fPIC",
        "-finput-charset=GBK",
        "-D_NOMNG",
        "-D_FILELINE",
        "-D__U32_TYPE=unsigned",
        f"-I{include_dir}",
        f"-I{module_dir}",
        f"-I{src / 'rapidjson'}",
        f"-I{src / 'HQChart.Complier.Source'}",
        f"-I{src / 'HQChart.Complier.ToJson'}",
        "-o",
        str(root / "HQChartPy2.so"),
        *[str(item) for item in sources],
        "-lpthread",
        "-lz",
    ]
    print("building HQChartPy2 extension")
    subprocess.check_call(cmd)

    site_packages = Path(sysconfig.get_paths()["purelib"])
    target = site_packages / "HQChartPy2.so"
    shutil.copy2(root / "HQChartPy2.so", target)
    print(f"installed {target}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
