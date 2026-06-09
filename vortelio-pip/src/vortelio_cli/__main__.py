"""Launcher: locate bundled vortelio binary, exec with passed args."""
from __future__ import annotations

import os
import platform
import shutil
import stat
import subprocess
import sys
import urllib.request
from pathlib import Path

VERSION = "0.3.60"
RELEASE_BASE = os.environ.get(
    "VORTELIO_RELEASE_BASE",
    f"https://github.com/metiu1/Vortelio/releases/download/v{VERSION}",
)

BIN_DIR = Path(__file__).parent / "bin"


def _binary_name() -> str:
    sysname = platform.system().lower()
    arch = platform.machine().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(arch, arch)
    if sysname.startswith("win"):
        return f"vortelio-windows-{arch}.exe"
    if sysname == "darwin":
        return f"vortelio-darwin-{arch}"
    return f"vortelio-linux-{arch}"


def _cache_dir() -> Path:
    base = os.environ.get("XDG_CACHE_HOME") or (Path.home() / ".cache")
    d = Path(base) / "vortelio" / VERSION
    d.mkdir(parents=True, exist_ok=True)
    return d


def _download(url: str, dest: Path) -> None:
    tmp = dest.with_suffix(dest.suffix + ".part")
    print(f"vortelio: downloading {url}", file=sys.stderr)
    with urllib.request.urlopen(url) as r, open(tmp, "wb") as f:
        shutil.copyfileobj(r, f)
    tmp.replace(dest)
    if os.name != "nt":
        dest.chmod(dest.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def _resolve_binary() -> Path:
    name = _binary_name()
    bundled = BIN_DIR / name
    if bundled.exists():
        return bundled
    cached = _cache_dir() / name
    if cached.exists():
        return cached
    url = f"{RELEASE_BASE}/{name}"
    try:
        _download(url, cached)
    except Exception as e:
        sys.stderr.write(
            f"vortelio: no bundled binary for this platform ({name}) and "
            f"download failed: {e}\nSet VORTELIO_RELEASE_BASE or place a binary at {bundled}\n"
        )
        sys.exit(1)
    return cached


def main() -> int:
    binary = _resolve_binary()
    if os.name == "nt":
        return subprocess.call([str(binary), *sys.argv[1:]])
    os.execv(str(binary), [str(binary), *sys.argv[1:]])
    return 0  # unreachable


if __name__ == "__main__":
    sys.exit(main())
