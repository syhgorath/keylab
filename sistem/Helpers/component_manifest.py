#!/usr/bin/env python3
# component_manifest.py - component.json doğrulayıcısı (docs/package-standard.md §4-§6, sshpocket_api 1)
#
# Kullanım: python3 -I component_manifest.py <component.json> [beklenen-id] [beklenen-sürüm]
# Çıkış 0: geçerli; stdout'a düz "anahtar=değer" satırları yazar (bash okur, eval edilmez).
# Çıkış 1: geçersiz; nedenler stderr'e. Yalnızca stdlib. Bilinmeyen alanlar yok sayılır;
# bilinmeyen ZORUNLU özellik (required_features) ve daha yeni sshpocket_api reddedilir.
import json
import re
import sys

SUPPORTED_API = {1}
MAX_BYTES = 256 * 1024
EFFECTS = ("installs-package", "changes-config", "destructive")
CONFIRMS = ("none", "yes-no", "type-server-name")
RUNNERS = ("ansible", "script")
STATUSES = ("supported", "planned")
VAR_SCALARS = ("string", "bool", "int", "path", "cidr", "enum")

RE_ID = re.compile(r"^[a-z][a-z0-9-]{0,63}$")
RE_VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
RE_NAME = re.compile(r"^[a-z][a-z0-9_]{0,63}$")
RE_PLATFORM = re.compile(r"^[a-z0-9][a-z0-9._-]{0,29}$")
RE_PROFILE = re.compile(r"^[a-z0-9][a-z0-9._-]{0,63}$")
RE_GROUP = re.compile(r"^[A-Za-z][A-Za-z0-9_]{0,63}$")
RE_CMD_BAD = re.compile(r"[;&|`$<>(){}\\\"'\n\r\t\x00]")
VAR_DESC_MAX = 2000   # değişken açıklaması (arayüzde gösterilir); diğer metinler 500
RE_TEXT_BAD = re.compile(r"[\x00-\x08\x0b-\x1f\x7f]")
RE_PYRANGE = re.compile(r"^>=[0-9]+\.[0-9]+(,<[0-9]+\.[0-9]+)?$")
RE_SCRIPT = re.compile(r"^scripts/[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.sh$")
RE_SCRIPT_ARG = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._=:/+,-]{0,63}$|^--?[A-Za-z0-9][A-Za-z0-9._=:/+,-]{0,63}$")
RE_OUTKEY = re.compile(r"^[a-z][a-z0-9_]{0,31}$")
RE_FINDINGS = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,59}\.tsv$")
RE_VALUE = re.compile(r"^(<[a-z0-9_ -]{1,40}>|[A-Za-z0-9_./:@=,+-]{1,200})$")

errors = []


def err(msg):
    errors.append(msg)


def is_str(v):
    return isinstance(v, str)


def is_int(v):
    return isinstance(v, int) and not isinstance(v, bool)


def text_ok(v, what, maxlen=500):
    if not is_str(v) or not v or len(v) > maxlen or RE_TEXT_BAD.search(v):
        err("%s geçersiz metin" % what)
        return False
    return True


def var_type_ok(t):
    if t in VAR_SCALARS:
        return True
    m = re.match(r"^list\[([a-z]+)\]$", t) if is_str(t) else None
    return bool(m and m.group(1) in VAR_SCALARS and m.group(1) != "enum")


