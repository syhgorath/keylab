#!/usr/bin/env python3
"""component_index.py - paket indeksini (index.json) DOĞRULAR ve satırlara çevirir (docs/package-index.md §3, §5).

Kullanım: python3 -I component_index.py <index.json> <şimdi-epoch>
İMZA bu betikten ÖNCE bash tarafında doğrulanmış olmalıdır; bu betik imzasız veriyle çağrılmaz.

Çıktı (stdout, "|" ayraçlı):
  meta|<serial>|<issued_at>|<valid_until>
  pkg|<id>|<repo>|<sürüm>|<asset>|<sha256>|<requires_sshpocket>|<platformlar,virgüllü>|<yanked: 0|1>
Hata: stderr'e Türkçe tek satır + çıkış kodu 1 (biçim), 3 (süresi dolmuş), 4 (gelecekte imzalanmış).
Yalnızca stdlib. Yinelenen anahtar, NaN/Infinity, bilinmeyen schema reddedilir; bilinmeyen EK alanlar yok sayılır.
"""
import calendar
import json
import re
import sys
import time

MAX_BYTES = 1024 * 1024
MAX_DAYS = 90
SKEW = 86400
RE_TIME = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
RE_ID = re.compile(r"^[a-z][a-z0-9-]{0,63}$")
RE_VER = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
RE_REPO = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$")
RE_SHA = re.compile(r"^[0-9a-f]{64}$")
RE_PLATFORM = re.compile(r"^[a-z0-9][a-z0-9.-]{0,63}$")
DEFAULT_REQUIRES = "3.7.0"


class Bad(Exception):
    def __init__(self, msg, code=1):
        super().__init__(msg)
        self.code = code


def pairs(items):
    d = {}
    for k, v in items:
        if k in d:
            raise Bad("yinelenen anahtar: %s" % k)
        d[k] = v
    return d


def no_const(name):
    raise Bad("geçersiz sayı değeri: %s" % name)


def epoch(s, what):
    if not isinstance(s, str) or not RE_TIME.match(s):
        raise Bad("%s biçimi geçersiz (YYYY-MM-DDTHH:MM:SSZ bekleniyor)" % what)
    try:
        return calendar.timegm(time.strptime(s, "%Y-%m-%dT%H:%M:%SZ"))
    except ValueError:
        raise Bad("%s geçerli bir tarih değil" % what)


def is_int(v):
    return isinstance(v, int) and not isinstance(v, bool)


def parse(path, now):
    raw = open(path, "rb").read()
    if len(raw) > MAX_BYTES:
        raise Bad("indeks 1 MiB sınırını aşıyor")
    try:
        d = json.loads(raw.decode("utf-8"), object_pairs_hook=pairs, parse_constant=no_const)
    except Bad:
        raise
    except Exception:
        raise Bad("JSON okunamadı")
    if not isinstance(d, dict):
        raise Bad("üst düzey değer nesne olmalı")
    if not is_int(d.get("schema")):
        raise Bad("schema yok")
    if d["schema"] != 1:
        raise Bad("bu sshpocket indeks schema %s sürümünü anlamıyor; sshpocket'i güncelleyin" % d["schema"])
    serial = d.get("serial")
    if not is_int(serial) or serial < 1:
        raise Bad("serial >= 1 olmalı")
    issued = epoch(d.get("issued_at"), "issued_at")
    valid = epoch(d.get("valid_until"), "valid_until")
    if not issued < valid <= issued + MAX_DAYS * 86400:
        raise Bad("valid_until, issued_at'ten sonra ve en çok %d gün içinde olmalı" % MAX_DAYS)
    if issued > now + SKEW:
        raise Bad("indeks gelecekte imzalanmış görünüyor (yerel saati kontrol edin)", 4)
    if now > valid:
        raise Bad("indeksin süresi dolmuş (%s)" % d["valid_until"], 3)
    pk = d.get("packages")
    if not isinstance(pk, dict):
        raise Bad("packages nesne olmalı")
    out = ["meta|%d|%s|%s" % (serial, d["issued_at"], d["valid_until"])]
    for pid in sorted(pk):
        e = pk[pid]
        if not RE_ID.match(pid):
            raise Bad("geçersiz paket kimliği: %s" % pid)
        if not isinstance(e, dict):
            raise Bad("%s: girdi nesne olmalı" % pid)
        repo, ver, asset, sha = e.get("repo"), e.get("version"), e.get("asset"), e.get("sha256")
        if not (isinstance(repo, str) and RE_REPO.match(repo) and ".." not in repo):
            raise Bad("%s: repo geçersiz" % pid)
        if not (isinstance(ver, str) and RE_VER.match(ver)):
            raise Bad("%s: version geçersiz" % pid)
        if asset != "%s-%s.tar.gz" % (pid, ver):
            raise Bad("%s: asset adı '%s-%s.tar.gz' olmalı" % (pid, pid, ver))
        if not (isinstance(sha, str) and RE_SHA.match(sha)):
            raise Bad("%s: sha256 geçersiz" % pid)
        req = e.get("requires_sshpocket", DEFAULT_REQUIRES)
        if not (isinstance(req, str) and RE_VER.match(req)):
            raise Bad("%s: requires_sshpocket geçersiz" % pid)
        plats = e.get("platforms", [])
        if not (isinstance(plats, list) and all(isinstance(p, str) and RE_PLATFORM.match(p) for p in plats)):
            raise Bad("%s: platforms geçersiz" % pid)
        yanked = e.get("yanked", False)
        if not isinstance(yanked, bool):
            raise Bad("%s: yanked bool olmalı" % pid)
        out.append("pkg|%s|%s|%s|%s|%s|%s|%s|%d" % (pid, repo, ver, asset, sha, req, ",".join(plats), 1 if yanked else 0))
    return out


def main():
    if len(sys.argv) != 3 or not sys.argv[2].isdigit():
        print("kullanım: component_index.py <index.json> <şimdi-epoch>", file=sys.stderr)
        return 2
    try:
        for line in parse(sys.argv[1], int(sys.argv[2])):
            print(line)
    except Bad as e:
        print(str(e), file=sys.stderr)
        return e.code
    return 0


if __name__ == "__main__":
    sys.exit(main())
