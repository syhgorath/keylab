#!/usr/bin/env python3
# component_validate.py - paket SÖZLEŞME UYUM denetimi (docs/package-standard.md §12 kontrol listesinin mekanik kısmı)
#
# Kullanım: python3 -I component_validate.py <paket-dizini | <id>-<sürüm>.tar.gz> [--strict] [--unsigned]
# --unsigned: arşiv HENÜZ imzalanmadı (imzalamadan önceki denetim); '.sig yok' uyarısını atlar.
# YALNIZCA STATİK: ansible/ssh/ağ çalıştırmaz, paketten hiçbir şey çalıştırmaz. Yalnızca stdlib.
# Çıkış 0: ❌ yok (⚠️ olabilir; --strict ile uyarılar da hata sayılır) · 1: en az bir ❌ · 2: kullanım hatası.
# Bildirim doğrulaması (component_manifest.py) AYNI dizinden çağrılır; paket reposunun CI'ı için ikisi birlikte kopyalanır.
import json
import os
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
MANIFEST_PY = os.path.join(HERE, "component_manifest.py")

SECTIONS = ["Bildirim", "Düzen", "Eylemler", "Değişkenler", "Güvenlik", "Belge"]
results = []          # (bölüm, düzey, metin)  düzey: ok | warn | fail

TEXT_EXT = {".yml", ".yaml", ".j2", ".md", ".json", ".txt", ".cfg", ".sh", ".py", ".ini", ".in", ".toml", ""}
MAX_FILE = 512 * 1024
# parola ÖZETİ (PBKDF2/crypt çıktısı) değişken olabilir; düz parola/anahtar olamaz (standart §6). Yalnızca adı _hash/_digest/_checksum ile BİTENLER gevşetilir.
DIGEST_NAME = re.compile(r"_(hash|digest|checksum)$", re.I)
SECRET_NAME = re.compile(r"(pass(word|wd)?|secret|token|credential|private_?key|api_?key)", re.I)
SECRET_FILE = re.compile(r"(^|/)(\.env|id_(ed25519|rsa|ecdsa|dsa)[^/]*|[^/]*\.pem|[^/]*\.secrets)$")
PRIVATE_KEY = re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----")
ROLLBACK_WORDS = re.compile(r"(geri al|rollback|restore|kurtar|recovery|undo)", re.I)
# kurtarma eylemleri kontrol kaybından sonra da çalışmalı: ön koşul (preview) beklenmez
RECOVERY_NAME = re.compile(r"(^|_)(rollback|recover|restore|undo|geri)", re.I)   # ad ya da "_" sonrası sözcük: rollback_now, kaldir_geri_yukle, geri_al


def ok(sec, msg):
    results.append((sec, "ok", msg))


def warn(sec, msg):
    results.append((sec, "warn", msg))


def fail(sec, msg):
    results.append((sec, "fail", msg))


# ---------------------------------------------------------------- girdi: dizin ya da arşiv
def safe_extract(archive, dest):
    """Arşivi YALNIZCA güvenliyse açar: mutlak yol, '..', bağ (sembolik/sert), aygıt dosyası, aşırı boyut/sayı reddedilir."""
    total = 0
    with tarfile.open(archive, "r:gz") as tf:
        members = tf.getmembers()
        if len(members) > 5000:
            raise ValueError("arşivde çok fazla öğe")
        for m in members:
            name = m.name
            if name.startswith("/") or ".." in name.split("/"):
                raise ValueError("güvensiz yol: %s" % name)
            if m.issym() or m.islnk() or m.isdev() or m.isfifo():
                raise ValueError("bağ/aygıt dosyası yasak: %s" % name)
            total += m.size
            if total > 50 * 1024 * 1024:
                raise ValueError("arşiv çok büyük (açılmış > 50 MB)")
        for m in members:
            tf.extract(m, dest)


