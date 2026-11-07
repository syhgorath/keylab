#!/usr/bin/env python3
"""component_scores.py <durum-klasörü> <paket> <sunucu> → denetim puanlarını araç başına listeler (en yeni önce, her araç için son 2).

Okuduğu dosya: <durum>/<paket>/<sunucu>/**/summary.json (derinlik ≤ 5). Ortak özet biçimi:
  {"tool": "Lynis", "date": "2026-10-25T08:44:34Z", "score": 83}
Geriye dönük: `score` yoksa `hardening_index` (Lynis) kullanılır; `tool` yoksa hardening_index varlığında "Lynis", aksi halde "denetim".
Çıktı (TSV, bir satır): <araç>\t<epoch>\t<puan>. Sunucudan gelen veriye güvenilmez: yalnızca sonlu sayı (0..1000) ve [A-Za-z0-9 ._-]{1,20} araç adı kabul edilir.
"""
import json
import math
import os
import re
import sys
from datetime import datetime, timezone

TOOL_RE = re.compile(r"^[A-Za-z0-9 ._-]{1,20}$")
MAX_DEPTH = 5


def parse_epoch(value, fallback):
    if isinstance(value, str):
        try:
            return int(datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc).timestamp())
        except ValueError:
            pass
    return fallback


def number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    if not math.isfinite(value) or value < 0 or value > 1000:
        return None
    return int(value) if float(value).is_integer() else round(float(value), 1)


def collect(base):
    rows = []
    base = os.path.realpath(base)
    if not os.path.isdir(base):
        return rows
    for root, dirs, files in os.walk(base):
        depth = root[len(base):].count(os.sep)
        if depth >= MAX_DEPTH:
            dirs[:] = []
        dirs[:] = [d for d in dirs if not os.path.islink(os.path.join(root, d))]
        if "summary.json" not in files:
            continue
        path = os.path.join(root, "summary.json")
        if os.path.islink(path):
            continue
        try:
            with open(path, "r", encoding="utf-8") as fh:
                data = json.load(fh)
            mtime = int(os.path.getmtime(path))
        except (OSError, ValueError):
            continue
        if not isinstance(data, dict):
            continue
        score = number(data.get("score"))
        legacy = number(data.get("hardening_index"))
        if score is None:
            score = legacy
        if score is None:
            continue
        tool = data.get("tool")
        if not (isinstance(tool, str) and TOOL_RE.match(tool)):
            tool = "Lynis" if legacy is not None else "denetim"
        rows.append((tool, parse_epoch(data.get("date"), mtime), score))
    return rows


def main(argv):
    if len(argv) != 4:
        print("kullanım: component_scores.py <durum-klasörü> <paket> <sunucu>", file=sys.stderr)
        return 2
    state, pkg, server = argv[1:]
    if not re.match(r"^[A-Za-z0-9._-]+$", pkg) or not re.match(r"^[A-Za-z0-9._-]+$", server):
        return 2
    rows = collect(os.path.join(state, pkg, server))
    per_tool = {}
    for tool, epoch, score in sorted(rows, key=lambda r: r[1], reverse=True):
        per_tool.setdefault(tool, [])
        if len(per_tool[tool]) < 2:
            per_tool[tool].append((epoch, score))
    for tool in sorted(per_tool):
        for epoch, score in per_tool[tool]:
            print("%s\t%d\t%s" % (tool, epoch, score))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
