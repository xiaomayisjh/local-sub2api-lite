#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""跨平台桌面端构建脚本。

默认一次性产出 debug + release 两个版本，逻辑对齐 scripts/build-desktop.{sh,ps1}。

用法示例:
    python scripts/build.py                         # 当前平台, 同时构建 debug + release
    python scripts/build.py --release-only          # 仅 release
    python scripts/build.py --debug-only            # 仅 debug
    python scripts/build.py --skip-frontend         # 跳过前端构建(复用已有 dist)
    python scripts/build.py --os linux --arch amd64 # 交叉编译到 linux/amd64
"""

from __future__ import annotations

import argparse
import os
import platform
import shutil
import subprocess
import sys
from pathlib import Path

# Windows 控制台默认 GBK, 统一切到 UTF-8 避免中文日志乱码。
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")  # type: ignore[attr-defined]
    except (AttributeError, ValueError):
        pass

ROOT = Path(__file__).resolve().parent.parent
FRONTEND_DIR = ROOT / "frontend"
DESKTOP_DIR = ROOT / "desktop"
DIST_DIR = ROOT / "dist"
BASE_NAME = "ANT-Sub2API-Local"

# Go 风格 OS 名称到可执行后缀的映射(仅 windows 需要 .exe)。
EXE_SUFFIX = ".exe"


def log(msg: str) -> None:
    print(f"==> {msg}", flush=True)


def fail(msg: str) -> "None":
    print(f"[ERROR] {msg}", file=sys.stderr, flush=True)
    sys.exit(1)


def run(cmd: list[str], cwd: Path, env: dict[str, str] | None = None) -> None:
    """执行子进程命令, 失败即终止。"""
    printable = " ".join(cmd)
    log(f"$ {printable}  (cwd={cwd})")
    try:
        subprocess.run(cmd, cwd=str(cwd), env=env, check=True)
    except FileNotFoundError:
        fail(f"找不到可执行程序: {cmd[0]}, 请确认已安装并在 PATH 中。")
    except subprocess.CalledProcessError as exc:
        fail(f"命令执行失败(退出码 {exc.returncode}): {printable}")


def resolve_tool(name: str) -> str:
    """跨平台解析工具路径, Windows 下兼容 .cmd/.exe 等扩展名。"""
    found = shutil.which(name)
    if found:
        return found
    if os.name == "nt":
        for ext in (".cmd", ".exe", ".bat"):
            found = shutil.which(name + ext)
            if found:
                return found
    fail(f"找不到 `{name}`, 请先安装并确保其在 PATH 中。")
    return name  # unreachable


def host_goos() -> str:
    mapping = {"windows": "windows", "linux": "linux", "darwin": "darwin"}
    sysname = platform.system().lower()
    return mapping.get(sysname, sysname)


def host_goarch() -> str:
    machine = platform.machine().lower()
    mapping = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
        "i386": "386",
        "i686": "386",
    }
    return mapping.get(machine, machine)


def build_frontend() -> None:
    log("构建前端 (pnpm install && pnpm run build) ...")
    pnpm = resolve_tool("pnpm")
    run([pnpm, "install"], cwd=FRONTEND_DIR)
    run([pnpm, "run", "build"], cwd=FRONTEND_DIR)


def output_path(
    goos: str, goarch: str, debug: bool, is_native: bool, label: str | None
) -> Path:
    name = BASE_NAME
    if debug:
        name += "-debug"
    if label:
        # CI 用平台标识(如 windows-amd64)区分各 runner 产物。
        name += f"-{label}"
    elif not is_native:
        # 交叉编译到非本机平台时附加平台标识, 避免相互覆盖。
        name += f"-{goos}-{goarch}"
    if goos == "windows":
        name += EXE_SUFFIX
    return DIST_DIR / name


def build_binary(
    go: str,
    goos: str,
    goarch: str,
    debug: bool,
    is_native: bool,
    extra_tags: str | None,
    cgo: bool,
    label: str | None,
) -> Path:
    if debug:
        tags = "production,debug,embed"
        ldflags = ""  # 保留符号便于栈追踪; 不加 -H windowsgui, 保留控制台。
    else:
        tags = "production,embed"
        ldflags = "-s -w"
        if goos == "windows":
            ldflags += " -H windowsgui"
    if extra_tags:
        tags += "," + extra_tags.strip(",")

    out = output_path(goos, goarch, debug, is_native, label)
    cgo_enabled = "1" if cgo else "0"
    log(f"构建 {'DEBUG' if debug else 'RELEASE'} -> {out.name}")
    log(
        f"    GOOS={goos} GOARCH={goarch} CGO_ENABLED={cgo_enabled} "
        f"tags={tags} ldflags={ldflags or '(none)'}"
    )

    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    # Windows 用纯 Go(go-webview2 + modernc sqlite) 可关 CGO;
    # Linux/macOS 的 Wails 需 CGO(webkit/cocoa), 由 --cgo 开启。
    env["CGO_ENABLED"] = cgo_enabled

    cmd = [go, "build", "-tags", tags]
    if ldflags:
        cmd += ["-ldflags", ldflags]
    cmd += ["-o", str(out), "."]
    run(cmd, cwd=DESKTOP_DIR, env=env)
    # 校验产物确实生成且非空, 避免 go build 静默产出空文件等异常。
    if not out.exists() or out.stat().st_size == 0:
        fail(f"构建未产出有效文件: {out}")
    return out


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="跨平台构建 local-sub2api-lite 桌面端(默认同时构建 debug + release)。"
    )
    group = p.add_mutually_exclusive_group()
    group.add_argument("--release-only", action="store_true", help="仅构建 release 版本")
    group.add_argument("--debug-only", action="store_true", help="仅构建 debug 版本")
    p.add_argument("--skip-frontend", action="store_true", help="跳过前端构建, 复用现有 dist")
    p.add_argument("--os", dest="goos", default=None, help="目标 GOOS(默认当前平台), 如 windows/linux/darwin")
    p.add_argument("--arch", dest="goarch", default=None, help="目标 GOARCH(默认当前平台), 如 amd64/arm64")
    p.add_argument("--cgo", action="store_true", help="开启 CGO(Linux/macOS 的 Wails 桌面端必需)")
    p.add_argument("--extra-tags", default=None, help="追加 go build tags(逗号分隔), 如 webkit2_41")
    p.add_argument("--label", default=None, help="产物文件名平台标识(CI 用), 如 windows-amd64")
    return p.parse_args()


def main() -> None:
    args = parse_args()

    goos = args.goos or host_goos()
    goarch = args.goarch or host_goarch()
    is_native = (goos == host_goos()) and (goarch == host_goarch())

    targets: list[bool] = []  # True=debug, False=release
    if args.release_only:
        targets = [False]
    elif args.debug_only:
        targets = [True]
    else:
        targets = [False, True]

    go = resolve_tool("go")

    log(f"目标平台: {goos}/{goarch} ({'本机' if is_native else '交叉编译'})")
    log(f"构建目标: {', '.join('debug' if d else 'release' for d in targets)}")

    DIST_DIR.mkdir(parents=True, exist_ok=True)

    if args.skip_frontend:
        log("跳过前端构建 (--skip-frontend)")
    else:
        build_frontend()

    log("同步 desktop 模块依赖 (go mod tidy) ...")
    run([go, "mod", "tidy"], cwd=DESKTOP_DIR)

    outputs: list[Path] = []
    for debug in targets:
        outputs.append(
            build_binary(
                go, goos, goarch, debug, is_native, args.extra_tags, args.cgo, args.label
            )
        )

    log("构建完成:")
    for out in outputs:
        size_mb = out.stat().st_size / (1024 * 1024) if out.exists() else 0
        print(f"    - {out}  ({size_mb:.1f} MB)")

    if True in targets:  # 构建了 debug 版本
        print("    Debug 版本: DevTools 与控制台已启用, 从终端运行以查看服务日志。")


if __name__ == "__main__":
    main()