def resolve_input(arg):
    """-> (kök_dizin, id_ipucu, sürüm_ipucu, geçici_dizin|None, arşiv_yolu|None)"""
    if os.path.isdir(arg):
        return os.path.abspath(arg), None, None, None, None
    if os.path.isfile(arg):
        base = os.path.basename(arg)
        m = re.match(r"^([a-z][a-z0-9-]*)-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz$", base)
        if not m:
            fail("Düzen", "arşiv adı '<id>-<sürüm>.tar.gz' olmalı (örn. paket-1.0.0.tar.gz): %s" % base)
            return None, None, None, None, arg
        tmp = tempfile.mkdtemp(prefix="sshpocket-validate-")
        os.chmod(tmp, 0o700)
        try:
            safe_extract(arg, tmp)
        except (ValueError, tarfile.TarError, OSError) as e:
            fail("Düzen", "arşiv güvenli açılamadı: %s" % e)
            return None, m.group(1), m.group(2), tmp, arg
        top = [n for n in os.listdir(tmp)]
        want = "%s-%s" % (m.group(1), m.group(2))
        if top != [want] or not os.path.isdir(os.path.join(tmp, want)):
            fail("Düzen", "arşiv kökü tek klasör '%s/' olmalı (bulundu: %s)" % (want, ", ".join(sorted(top)) or "boş"))
            return None, m.group(1), m.group(2), tmp, arg
        ok("Düzen", "arşiv kökü '%s/'" % want)
        return os.path.join(tmp, want), m.group(1), m.group(2), tmp, arg
    fail("Düzen", "bulunamadı: %s" % arg)
    return None, None, None, None, None


# ---------------------------------------------------------------- yardımcılar
def read_text(path):
    try:
        if os.path.getsize(path) > MAX_FILE:
            return ""
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return ""


# DİZİN modunda (yazarın çalışma klasörü) gitignore'lu geliştirme klasörleri atlanır; ARŞİV modunda ATLANMAZ (yayın arşivinde bunlar olmamalı).
DEV_DIRS = (".venv", "venv", ".tox", ".pytest_cache", ".mypy_cache", ".ruff_cache")
skip_dev = []


def walk_files(root):
    for d, dirs, files in os.walk(root):
        dirs[:] = [x for x in dirs if x not in (".git", "node_modules", "__pycache__") and x not in skip_dev]
        for fn in files:
            yield os.path.join(d, fn)


def corpus(root, subdirs):
    out = []
    for sd in subdirs:
        p = os.path.join(root, sd)
        if os.path.isfile(p):
            out.append(read_text(p))
        elif os.path.isdir(p):
            for f in walk_files(p):
                if os.path.splitext(f)[1].lower() in TEXT_EXT:
                    out.append(read_text(f))
    return "\n".join(out)


def parse_info(text):
    info = {}
    for line in text.splitlines():
        if "=" not in line:
            continue
        k, v = line.split("=", 1)
        info.setdefault(k, []).append(v)
    return info


def first(info, key, default=""):
    return info.get(key, [default])[0]


# ---------------------------------------------------------------- denetimler
def check_manifest(root, id_hint, ver_hint):
    mf = os.path.join(root, "component.json")
    if not os.path.isfile(mf):
        fail("Bildirim", "component.json yok (paket kökünde olmalı)")
        return None
    args = [sys.executable, "-I", MANIFEST_PY, mf]
    if id_hint:
        args += [id_hint, ver_hint]
    p = subprocess.run(args, capture_output=True, text=True)
    if p.returncode != 0:
        for line in (p.stderr or "bildirim doğrulanamadı").strip().splitlines():
            fail("Bildirim", line.strip())
        return None
    info = parse_info(p.stdout)
    ok("Bildirim", "component.json geçerli (id=%s, sürüm=%s, runner=%s)" % (first(info, "id"), first(info, "version"), first(info, "runner")))
    if not any(v.endswith(":supported") for v in info.get("profile", [])):
        fail("Bildirim", "en az bir 'supported' profil gerekir (yalnızca planned/boş)")
    else:
        ok("Bildirim", "desteklenen profil var: %s" % ", ".join(v for v in info.get("profile", []) if v.endswith(":supported")))
    return info


