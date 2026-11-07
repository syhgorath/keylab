#!/usr/bin/env python3
# component_run.py - paket değişkenlerinin doğrulanması ve değişken dosyasının üretimi (docs/package-standard.md §6-§7)
#
# Kullanım: python3 -I component_run.py vars <component.json> <değerler-dosyası> <çıktı.json> [--set ad=değer ...]
# Değerler dosyası düz "ad=değer" satırlarıdır (# yorum, boş satır serbest; liste = virgülle ayrılmış). Hiçbir şey
# eval edilmez. --set, dosyadaki değeri ezer (çalıştırıcının zorladığı değerler: örn. outputs.dir_var).
#        python3 -I component_run.py describe <component.json>            → değişken başına bir TSV satırı
#        python3 -I component_run.py env <vars.json> <sunucu> <eylem> <profil>   → script runner: sunucuda betiğe verilecek export satırları
#        python3 -I component_run.py net-prefix <adres> <maske> | net-suggest <adres> <önek> | net-valid <adres> | net-add <liste> <cidr>   (erişim ağı önerisi / kapsama)
#        python3 -I component_run.py check <component.json> <ad> <değer>   → tek değeri doğrular (warn_if eşleşirse stdout'a "warn=<ad>")
# Çıkış 0: geçerli; çıktı.json izin 600 ile YENİ dosya olarak yazılır. stdout'a yalnızca "warn=<ad>" (warn_if
# eşleşmesi) ve "ok" yazılır; DEĞERLER stdout'a basılmaz. Çıkış 1: geçersiz; nedenler stderr'e. Yalnızca stdlib.
import ipaddress
import json
import os
import re
import sys

MAX_BYTES = 256 * 1024
MAX_VALUES_BYTES = 64 * 1024
RE_NAME = re.compile(r"^[a-z][a-z0-9_]{0,63}$")
RE_CTRL = re.compile(r"[\x00-\x1f\x7f]")
RE_INT = re.compile(r"^-?[0-9]{1,12}$")
RE_LIST = re.compile(r"^list\[(.+)\]$")

errors = []


def err(msg):
    errors.append(msg)


def read_json(path):
    try:
        if os.path.getsize(path) > MAX_BYTES:
            err("component.json çok büyük")
            return None
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError) as e:
        err("component.json okunamadı: %s" % e)
        return None


def parse_values(path):
    out = {}
    if path in ("", "/dev/null"):
        return out
    try:
        if os.path.getsize(path) > MAX_VALUES_BYTES:
            err("değerler dosyası çok büyük")
            return out
        with open(path, "r", encoding="utf-8") as f:
            lines = f.read().splitlines()
    except FileNotFoundError:
        return out
    except (OSError, UnicodeDecodeError) as e:
        err("değerler dosyası okunamadı: %s" % e)
        return out
    for i, line in enumerate(lines, 1):
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if "=" not in s:
            err("değerler dosyası satır %d: 'ad=değer' biçiminde değil" % i)
            continue
        k, v = s.split("=", 1)
        k = k.strip()
        if not RE_NAME.match(k):
            err("değerler dosyası satır %d: geçersiz değişken adı" % i)
            continue
        if k in out:
            err("değerler dosyası: yinelenen değişken: %s" % k)
            continue
        out[k] = v.strip()
    return out


def scalar(t, raw, spec, name):
    """raw -> tür değeri; geçersizse err() ve None."""
    if t in ("string", "path"):
        if RE_CTRL.search(raw) or len(raw) > 1024:
            err("%s: değer geçersiz (denetim karakteri/çok uzun)" % name)
            return None
        return raw
    if t == "bool":
        low = raw.lower()
        if low in ("true", "yes"):
            return True
        if low in ("false", "no"):
            return False
        err("%s: true/false olmalı" % name)
        return None
    if t == "int":
        if not RE_INT.match(raw):
            err("%s: tamsayı olmalı" % name)
            return None
        n = int(raw)
        lo, hi = spec.get("min"), spec.get("max")
        if isinstance(lo, int) and not isinstance(lo, bool) and n < lo:
            err("%s: %d en az %d olmalı" % (name, n, lo))
            return None
        if isinstance(hi, int) and not isinstance(hi, bool) and n > hi:
            err("%s: %d en çok %d olmalı" % (name, n, hi))
            return None
        return n
    if t == "cidr":
        try:
            ipaddress.ip_network(raw, strict=False)
        except ValueError:
            err("%s: geçerli bir CIDR değil" % name)
            return None
        return raw
    if t == "enum":
        vals = spec.get("values") or []
        if raw not in vals:
            err("%s: izin verilen değerlerden biri olmalı" % name)
            return None
        return raw
    err("%s: bilinmeyen tür" % name)
    return None


