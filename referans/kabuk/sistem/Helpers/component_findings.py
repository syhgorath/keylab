#!/usr/bin/env python3
"""component_findings.py <durum-klasörü> <paket> <sunucu> <dosya-adı> → denetim bulgularını listeler (önem sırasıyla).

Okuduğu dosya: <durum>/<paket>/<sunucu>/**/<dosya-adı> (derinlik ≤ 5; birden çoksa en yeni). Biçim: docs/package-standard.md "Bulgular".
Başlık satırının ilk 10 sütunu tam olarak şu olmalı (sonda fazladan sütun yok sayılır):
  kimlik risk tur aciklama ayrinti oneri alan deger kapanir_varsayilan platform
Çıktı: sütunlar \x1f (birim ayırıcı) ile ayrılır (sekme boşluk sayıldığından kabuk `read` boş sütunları yutardı). İlk satır
`#\x1f<epoch>\x1f<toplam>\x1f<atlanan>\x1f<kesildi 0|1>\x1f<eşlemeli>\x1f<dosya-yolu>`, ardından en çok 2000 bulgu satırı (10 sütun).
Sıralama: risk (yuksek, orta, dusuk, bilgi, bilinmiyor), sonra dosya sırası. Dosya yoksa çıktı boş (dönüş 0); başlık bozuksa dönüş 3.
Sunucudan gelen veriye güvenilmez: geçersiz satır atlanır (sayılır), denetim karakterleri boşluğa çevrilir, alanlar 300 karakterle kesilir.
"""
import os
import re
import sys

COLUMNS = ["kimlik", "risk", "tur", "aciklama", "ayrinti", "oneri", "alan", "deger", "kapanir_varsayilan", "platform"]
RISKS = ["yuksek", "orta", "dusuk", "bilgi", "bilinmiyor"]
MAX_DEPTH = 5
MAX_ROWS = 2000
MAX_FIELD = 300
MAX_BYTES = 4 * 1024 * 1024
ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}$")
AREA_RE = re.compile(r"^[a-z][a-z0-9_]{0,39}$")
VALUE_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]{0,63}=[^\t\r\n]{0,200}$")
CTRL_RE = re.compile(r"[\x00-\x1f\x7f]")
FILE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,59}\.tsv$")


def find_newest(base, filename):
    best = None
    base = os.path.realpath(base)
    if not os.path.isdir(base):
        return None
    for root, dirs, files in os.walk(base):
        if root[len(base):].count(os.sep) >= MAX_DEPTH:
            dirs[:] = []
        dirs[:] = [d for d in dirs if not os.path.islink(os.path.join(root, d))]
        if filename not in files:
            continue
        path = os.path.join(root, filename)
        if os.path.islink(path):
            continue
        try:
            mtime = os.path.getmtime(path)
        except OSError:
            continue
        if best is None or mtime > best[0]:
            best = (mtime, path)
    return best


def clean(value):
    return CTRL_RE.sub(" ", value).strip()[:MAX_FIELD]


def valid(row):
    kimlik, risk, tur, _, _, _, alan, deger, kapanir, platform = row
    return (bool(ID_RE.match(kimlik)) and risk in RISKS and tur in ("uyari", "oneri")
            and platform in ("ubuntu", "proxmox", "hepsi")
            and kapanir in ("", "evet", "hayir")
            and (alan == "" or bool(AREA_RE.match(alan)))
            and (deger == "" or bool(VALUE_RE.match(deger))))


def main(argv):
    if len(argv) != 5:
        print("kullanım: component_findings.py <durum-klasörü> <paket> <sunucu> <dosya-adı>", file=sys.stderr)
        return 2
    state, pkg, server, fname = argv[1:]
    name_re = re.compile(r"^[A-Za-z0-9._-]+$")
    if not (name_re.match(pkg) and name_re.match(server) and FILE_RE.match(fname)) or ".." in fname:
        return 2
    found = find_newest(os.path.join(state, pkg, server), fname)
    if found is None:
        return 0
    mtime, path = found
    try:
        with open(path, "rb") as fh:
            raw = fh.read(MAX_BYTES + 1)
        text = raw[:MAX_BYTES].decode("utf-8", errors="replace")
    except OSError:
        return 0
    lines = text.splitlines()
    if not lines or [c.strip() for c in lines[0].split("\t")[:len(COLUMNS)]] != COLUMNS:
        return 3
    rows, skipped = [], 0
    for line in lines[1:]:
        if not line.strip():
            continue
        cols = line.split("\t")
        if len(cols) < len(COLUMNS):
            skipped += 1
            continue
        row = [clean(c) for c in cols[:len(COLUMNS)]]
        if not valid(row):
            skipped += 1
            continue
        rows.append(row)
    truncated = 1 if (len(rows) > MAX_ROWS or len(raw) > MAX_BYTES) else 0
    rows = sorted(rows, key=lambda r: RISKS.index(r[1]))[:MAX_ROWS]   # sorted kararlıdır: aynı riskte dosya sırası korunur
    mapped = sum(1 for r in rows if r[6])
    print("#\x1f%d\x1f%d\x1f%d\x1f%d\x1f%d\x1f%s" % (int(mtime), len(rows), skipped, truncated, mapped, path))
    for r in rows:
        print("\x1f".join(r))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