def check_layout(root, info):
    ver = first(info, "version")
    vf = os.path.join(root, "VERSION")
    if not os.path.isfile(vf):
        fail("Düzen", "VERSION dosyası yok (sürüm üçlüsü: VERSION, component.json, etiket)")
    else:
        v = read_text(vf).strip()
        if v == ver:
            ok("Düzen", "VERSION = component.json sürümü (%s)" % ver)
        else:
            fail("Düzen", "VERSION '%s' ≠ component.json sürümü '%s'" % (v, ver))
    cy = os.path.join(root, "component.yml")
    if os.path.isfile(cy):
        m = re.search(r"^version:\s*['\"]?([0-9.]+)", read_text(cy), re.M)
        if m and m.group(1) != ver:
            warn("Düzen", "component.yml sürümü '%s' ≠ component.json '%s' (JSON yml'den türetilmeli)" % (m.group(1), ver))
    cl = os.path.join(root, "CHANGELOG.md")
    if os.path.isfile(cl) and not re.search(r"^## \[%s\]" % re.escape(ver), read_text(cl), re.M):
        warn("Düzen", "CHANGELOG.md'de [%s] başlığı yok" % ver)

    if first(info, "runner") == "ansible":
        rq = os.path.join(root, "requirements.txt")
        if not os.path.isfile(rq):
            fail("Düzen", "requirements.txt yok (runner=ansible; hash'li, sabit sürümlü olmalı)")
        else:
            check_requirements(rq)
        if not os.path.isfile(os.path.join(root, "ansible.cfg")):
            warn("Düzen", "ansible.cfg yok (çalışma dizini paket kökü; ayarlar oradan okunur)")


def check_requirements(path):
    raw = read_text(path)
    logical, cur = [], ""
    for line in raw.splitlines():
        s = line.rstrip()
        if s.lstrip().startswith("#") or not s.strip():
            continue
        if s.endswith("\\"):
            cur += s[:-1] + " "
        else:
            logical.append((cur + s).strip())
            cur = ""
    if cur.strip():
        logical.append(cur.strip())
    reqs = [l for l in logical if not l.startswith("-")]
    bad = []
    for l in reqs:
        if "==" not in l:
            bad.append("%s: sabit sürüm (==) yok" % l.split()[0])
        elif "--hash=sha256:" not in l:
            bad.append("%s: --hash=sha256 yok" % l.split()[0])
        if re.search(r"(git\+|https?://|file:|@ )", l):
            bad.append("%s: URL/VCS bağımlılığı yasak" % l.split()[0])
    if any(l.startswith(("-e ", "-r ", "--editable")) for l in logical):
        bad.append("-e/-r yönergeleri yasak")
    if not reqs:
        bad.append("hiç bağımlılık yok (ansible-core bekleniyordu)")
    if bad:
        for b in bad:
            fail("Düzen", "requirements.txt: %s" % b)
    else:
        ok("Düzen", "requirements.txt: %d bağımlılık, hepsi sabit sürümlü ve hash'li" % len(reqs))