def typed(t, raw, spec, name):
    m = RE_LIST.match(t)
    if m:
        items = [x.strip() for x in raw.split(",")] if raw.strip() else []
        if any(x == "" for x in items):
            err("%s: listede boş öğe var" % name)
            return None
        res = []
        for x in items:
            v = scalar(m.group(1), x, spec, name)
            if v is None:
                return None
            res.append(v)
        return res
    return scalar(t, raw, spec, name)


def declared_vars(manifest):
    d = read_json(manifest)
    out = {}
    order = []
    if isinstance(d, dict):
        vs = d.get("vars", [])
        for v in vs if isinstance(vs, list) else []:
            if isinstance(v, str):
                v = {"name": v, "type": "string"}
            if isinstance(v, dict) and isinstance(v.get("name"), str):
                out[v["name"]] = v
                order.append(v["name"])
    elif d is not None:
        err("component.json nesne olmalı")
    return out, order


def clean(text):
    return re.sub(r"[\t\r\n\x00-\x1f\x7f]+", " ", str(text)).strip()


def read_vars_file(path):
    vals = {}
    try:
        with open(path, "r", encoding="utf-8") as fh:
            for line in fh:
                if "=" in line and not line.lstrip().startswith("#"):
                    k, _, val = line.rstrip("\n").partition("=")
                    vals[k.strip()] = val.strip()
    except OSError:
        pass
    return vals


def current_platform(manifest, decl, vars_path):
    """platform_var'ın değeri: vars dosyasında yazılı değer, yoksa bildirilen varsayılan; belirlenemezse '' (süzme yapılmaz)."""
    d = read_json(manifest)
    pv = d.get("platform_var") if isinstance(d, dict) else None
    if not isinstance(pv, str) or pv not in decl:
        return ""
    val = read_vars_file(vars_path).get(pv) if vars_path else None
    if not val:
        dv = decl[pv].get("default")
        val = dv if isinstance(dv, str) else ""
    return val or ""


def cmd_platform(argv):
    # platform <component.json> [<vars-dosyası>]: platform_var'ın geçerli değeri (belirlenemezse boş satır)
    if len(argv) not in (3, 4):
        print("kullanım: component_run.py platform <component.json> [<vars-dosyası>]", file=sys.stderr)
        return 2
    decl, _ = declared_vars(argv[2])
    if errors:
        for m in errors:
            print("HATA: %s" % m, file=sys.stderr)
        return 1
    print(clean(current_platform(argv[2], decl, argv[3] if len(argv) == 4 else "")))
    return 0