def main():
    if len(sys.argv) < 2:
        print("kullanım: component_manifest.py <component.json> [id] [sürüm]", file=sys.stderr)
        return 2
    path = sys.argv[1]
    want_id = (sys.argv[2] if len(sys.argv) > 2 else "") or None
    want_ver = (sys.argv[3] if len(sys.argv) > 3 else "") or None
    try:
        with open(path, "rb") as f:
            raw = f.read(MAX_BYTES + 1)
    except OSError:
        print("bildirim okunamadı", file=sys.stderr)
        return 1
    if len(raw) > MAX_BYTES:
        print("bildirim çok büyük", file=sys.stderr)
        return 1
    try:
        d = json.loads(raw.decode("utf-8"))
    except Exception:
        print("bildirim geçerli JSON değil", file=sys.stderr)
        return 1
    if not isinstance(d, dict):
        print("bildirim bir nesne olmalı", file=sys.stderr)
        return 1

    # --- kimlik, sürüm, sözleşme sürümü ---
    pid, ver, api = d.get("id"), d.get("version"), d.get("sshpocket_api")
    if not (is_str(pid) and RE_ID.match(pid)):
        err("id geçersiz")
    if not (is_str(ver) and RE_VERSION.match(ver)):
        err("version geçersiz")
    if want_id is not None and pid != want_id:
        err("id paketle uyuşmuyor")
    if want_ver is not None and ver != want_ver:
        err("version paketle uyuşmuyor")
    if not is_int(api):
        err("sshpocket_api tamsayı olmalı")
    elif api not in SUPPORTED_API:
        err("sshpocket_api %s desteklenmiyor (bu sshpocket: %s); daha yeni bir sshpocket gerekebilir"
            % (api, ",".join(str(x) for x in sorted(SUPPORTED_API))))
    rf = d.get("required_features")
    if rf is not None and (not isinstance(rf, list) or rf):
        err("bilinmeyen zorunlu özellik (required_features): bu sshpocket sürümü desteklemiyor")

    runner = d.get("runner")
    if runner not in RUNNERS:
        err("runner 'ansible' ya da 'script' olmalı")
    for k in ("name", "description"):
        if k in d:
            text_ok(d[k], k)
    group = d.get("inventory_group")
    if group is not None and not (is_str(group) and RE_GROUP.match(group)):
        err("inventory_group geçersiz")

    # --- kontrol makinesi gereksinimi ---
    req_py = ""
    req = d.get("requires", {})
    if not isinstance(req, dict):
        err("requires nesne olmalı")
    else:
        ctrl = req.get("control", {})
        if not isinstance(ctrl, dict):
            err("requires.control nesne olmalı")
        elif "python" in ctrl:
            if is_str(ctrl["python"]) and RE_PYRANGE.match(ctrl["python"]):
                req_py = ctrl["python"]
            else:
                err("requires.control.python geçersiz (örn. \">=3.12\" veya \">=3.12,<4.0\")")

    # --- profiller ---
    profiles = d.get("profiles")
    prof_lines = []
    if not isinstance(profiles, list) or not profiles:
        err("profiles boş olamaz")
    else:
        seen = set()
        for p in profiles:
            if (not isinstance(p, dict) or not (is_str(p.get("id")) and RE_PROFILE.match(p["id"]))
                    or p.get("status") not in STATUSES):
                err("profil geçersiz (id, status: supported|planned)")
                continue
            if p["id"] in seen:
                err("yinelenen profil: %s" % p["id"])
            seen.add(p["id"])
            prof_lines.append("profile=%s:%s" % (p["id"], p["status"]))

    # --- eylemler ---
    actions = d.get("actions")
    act_lines = []
    if not isinstance(actions, dict) or not actions:
        err("actions boş olamaz")
        actions = {}
    for name, a in actions.items():
        if not (is_str(name) and RE_NAME.match(name)):
            err("eylem adı geçersiz: %r" % (name,))
            continue
        if not isinstance(a, dict):
            err("eylem nesne olmalı: %s" % name)
            continue
        cmd = a.get("cmd")
        if not is_str(cmd) or not cmd.strip() or len(cmd) > 400 or RE_CMD_BAD.search(cmd):
            err("%s: cmd geçersiz (kabuk meta karakteri/satır sonu yok, boş olamaz)" % name)
            cmd = ""
        if runner == "script" and cmd:
            # script: ilk sözcük paket içindeki 'scripts/<ad>.sh' yolu (".." ve mutlak yol yok), kalanı düz argüman (sunucuda 'bash -s -- <argümanlar>')
            sw = cmd.split()
            if not RE_SCRIPT.match(sw[0]) or ".." in sw[0]:
                err("%s: script runner için cmd 'scripts/<ad>.sh [argümanlar]' ile başlamalı" % name)
                cmd = ""
            elif any(not RE_SCRIPT_ARG.match(w) for w in sw[1:]) or len(sw) > 9:
                err("%s: script argümanları düz sözcük olmalı (en çok 8; harf/rakam ve . _ = : / + , -)" % name)
                cmd = ""
        effects = a.get("effects")
        if (not isinstance(effects, list) or any(e not in EFFECTS for e in effects)
                or len(set(effects)) != len(effects)):
            err("%s: effects geçersiz (%s)" % (name, "|".join(EFFECTS)))
            effects = []
        for k in ("requires_sudo", "interactive"):
            if k in a and not isinstance(a[k], bool):
                err("%s: %s bool olmalı" % (name, k))
        if "platforms" in a:
            pl = a["platforms"]
            if not isinstance(pl, list) or not pl or len(pl) > 10 or any(not is_str(x) or not RE_PLATFORM.match(x) for x in pl) or len(set(pl)) != len(pl):
                err("%s: platforms 1-10 benzersiz, küçük harfli kimlikten oluşan liste olmalı" % name)
        confirm = a.get("confirm", "none")
        if confirm not in CONFIRMS:
            err("%s: confirm geçersiz (%s)" % (name, "|".join(CONFIRMS)))
            confirm = "none"
        # sshpocket asgari kuralı: paket yalnızca sıkılaştırabilir
        if "destructive" in effects:
            minimum = "type-server-name"
        elif effects:
            minimum = "yes-no"
        else:
            minimum = "none"
        effective = max(confirm, minimum, key=CONFIRMS.index)
        warns = a.get("warnings", [])
        if not isinstance(warns, list) or len(warns) > 20:
            err("%s: warnings liste olmalı" % name)
            warns = []
        for w in warns:
            text_ok(w, "%s: warning" % name)
        act_lines.append("action=%s" % name)
        act_lines.append("action.%s.cmd=%s" % (name, cmd))
        act_lines.append("action.%s.effects=%s" % (name, ",".join(effects)))
        act_lines.append("action.%s.sudo=%s" % (name, "yes" if a.get("requires_sudo") is True else "no"))
        act_lines.append("action.%s.interactive=%s" % (name, "yes" if a.get("interactive") is True else "no"))
        act_lines.append("action.%s.confirm=%s" % (name, effective))
        act_lines.append("action.%s.confirm_raised=%s" % (name, "yes" if effective != confirm else "no"))
        act_lines.append("action.%s.warnings=%d" % (name, len(warns)))
        if isinstance(a.get("platforms"), list) and a["platforms"] and all(is_str(x) for x in a["platforms"]):
            act_lines.append("action.%s.platforms=%s" % (name, ",".join(a["platforms"])))
        for w in warns:
            if is_str(w):
                act_lines.append("action.%s.warning=%s" % (name, w))
        # outputs (§8): dir_var = bildirilmiş 'path' türünde değişken; prefix verilmişse sshpocket'in türettiğiyle BİREBİR aynı olmalı
        outs = a.get("outputs")
        if outs is not None:
            if not isinstance(outs, dict):
                err("%s: outputs nesne olmalı" % name)
            else:
                dv = outs.get("dir_var")
                if dv is not None and runner == "script":
                    err("%s: outputs.dir_var script runner'da desteklenmez (betik sunucuda çalışır; sonuç SSHPOCKET_RESULT satırıyla gelir)" % name)
                elif dv is not None:
                    decl = {}
                    for vv in d.get("vars", []) if isinstance(d.get("vars", []), list) else []:
                        if isinstance(vv, dict) and is_str(vv.get("name")):
                            decl[vv["name"]] = vv.get("type", "string")
                    if not is_str(dv) or decl.get(dv) != "path":
                        err("%s: outputs.dir_var 'path' türünde bildirilmiş bir değişken olmalı" % name)
                    else:
                        act_lines.append("action.%s.outputs.dir_var=%s" % (name, dv))
                fmt = outs.get("format")
                if fmt is not None:
                    if fmt != "keyvalue-line":
                        err("%s: outputs.format yalnızca 'keyvalue-line' olabilir" % name)
                    else:
                        okeys = outs.get("keys")
                        if (not isinstance(okeys, list) or not okeys or len(okeys) > 20
                                or any(not (is_str(k) and RE_OUTKEY.match(k)) for k in okeys)
                                or len(set(okeys)) != len(okeys)):
                            err("%s: outputs.keys geçersiz (küçük harf/rakam/_ ; en çok 20, yinelenmez)" % name)
                        else:
                            act_lines.append("action.%s.outputs.keys=%s" % (name, ",".join(okeys)))
                        want = "SSHPOCKET_RESULT %s.%s" % (d.get("id"), name)
                        if "prefix" in outs and outs["prefix"] != want:
                            err("%s: outputs.prefix '%s' olmalı (sshpocket türetir; paketteki farklıysa reddedilir)" % (name, want))
                        act_lines.append("action.%s.outputs.format=keyvalue-line" % name)
                elif "keys" in outs or "prefix" in outs:
                    err("%s: outputs.keys/prefix için outputs.format gerekir" % name)
                if "summary" in outs and not text_ok(outs["summary"], "%s: outputs.summary" % name):
                    pass
                if "findings" in outs:
                    fnd = outs["findings"]
                    if not (is_str(fnd) and RE_FINDINGS.match(fnd) and ".." not in fnd):
                        err("%s: outputs.findings düz dosya adı olmalı (.tsv; '/' yok; en çok 64 karakter)" % name)
                    else:
                        act_lines.append("action.%s.outputs.findings=%s" % (name, fnd))
    # requires: var olan eylemlere, kendine ve döngüsüz
    graph = {}
    for name, a in actions.items():
        if not isinstance(a, dict):
            continue
        req = a.get("requires", [])
        if not isinstance(req, list) or any(not is_str(r) for r in req):
            err("%s: requires liste olmalı" % name)
            continue
        for r in req:
            if r not in actions:
                err("%s: requires bilinmeyen eylem: %s" % (name, r))
            if r == name:
                err("%s: requires kendini gösteremez" % name)
        graph[name] = [r for r in req if r in actions]
        # ön koşul eylem, bu eylemin göründüğü her platformda da görünmeli (aksi halde eylem hiç çalıştırılamaz)
        mine = a.get("platforms")
        for r in graph[name]:
            theirs = actions[r].get("platforms") if isinstance(actions[r], dict) else None
            if isinstance(theirs, list) and (not isinstance(mine, list) or any(x not in theirs for x in mine)):
                err("%s: requires %s, ama %s bu eylemin platformlarının hepsinde yok" % (name, r, r))
        act_lines.append("action.%s.requires=%s" % (name, ",".join(graph[name])))

    def has_cycle():
        state = {}

        def visit(n):
            state[n] = 1
            for m in graph.get(n, []):
                if state.get(m) == 1 or (state.get(m) is None and visit(m)):
                    return True
            state[n] = 2
            return False
        return any(state.get(n) is None and visit(n) for n in graph)
    if has_cycle():
        err("requires döngüsü var")

    # --- değişkenler ---
    var_lines = []
    seen_vars = set()
    var_specs = {}
    vars_ = d.get("vars", [])
    if not isinstance(vars_, list):
        err("vars liste olmalı")
        vars_ = []
    for v in vars_:
        typed = 1
        if is_str(v):                       # eski biçim: yalnızca ad (üst veri yok, linux-hardening#29)
            v = {"name": v, "type": "string"}
            typed = 0
        if not isinstance(v, dict) or not (is_str(v.get("name")) and RE_NAME.match(v["name"])):
            err("vars: değişken adı geçersiz")
            continue
        n = v["name"]
        if n in seen_vars:
            err("vars: yinelenen değişken: %s" % n)
        seen_vars.add(n)
        t = v.get("type", "string")
        if not var_type_ok(t):
            err("%s: değişken türü geçersiz" % n)
            continue
        if "required" in v and not isinstance(v["required"], bool):
            err("%s: required bool olmalı" % n)
        if "group" in v:
            g = v["group"]
            if not is_str(g) or not g.strip() or len(g) > 40 or RE_TEXT_BAD.search(g):
                err("%s: group 1-40 karakterlik düz metin olmalı" % n)
        if "platforms" in v:
            pl = v["platforms"]
            if not isinstance(pl, list) or not pl or len(pl) > 10 or any(not is_str(x) or not RE_PLATFORM.match(x) for x in pl):
                err("%s: platforms 1-10 küçük harfli kimlikten oluşan liste olmalı" % n)
        if "description" in v:
            text_ok(v["description"], "%s: description" % n, maxlen=VAR_DESC_MAX)
        if t == "enum":
            vals = v.get("values")
            if not isinstance(vals, list) or not vals or any(not is_str(x) or RE_TEXT_BAD.search(x) for x in vals):
                err("%s: enum için values gerekir" % n)
        if t == "int":
            lo, hi = v.get("min"), v.get("max")
            for b, bn in ((lo, "min"), (hi, "max")):
                if b is not None and not is_int(b):
                    err("%s: %s tamsayı olmalı" % (n, bn))
            if is_int(lo) and is_int(hi) and lo > hi:
                err("%s: min > max" % n)
            dv = v.get("default")
            if dv is not None and (not is_int(dv) or (is_int(lo) and dv < lo) or (is_int(hi) and dv > hi)):
                err("%s: default türü/aralığı geçersiz" % n)
        if t == "bool":
            if "default" in v and not isinstance(v["default"], bool):
                err("%s: bool default geçersiz" % n)
            if "warn_if" in v and not isinstance(v["warn_if"], bool):
                err("%s: warn_if bool olmalı" % n)
        var_specs[n] = (t, v)
        var_lines.append("var=%s:%s:%s:%s" % (n, t, "yes" if v.get("required") is True else "no", typed))

    # --- platform_var: değişkenleri platforma göre süzmek için platformu seçen değişkenin adı (isteğe bağlı) ---
    if "platform_var" in d:
        pv = d["platform_var"]
        if not is_str(pv) or pv not in var_specs or var_specs[pv][0] not in ("string", "enum"):
            err("platform_var tanımlı bir string/enum değişkenin adı olmalı")

    # --- eylem düzeyi sabit değişkenler (action.set): runner bunları kullanıcının değerinin ÜSTÜNE yazar ---
    # Amaç: aynı playbook'un güvenli/gerçek koşu eylemleri (örn. *_preview: hardening_pve_apply=false, *_apply: true) kullanıcı değerinden bağımsız olsun.
    for name, a in actions.items():
        if not isinstance(a, dict) or "set" not in a:
            continue
        sv = a["set"]
        if not isinstance(sv, dict) or not sv or len(sv) > 8:
            err("%s: set boş olmayan (en çok 8) nesne olmalı" % name)
            continue
        dir_var = a.get("outputs", {}).get("dir_var") if isinstance(a.get("outputs"), dict) else None
        for k in sorted(sv):
            val = sv[k]
            if k not in var_specs:
                err("%s: set bilinmeyen değişken: %s" % (name, k))
                continue
            if k == dir_var:
                err("%s: set, outputs.dir_var değişkenini (%s) ezemez" % (name, k))
                continue
            vt, vs = var_specs[k]
            ok_val = False
            if vt == "bool":
                ok_val = isinstance(val, bool)
            elif vt == "int":
                ok_val = is_int(val) and not isinstance(val, bool) and (vs.get("min") is None or val >= vs["min"]) and (vs.get("max") is None or val <= vs["max"])
            elif vt == "enum":
                ok_val = is_str(val) and val in vs.get("values", [])
            elif vt in ("string", "path"):
                ok_val = is_str(val) and bool(val) and len(val) <= 200 and not RE_TEXT_BAD.search(val)
            if not ok_val:
                err("%s: set %s için geçersiz değer/tür (yalnızca bool, int, enum, string, path; liste yok)" % (name, k))
                continue
            act_lines.append("action.%s.set=%s=%s" % (name, k, ("true" if val else "false") if isinstance(val, bool) else val))

    # --- sunucu eklerken platform seçimi ve eylem düzeyi zorunlu değişkenler (hepsi isteğe bağlı; yoksa davranış değişmez) ---
    # requires_vars: eylem, listedeki değişken .vars dosyasında boş/yoksa çalışmaz (kapı: ComponentsGates.sh). required'dan farkı: yalnızca o eylemleri kapatır.
    # platform_choices: seçilebilir platformlar; defaults = platforma göre güvenli varsayılanlar (dosyada yoksa yazılır), apply_vars = değişiklik
    #   yapan eylemin onayından SONRA o koşuya özel verilen değerler (apply_warnings zorunlu); onboarding_vars = sunucu eklenirken sorulacak değişkenler.
    plat_lines = []
    if is_str(d.get("platform_var")) and d["platform_var"] in var_specs:
        plat_lines.append("platform_var=%s" % d["platform_var"])

    def _declared(n, what, want_types=None):
        if not is_str(n) or n not in var_specs:
            err("%s: tanımlı bir değişken adı olmalı (%s)" % (what, n if is_str(n) else "?"))
            return False
        if want_types and var_specs[n][0] not in want_types:
            err("%s: %s için tür %s olmalı" % (what, n, "/".join(want_types)))
            return False
        return True

    def _scalar_ok(n, val):
        vt, vs = var_specs[n]
        if vt == "bool":
            return isinstance(val, bool)
        if vt == "enum":
            return is_str(val) and val in vs.get("values", [])
        if vt in ("string", "path"):
            return is_str(val) and bool(val) and len(val) <= 200 and not RE_TEXT_BAD.search(val)
        return False

    for name, a in actions.items():
        if not isinstance(a, dict) or "requires_vars" not in a:
            continue
        rv = a["requires_vars"]
        if not isinstance(rv, list) or not rv or len(rv) > 8 or len(set(map(str, rv))) != len(rv):
            err("%s: requires_vars 1-8 benzersiz değişken adından oluşan liste olmalı" % name)
            continue
        if all(_declared(n, "%s: requires_vars" % name) for n in rv):
            act_lines.append("action.%s.requires_vars=%s" % (name, ",".join(rv)))

    ob = d.get("onboarding_vars")
    if ob is not None:
        if not isinstance(ob, list) or len(ob) > 8 or len(set(map(str, ob))) != len(ob):
            err("onboarding_vars en çok 8 benzersiz değişken adından oluşan liste olmalı")
        else:
            for n in ob:
                if _declared(n, "onboarding_vars"):
                    if "warn_if" in var_specs[n][1]:
                        err("onboarding_vars: %s riskli değer (warn_if) taşıyor; sunucu eklerken sorulamaz" % n)
                    else:
                        plat_lines.append("onboarding_var=%s" % n)

    pc = d.get("platform_choices")
    if pc is not None:
        if not isinstance(pc, list) or not pc or len(pc) > 10:
            err("platform_choices 1-10 öğeli liste olmalı")
            pc = []
        seen_ids = set()
        for i, c in enumerate(pc):
            w = "platform_choices[%d]" % i
            if not isinstance(c, dict) or not is_str(c.get("id")) or not RE_PLATFORM.match(c["id"]) or c["id"] in seen_ids:
                err("%s: id benzersiz, küçük harfli kimlik olmalı" % w)
                continue
            cid = c["id"]
            seen_ids.add(cid)
            if not is_str(c.get("label")) or not c["label"].strip() or len(c["label"]) > 60 or RE_TEXT_BAD.search(c["label"]):
                err("%s: label 1-60 karakterlik düz metin olmalı" % w)
                continue
            if "description" in c:
                text_ok(c["description"], "%s: description" % w, maxlen=VAR_DESC_MAX)
            if "auto_detect" in c and not isinstance(c["auto_detect"], bool):
                err("%s: auto_detect bool olmalı" % w)
            plat_lines.append("platform_choice=%s" % cid)
            plat_lines.append("platform_choice.%s.label=%s" % (cid, c["label"]))
            if is_str(c.get("description")) and c["description"]:
                plat_lines.append("platform_choice.%s.description=%s" % (cid, c["description"]))
            plat_lines.append("platform_choice.%s.auto_detect=%s" % (cid, "no" if c.get("auto_detect") is False else "yes"))
            for key, tag in (("defaults", "default"), ("apply_vars", "apply_var")):
                dv = c.get(key)
                if dv is None:
                    continue
                if not isinstance(dv, dict) or len(dv) > 8:
                    err("%s: %s en çok 8 değişkenli nesne olmalı (boş nesne kabul edilir)" % (w, key))
                    continue
                for k in sorted(dv):
                    if not _declared(k, "%s: %s" % (w, key)):
                        continue
                    if "warn_if" in var_specs[k][1] and key == "defaults":
                        err("%s: defaults %s riskli değer (warn_if) taşıyan değişkeni yazamaz" % (w, k))
                    elif key == "apply_vars" and var_specs[k][0] not in ("bool", "enum"):
                        err("%s: apply_vars %s için tür bool/enum olmalı" % (w, k))
                    elif not _scalar_ok(k, dv[k]):
                        err("%s: %s %s için geçersiz değer/tür" % (w, key, k))
                    else:
                        val = dv[k]
                        plat_lines.append("platform_choice.%s.%s=%s=%s" % (cid, tag, k, ("true" if val else "false") if isinstance(val, bool) else val))
            aw = c.get("apply_warnings")
            if c.get("apply_vars") and (not isinstance(aw, list) or not aw or len(aw) > 10):
                err("%s: apply_vars doluysa apply_warnings (1-10 metin) zorunlu" % w)
            elif aw is not None:
                if not isinstance(aw, list) or len(aw) > 10:
                    err("%s: apply_warnings liste olmalı" % w)
                else:
                    for t in aw:
                        if text_ok(t, "%s: apply_warnings"  % w) is not False and is_str(t):
                            plat_lines.append("platform_choice.%s.apply_warning=%s" % (cid, t))

    # --- invoke.append ---
    inv_lines = []
    inv = d.get("invoke", {})
    if runner == "script" and (inv or d.get("inventory_group") is not None):
        err("invoke ve inventory_group yalnızca ansible runner içindir")
        inv = {}
    app = inv.get("append", []) if isinstance(inv, dict) else None
    if not isinstance(app, list):
        err("invoke.append liste olmalı")
        app = []
    for i, e in enumerate(app):
        if (not isinstance(e, dict) or not is_str(e.get("arg")) or not e["arg"].startswith("-")
                or RE_CMD_BAD.search(e["arg"]) or " " in e["arg"]):
            err("invoke.append[%d]: arg geçersiz" % i)
            continue
        val = e.get("value")
        if val is not None and not (is_str(val) and RE_VALUE.match(val)):
            err("invoke.append[%d]: value geçersiz" % i)
            continue
        for k in ("required", "interactive"):
            if k in e and not isinstance(e[k], bool):
                err("invoke.append[%d]: %s bool olmalı" % (i, k))
        inv_lines.append("invoke=%s|%s|%s|%s" % (
            e["arg"], val if val is not None else "",
            "yes" if e.get("required") is True else "no",
            "yes" if e.get("interactive") is True else "no"))

    if errors:
        for m in errors:
            print(m, file=sys.stderr)
        return 1
    out = ["id=%s" % pid, "version=%s" % ver, "sshpocket_api=%d" % api, "runner=%s" % runner]
    if group:
        out.append("inventory_group=%s" % group)
    if req_py:
        out.append("requires.python=%s" % req_py)
    out += prof_lines + act_lines + var_lines + plat_lines + inv_lines
    print("\n".join(out))
    return 0


sys.exit(main())