def check_actions(root, info):
    pid = first(info, "id")
    runner = first(info, "runner")
    names = info.get("action", [])
    code = corpus(root, ["playbooks", "roles", "vars", "inventory", "templates", "files", "scripts"])
    mutating_without_docs = False
    for a in names:
        cmd = first(info, "action.%s.cmd" % a)
        effects = first(info, "action.%s.effects" % a)
        sudo = first(info, "action.%s.sudo" % a)
        nwarn = int(first(info, "action.%s.warnings" % a, "0") or 0)
        requires = [r for r in first(info, "action.%s.requires" % a).split(",") if r]
        mutating = bool(effects)
        label = "%s%s" % (a, " (değiştirir: %s)" % effects if mutating else " (salt okunur)")
        issues = []

        # komut: ansible-playbook <var olan playbook>
        toks = cmd.split()
        if runner == "ansible":
            if not toks or toks[0] != "ansible-playbook":
                fail("Eylemler", "%s: cmd 'ansible-playbook' ile başlamalı" % a)
                continue
            pbs = [t for t in toks[1:] if t.endswith((".yml", ".yaml"))]
            if not pbs:
                fail("Eylemler", "%s: cmd bir playbook (.yml) içermiyor" % a)
                continue
            for pb in pbs:
                if pb.startswith("/") or ".." in pb.split("/"):
                    fail("Eylemler", "%s: playbook yolu paket içinde ve göreli olmalı (%s)" % (a, pb))
                elif not os.path.isfile(os.path.join(root, pb)):
                    fail("Eylemler", "%s: playbook yok: %s" % (a, pb))
                else:
                    pass
        elif runner == "script":
            # script: cmd 'scripts/<ad>.sh [argümanlar]' (biçimi manifest doğrulayıcısı denetler); betik pakette olmalı, düz dosya (sembolik bağ değil)
            script = os.path.join(root, toks[0]) if toks else ""
            if not toks or not os.path.isfile(script):
                fail("Eylemler", "%s: betik yok: %s" % (a, toks[0] if toks else "(boş)"))
                continue
            if os.path.islink(script):
                fail("Eylemler", "%s: betik sembolik bağ olamaz: %s" % (a, toks[0]))
                continue
            if os.path.getsize(script) > 262144:
                fail("Eylemler", "%s: betik 256 KB'tan büyük: %s" % (a, toks[0]))
                continue
        # değiştiren eylem kuralları (uyarı: kurtarma eylemleri ön koşulsuz olabilir)
        if mutating:
            if sudo != "yes":
                issues.append("requires_sudo belirtilmemiş")
            if nwarn == 0:
                issues.append("hiç warning yok (neyi değiştirdiğini/önizlemenin göstermediklerini yazın)")
            readonly = [r for r in requires if not first(info, "action.%s.effects" % r)]
            # yalnızca araç kuran eylem (örn. denetim tarayıcısı) yapılandırmayı değiştirmez; kurtarma eylemleri bilerek ön koşulsuzdur
            config_changing = bool(set(effects.split(",")) - {"installs-package"})
            if not readonly and config_changing and not RECOVERY_NAME.search(a):
                issues.append("salt okunur bir 'preview' ön koşulu yok (geri alma/kurtarma eylemleri bilerek ön koşulsuz olabilir)")
            mutating_without_docs = True
        if first(info, "action.%s.outputs.format" % a):
            tag = "SSHPOCKET_RESULT %s.%s" % (pid, a)
            if tag not in code:
                issues.append("outputs.format var ama playbook/rol/betiklerde '%s' çıktısı bulunamadı" % tag)
        if issues:
            for i in issues:
                warn("Eylemler", "%s: %s" % (a, i))
        else:
            ok("Eylemler", label)
    return mutating_without_docs


def check_vars(root, info):
    vars_ = [v.split(":") for v in info.get("var", [])]
    if not vars_:
        ok("Değişkenler", "değişken tanımı yok")
        return
    code = corpus(root, ["playbooks", "roles", "vars", "inventory", "templates", "files", "scripts"])
    for name, vtype, req, typed in [(v + [""] * 4)[:4] for v in vars_]:
        notes = []
        if typed == "0":
            notes.append("eski biçim (yalnızca ad; tür/varsayılan/açıklama üst verisi yok)")
        # bool/int bir parola taşıyamaz (örn. password_inactive_days = gün sayısı): yalnızca metin alabilen türlerde uyar
        is_digest = bool(SECRET_NAME.search(name) and DIGEST_NAME.search(name))
        if SECRET_NAME.search(name) and not is_digest and vtype not in ("bool", "int"):
            notes.append("adı parola/anahtar/token izlenimi veriyor: gizli değer değişken OLAMAZ (standart §6)")
        # script runner: değişken betikte SSHPOCKET_VAR_<AD> olarak okunur
        if name not in code and ("SSHPOCKET_VAR_%s" % name.upper()) not in code:
            notes.append("playbook/rol/vars/betik içinde kullanılmıyor")
        if notes:
            for n in notes:
                warn("Değişkenler", "%s: %s" % (name, n))
        else:
            ok("Değişkenler", "%s (%s%s%s)" % (name, vtype, ", zorunlu" if req == "yes" else "", ", parola ÖZETİ: düz parola olamaz" if is_digest and vtype not in ("bool", "int") else ""))