def cmd_env(argv):
    # env <vars.json> <sunucu> <eylem> <profil>: script runner için sunucuda betiğe verilecek 'export' satırları (stdout).
    # Değişkenler SSHPOCKET_VAR_<AD> olarak, değerler POSIX tek tırnakla kaçırılarak yazılır; liste = virgülle birleşik, bool = true|false.
    if len(argv) != 6:
        print("kullanım: component_run.py env <vars.json> <sunucu> <eylem> <profil>", file=sys.stderr)
        return 2
    import shlex
    vars_path, server, action, profile = argv[2:6]
    for label, val in (("sunucu", server), ("eylem", action), ("profil", profile)):
        if not re.match(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$", val):
            print("HATA: %s adı geçersiz" % label, file=sys.stderr)
            return 1
    data = read_json(vars_path)
    if errors or not isinstance(data, dict):
        print("HATA: değişken dosyası okunamadı", file=sys.stderr)
        return 1
    lines = ["export SSHPOCKET_SERVER=%s" % shlex.quote(server),
             "export SSHPOCKET_ACTION=%s" % shlex.quote(action),
             "export SSHPOCKET_PROFILE=%s" % shlex.quote(profile)]
    for name in sorted(data):
        v = data[name]
        if not RE_NAME.match(name):
            print("HATA: değişken adı geçersiz: %r" % name, file=sys.stderr)
            return 1
        if isinstance(v, bool):
            txt = "true" if v else "false"
        elif isinstance(v, list):
            txt = ",".join(str(x) for x in v)
        elif isinstance(v, (int, str)):
            txt = str(v)
        else:
            print("HATA: %s: desteklenmeyen değer türü" % name, file=sys.stderr)
            return 1
        if RE_CTRL.search(txt):
            print("HATA: %s: denetim karakteri içeriyor" % name, file=sys.stderr)
            return 1
        lines.append("export SSHPOCKET_VAR_%s=%s" % (name.upper(), shlex.quote(txt)))
    print("\n".join(lines))
    return 0


def cmd_describe(argv):
    # describe <component.json> [<vars-dosyası>]: vars-dosyası verilirse platform_var'a göre platforms dışındaki değişkenler çıkarılır
    if len(argv) not in (3, 4):
        print("kullanım: component_run.py describe <component.json> [<vars-dosyası>]", file=sys.stderr)
        return 2
    decl, order = declared_vars(argv[2])
    if errors:
        for m in errors:
            print("HATA: %s" % m, file=sys.stderr)
        return 1
    plat = current_platform(argv[2], decl, argv[3]) if len(argv) == 4 else ""
    # grup sırası: grubun ilk göründüğü yere göre; gruplar yan yana, grupsuzlar ilk sıralarında kalır
    groups = []
    for name in order:
        g = decl[name].get("group")
        g = clean(g) if isinstance(g, str) else ""
        if g and g not in groups:
            groups.append(g)
    def sort_key(name):
        g = decl[name].get("group")
        g = clean(g) if isinstance(g, str) else ""
        return (groups.index(g) + 1) if g else 0
    for name in sorted(order, key=lambda n: (sort_key(n), order.index(n))):
        v = decl[name]
        pls = v.get("platforms")
        if plat and isinstance(pls, list) and plat not in pls:
            continue
        dflt = json.dumps(v["default"], ensure_ascii=False) if "default" in v else ""
        warn = json.dumps(v["warn_if"]) if "warn_if" in v else ""
        mn = str(v["min"]) if isinstance(v.get("min"), int) and not isinstance(v.get("min"), bool) else ""
        mx = str(v["max"]) if isinstance(v.get("max"), int) and not isinstance(v.get("max"), bool) else ""
        grp = v.get("group")
        grp = clean(grp) if isinstance(grp, str) else ""
        vals = json.dumps(v["values"], ensure_ascii=False, separators=(",", ":")) if isinstance(v.get("values"), list) else ""
        print("\t".join([name, clean(v.get("type", "string")), "yes" if v.get("required") is True else "no",
                         clean(dflt), mn, mx, clean(warn), clean(v.get("description", "")), grp, clean(vals)]))
    return 0


def cmd_check(argv):
    if len(argv) != 5:
        print("kullanım: component_run.py check <component.json> <ad> <değer>", file=sys.stderr)
        return 2
    decl, _ = declared_vars(argv[2])
    name, raw = argv[3], argv[4]
    if name not in decl:
        err("%s: pakette tanımlı bir değişken değil" % name)
    elif raw == "":
        err("%s: değer boş" % name)
    else:
        spec = decl[name]
        v = typed(spec.get("type", "string"), raw, spec, name)
        if v is not None and "warn_if" in spec and isinstance(v, bool) and v == spec["warn_if"]:
            print("warn=%s" % name)
    if errors:
        for m in errors:
            print("HATA: %s" % m, file=sys.stderr)
        return 1
    return 0


def cmd_net_add(argv):
    """net-add <virgüllü liste> <yeni CIDR> → 1. satır: durum (added | duplicate | covered:<kapsayan> | replaced:<kaldırılanlar>), 2. satır: sonuç listesi (virgüllü)"""
    existing = [x for x in argv[2].split(",") if x]
    new_raw = argv[3]
    try:
        new = ipaddress.ip_network(new_raw, strict=False)
        nets = [(x, ipaddress.ip_network(x, strict=False)) for x in existing]
    except ValueError:
        return 1
    for raw, net in nets:
        if net == new:
            print("duplicate")
            print(",".join(existing))
            return 0
    for raw, net in nets:
        if net.version == new.version and new.subnet_of(net):
            print("covered:%s" % raw)
            print(",".join(existing))
            return 0
    keep = [raw for raw, net in nets if not (net.version == new.version and net.subnet_of(new))]
    removed = [raw for raw, net in nets if raw not in keep]
    print(("replaced:%s" % ",".join(removed)) if removed else "added")
    print(",".join(keep + [new_raw]))
    return 0


def cmd_net(argv):
    """net-prefix <adres> <maske>  → "adres/önek" · net-suggest <adres> <önek> → "CIDR\taçıklama" satırları · net-valid <adres> → 0/1"""
    sub = argv[1]
    try:
        if sub == "net-valid" and len(argv) == 3:
            ipaddress.ip_address(argv[2])
            print(argv[2])
            return 0
        if sub == "net-prefix" and len(argv) == 4:
            addr = ipaddress.ip_address(argv[2])
            m = argv[3]
            mask = ipaddress.ip_address(int(m, 16) if m.lower().startswith("0x") else m)
            plen = bin(int(mask)).count("1")
            if int(mask) != ((1 << plen) - 1) << (addr.max_prefixlen - plen):
                return 1                                   # bitişik olmayan maske
            print("%s/%d" % (addr, plen))
            return 0
        if sub == "net-suggest" and len(argv) == 4:
            addr = ipaddress.ip_address(argv[2])
            plen = int(argv[3])
            if not 0 <= plen <= addr.max_prefixlen:
                return 1
            out = []
            if plen < addr.max_prefixlen:
                out.append((str(ipaddress.ip_network("%s/%d" % (addr, plen), strict=False)), "bu ağdaki herkes"))
            elif addr.version == 4:
                out.append((str(ipaddress.ip_network("%s/24" % addr, strict=False)),
                            "tahmini /24 (ağ maskesi bilinmiyor; doğrulayın)"))
            out.append(("%s/%d" % (addr, addr.max_prefixlen), "yalnızca bu adres"))
            seen = set()
            for cidr, label in out:
                if cidr not in seen:
                    seen.add(cidr)
                    print("%s\t%s" % (cidr, label))
            return 0
    except (ValueError, TypeError):
        return 1
    return 2


def main(argv):
    if len(argv) == 4 and argv[1] == "net-add":
        return cmd_net_add(argv)
    if len(argv) >= 2 and argv[1] in ("net-prefix", "net-suggest", "net-valid"):
        return cmd_net(argv)
    if len(argv) >= 2 and argv[1] == "env":
        return cmd_env(argv)
    if len(argv) >= 2 and argv[1] == "platform":
        return cmd_platform(argv)
    if len(argv) >= 2 and argv[1] == "describe":
        return cmd_describe(argv)
    if len(argv) >= 2 and argv[1] == "check":
        return cmd_check(argv)
    if len(argv) < 5 or argv[1] != "vars":
        print("kullanım: component_run.py vars|describe|check ...", file=sys.stderr)
        return 2
    manifest, values_path, out_path = argv[2], argv[3], argv[4]
    forced = {}
    rest = argv[5:]
    require = True
    while rest:
        if rest[0] == "--no-required":     # salt okunur eylemler: zorunlu değişken eksikliği hata sayılmaz (verilenler yine doğrulanır)
            require = False
            rest = rest[1:]
            continue
        if rest[0] != "--set" or len(rest) < 2 or "=" not in rest[1]:
            print("--set ad=değer ya da --no-required bekleniyordu", file=sys.stderr)
            return 2
        k, v = rest[1].split("=", 1)
        forced[k] = v
        rest = rest[2:]

    d = read_json(manifest)
    declared = {}
    if isinstance(d, dict):
        for v in d.get("vars", []) if isinstance(d.get("vars", []), list) else []:
            if isinstance(v, str):
                v = {"name": v, "type": "string"}
            if isinstance(v, dict) and isinstance(v.get("name"), str):
                declared[v["name"]] = v
    elif d is not None:
        err("component.json nesne olmalı")

    values = parse_values(values_path)
    values.update(forced)
    for k in sorted(values):
        if k not in declared:
            err("%s: pakette tanımlı bir değişken değil" % k)

    result, warns = {}, []
    for name, spec in declared.items():
        t = spec.get("type", "string")
        if name in values and values[name] != "":
            v = typed(t, values[name], spec, name)
            if v is None:
                continue                     # neden zaten bildirildi
        elif "default" in spec:
            v = spec["default"]
        elif name in values and RE_LIST.match(t):
            v = []                           # "ad=" → boş liste (zorunluysa aşağıda reddedilir)
        else:
            v = None
        if v is None or (spec.get("required") is True and v == []):
            if spec.get("required") is True and require:
                err("%s: zorunlu değişken eksik" % name)
            continue
        result[name] = v
        if "warn_if" in spec and isinstance(v, bool) and v == spec["warn_if"]:
            warns.append(name)

    if errors:
        for m in errors:
            print("HATA: %s" % m, file=sys.stderr)
        return 1
    try:
        fd = os.open(out_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump(result, f, ensure_ascii=False, indent=2, sort_keys=True)
            f.write("\n")
    except OSError as e:
        print("HATA: değişken dosyası yazılamadı: %s" % e, file=sys.stderr)
        return 1
    for w in sorted(warns):
        print("warn=%s" % w)
    print("ok")
    return 0


sys.exit(main(sys.argv))