def check_security(root):
    bad = False
    for f in walk_files(root):
        rel = os.path.relpath(f, root)
        if os.path.islink(f):
            fail("Güvenlik", "sembolik bağ yasak (kurulumda reddedilir): %s" % rel)
            bad = True
            continue
        if SECRET_FILE.search(rel):
            fail("Güvenlik", "gizli bilgi dosyası paket içinde: %s" % rel)
            bad = True
        elif os.path.splitext(f)[1].lower() in TEXT_EXT and PRIVATE_KEY.search(read_text(f)):
            fail("Güvenlik", "özel anahtar bloğu bulundu: %s" % rel)
            bad = True
    for d, dirs, files in os.walk(root):
        dirs[:] = [x for x in dirs if x not in skip_dev]
        for n in dirs:
            if os.path.islink(os.path.join(d, n)):
                fail("Güvenlik", "sembolik bağ yasak: %s" % os.path.relpath(os.path.join(d, n), root))
                bad = True
    if not bad:
        ok("Güvenlik", "sembolik bağ, özel anahtar ve gizli bilgi dosyası yok")


def check_docs(root, mutating, archive, unsigned=False):
    md = corpus(root, [n for n in os.listdir(root) if n.lower().endswith(".md")] + (["docs"] if os.path.isdir(os.path.join(root, "docs")) else []))
    if mutating:
        if ROLLBACK_WORDS.search(md):
            ok("Belge", "değiştiren eylemler için geri alma/kurtarma anlatılmış görünüyor (belgede anahtar sözcük bulundu)")
        else:
            warn("Belge", "değiştiren eylem var ama README/docs'ta geri alma/kurtarma yolu anlatılmamış (kapsamı dürüstçe yazın)")
    else:
        ok("Belge", "değiştiren eylem yok; geri alma belgesi gerekmiyor")
    if archive and unsigned:
        ok("Belge", "imza henüz beklenmiyor (--unsigned)")
    elif archive:
        if os.path.isfile(archive + ".sig"):
            ok("Belge", "imza dosyası yanında (.sig); imzayı doğrulamak için scripts/lock-component.sh")
        else:
            warn("Belge", "%s.sig yok (paket 'ssh-keygen -Y sign -n sshpocket-component' ile imzalanmalı)" % os.path.basename(archive))


# ---------------------------------------------------------------- çıktı
def render(strict):
    icon = {"ok": "✅", "warn": "⚠️ ", "fail": "❌"}
    nfail = sum(1 for _, l, _ in results if l == "fail")
    nwarn = sum(1 for _, l, _ in results if l == "warn")
    for sec in SECTIONS:
        rows = [(l, t) for s, l, t in results if s == sec]
        if not rows:
            continue
        print(sec)
        for l, t in rows:
            print("  %s %s" % (icon[l], t))
        print("")
    if nfail:
        print("Sonuç: ❌ uyumsuz (%d hata, %d uyarı)" % (nfail, nwarn))
    elif nwarn and strict:
        print("Sonuç: ❌ --strict: %d uyarı hata sayıldı" % nwarn)
    elif nwarn:
        print("Sonuç: ✅ uyumlu (%d uyarı: bakmanız önerilir)" % nwarn)
    else:
        print("Sonuç: ✅ uyumlu")
    return 1 if (nfail or (strict and nwarn)) else 0


def main(argv):
    args = [a for a in argv[1:] if not a.startswith("--")]
    strict = "--strict" in argv[1:]
    unsigned = "--unsigned" in argv[1:]
    if len(args) != 1 or any(a.startswith("--") and a not in ("--strict", "--unsigned") for a in argv[1:]):
        print("kullanım: component_validate.py <paket-dizini | <id>-<sürüm>.tar.gz> [--strict] [--unsigned]", file=sys.stderr)
        return 2
    root, id_hint, ver_hint, tmp, archive = resolve_input(args[0])
    if root is not None and archive is None:
        skip_dev.extend(DEV_DIRS)
        found = [x for x in DEV_DIRS if os.path.isdir(os.path.join(root, x))]
        if found:
            ok("Düzen", "geliştirme klasörleri atlandı (%s): dizin modu; yayın arşivinde bulunmamalı, arşiv modu atlamaz" % ", ".join(found))
    try:
        print("sshpocket --components validate %s\n" % args[0])
        if root is not None:
            info = check_manifest(root, id_hint, ver_hint)
            if info is not None:
                check_layout(root, info)
                mutating = check_actions(root, info)
                check_vars(root, info)
            else:
                mutating = False
            check_security(root)
            check_docs(root, mutating, archive, unsigned)
        return render(strict)
    finally:
        if tmp and os.path.isdir(tmp):
            shutil.rmtree(tmp, ignore_errors=True)


sys.exit(main(sys.argv))
