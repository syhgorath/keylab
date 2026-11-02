#!/usr/bin/env bats
load test_helper

setup() {
    load_helpers
    # load_helpers gerçek tuş okuyucuyu yükler; sahte kuyrukla değiştir (pty gerekmez)
    _ui_read_key() { UI_KEY="${KEYQ[$KEYI]:-EOF}"; KEYI=$((KEYI + 1)); }
}

@test "ui: boru/test çıktısında renk kodu yok (düz metin)" {
    run ui_header "Ana menü › x"
    [[ "$output" == *"Ana menü › x"* ]]
    [[ "$output" != *$'\033'* ]]
    run ui_footer "ipucu"
    [[ "$output" == *"ipucu"* ]]
    [[ "$output" != *$'\033'* ]]
}

@test "ui: grup başlığı çevrilir" {
    [ "$(ui_group_title MainMenu)" = "Ana menü" ]
    [ "$(ui_group_title Diger)" = "Diger" ]
}

@test "ui: server_summary kullanıcı@ip[:port] verir, eksik/geçersizde boş" {
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/srv"
    mkdir -p "$SSHPOCKET_SERVERS_DIR"/{alpha,beta,gamma}
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=2222\n' > "$SSHPOCKET_SERVERS_DIR/alpha/server.env"
    printf 'IP=192.0.2.8\nUSER=root\n' > "$SSHPOCKET_SERVERS_DIR/beta/server.env"
    printf 'IP=$(touch x)\nUSER=root\n' > "$SSHPOCKET_SERVERS_DIR/gamma/server.env"
    [ "$(server_summary alpha)" = "deploy@192.0.2.7:2222" ]
    [ "$(server_summary beta)" = "root@192.0.2.8" ]
    [ -z "$(server_summary gamma)" ]
    [ -z "$(server_summary yok)" ]
}

@test "ui: ana menüde başlık, sunucu özeti ve alt ipucu görünür" {
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    chmod +x "$SANDBOX/bin/"*
    mkdir -p "$SANDBOX/servers/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$SANDBOX/servers/alpha/server.env"
    run bash -c "printf '0\n' | TERM=xterm PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Ana menü"* ]]
    [[ "$output" == *"deploy@192.0.2.7"* ]]
    [[ "$output" == *"0 veya q = çıkış"* ]]
}

@test "ui: modül menüsünde ekmek kırıntısı ve geri ipucu görünür" {
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    chmod +x "$SANDBOX/bin/"*
    mkdir -p "$SANDBOX/servers/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$SANDBOX/servers/alpha/server.env"
    run bash -c "printf '1\n0\n\n0\n' | TERM=xterm PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Ana menü › ALPHA"* ]]
    [[ "$output" == *"0 veya q = geri"* ]]
}

# ---------- ok tuşlu seçim (sahte tuş kuyruğu, pty gerekmez) ----------
feed() { KEYQ=("$@"); KEYI=0; }
pick() { ui_pick_arrows "$@" 2>/dev/null; }

@test "ok tuşu: Enter başlangıç satırını seçer" {
    feed ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "1" ]
}

@test "ok tuşu: aşağı/yukarı hareket eder, sarar" {
    feed DOWN DOWN ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "3" ]
    feed UP ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "0" ]          # yukarı sararak Geri satırına gider
    feed DOWN DOWN DOWN DOWN ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "1" ]          # aşağı sararak başa döner
}

@test "ok tuşu: j/k hareket eder" {
    feed CHAR:j CHAR:j CHAR:k ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "2" ]
}

@test "ok tuşu: q, Q, sol ok, ESC ve EOF geri (0) verir" {
    for k in CHAR:q CHAR:Q LEFT ESC; do
        feed DOWN "$k"
        run pick 1 "Geri" "A" "B"
        [ "$output" = "0" ]
    done
    feed
    run pick 1 "Geri" "A" "B"
    [ "$output" = "0" ]
}

@test "ok tuşu: numara yazıp Enter doğrudan seçer; 0 geri; geçersiz yok sayılır" {
    feed CHAR:2 ENTER
    run pick 1 "Geri" "A" "B" "C"
    [ "$output" = "2" ]
    feed CHAR:0 ENTER
    run pick 2 "Geri" "A" "B" "C"
    [ "$output" = "0" ]
    feed CHAR:9 ENTER ENTER                # 9 yok → temizlenir, ikinci Enter başlangıç satırını seçer
    run pick 3 "Geri" "A" "B" "C"
    [ "$output" = "3" ]
}

@test "ok tuşu: iki haneli numara ve Backspace" {
    items=(); for i in 1 2 3 4 5 6 7 8 9 10 11 12; do items+=("S$i"); done
    feed CHAR:1 CHAR:2 ENTER
    run pick 1 "Geri" "${items[@]}"
    [ "$output" = "12" ]
    feed CHAR:1 CHAR:2 BACKSPACE CHAR:0 ENTER
    run pick 1 "Geri" "${items[@]}"
    [ "$output" = "10" ]
}

@test "ok tuşu: imleç gizlenir ve her çıkışta geri açılır" {
    feed ENTER
    out=$(ui_pick_arrows 1 "Geri" "A" "B" 2>&1 >/dev/null)
    [[ "$out" == *$'\033[?25l'* ]]
    [[ "$out" == *$'\033[?25h'* ]]
}

@test "ok tuşu: çizim seçili satırı vurgular" {
    feed DOWN ENTER
    out=$(ui_pick_arrows 1 "Geri" "AAA" "BBB" 2>&1 >/dev/null)
    [[ "$out" == *$'\033[7m'*"BBB"* ]]
}

@test "ok tuşu: UI ayarı numbers ise etkin değil, arrows ise zorla etkin" {
    export SSHPOCKET_CONF="$BATS_TEST_TMPDIR/c.conf"
    SSHPOCKET_UI=numbers run ui_arrows_enabled
    [ "$status" -ne 0 ]
    SSHPOCKET_UI=arrows run ui_arrows_enabled
    [ "$status" -eq 0 ]
    SSHPOCKET_UI=auto run ui_arrows_enabled     # bats içinde terminal yok
    [ "$status" -ne 0 ]
}

@test "ok tuşu: Menu ok modunda seçilen modül fonksiyonunu döndürür" {
    source "$H/RegisterModules.sh"; source "$H/Menu.sh"
    RegisterModules "ONE" "MainMenu" "OneModule"
    RegisterModules "TWO" "MainMenu" "TwoModule"
    export SSHPOCKET_UI=arrows
    feed DOWN ENTER
    out=$(Menu "MainMenu" 2>/dev/null)
    [ "$out" = "TwoModule" ]
    feed CHAR:q
    out=$(Menu "MainMenu" 2>/dev/null)
    [ "$out" = "exit" ]
}

@test "ok tuşu (gerçek okuyucu): ESC dizileri çözülür, menüler ok modunda çalışır" {
    unset -f _ui_read_key
    source "$H/UI.sh"
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    chmod +x "$SANDBOX/bin/"*
    mkdir -p "$SANDBOX/servers/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$SANDBOX/servers/alpha/server.env"
    # ↓ ↑ (başa dön) Enter → ALPHA menüsü; o menüde q → geri; Enter (devam); ana menüde q → çıkış
    run bash -c "printf '\033[B\033[A\nq\n\nq' | SSHPOCKET_UI=arrows SSHPOCKET_STATUS=off TERM=xterm PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Ana menü › ALPHA"* ]]
    [[ "$output" == *"Exiting"* ]]
}

@test "ok tuşu (gerçek okuyucu): Ayarlar paneli ok modunda ayarı çevirir ve geri döner" {
    unset -f _ui_read_key
    source "$H/UI.sh"
    export SSHPOCKET_CONF="$BATS_TEST_TMPDIR/c.conf"
    export SISTEM_PATH="$ROOT/sistem"
    # Enter → 1. ayar (STATUS on→off); sonra q
    SSHPOCKET_UI=arrows run bash -c 'source "'"$H"'/Settings.sh"; source "'"$H"'/UI.sh"; clear(){ :; }; SettingsAction' <<< $'\nq'
    grep -q '^STATUS=off$' "$SSHPOCKET_CONF"
}

@test "ok tuşu (gerçek okuyucu): Sunucuları yönet ok modunda açılır, Sil seçilip ad boş bırakılırsa hiçbir şey silinmez" {
    unset -f _ui_read_key
    source "$H/UI.sh"
    make_sandbox
    mkdir -p "$SANDBOX/servers/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$SANDBOX/servers/alpha/server.env"
    run bash -c "export SSHPOCKET_UI=arrows SISTEM_PATH='$SANDBOX/sistem' TERM=xterm; \
        source '$SANDBOX/sistem/Helpers/Settings.sh'; source '$SANDBOX/sistem/Helpers/UI.sh'; source '$SANDBOX/sistem/Helpers/BuiltinMenu.sh' >/dev/null 2>&1; \
        ReloadModules(){ :; }; clear(){ :; }; \
        ManageServersAction" <<< $'\n\n'
    [[ "$output" == *"Ana menü › Sunucuları yönet"* ]]
    [[ "$output" == *$'\033[7m'*"Sil"* ]]          # ok moduna özgü vurgulu satır
    [ -f "$SANDBOX/servers/alpha/server.env" ]
}

# ---------- fzf arka ucu ve açıklamalar ----------
# Sahte fzf: argümanları ve stdin'i kaydeder; FZF_FAKE_PICK satır numarasını (1. alan) seçer; FZF_FAKE_RC çıkış kodu
make_fake_fzf() {
    mkdir -p "$BATS_TEST_TMPDIR/fbin"
    cat > "$BATS_TEST_TMPDIR/fbin/fzf" <<STUB
#!/bin/bash
[ "\$1" = "--version" ] && { echo "\${FZF_FAKE_VERSION:-0.0.1 (fake)}"; exit 0; }
printf '%s\n' "\$@" > "$BATS_TEST_TMPDIR/fzf.args"
cat > "$BATS_TEST_TMPDIR/fzf.stdin"
for a in "\$@"; do case "\$a" in --expect=*) echo "\${FZF_FAKE_KEY:-}";; esac; done
# --preview değerinden açıklama dosyası yolunu çıkarıp sakla
prev=""; while [ \$# -gt 0 ]; do [ "\$1" = "--preview" ] && prev="\$2"; shift; done
f=\$(printf '%s' "\$prev" | grep -oE "'[^']+'\$" | tr -d "'")
[ -f "\$f" ] && cp "\$f" "$BATS_TEST_TMPDIR/fzf.desc"
if [ "\${FZF_FAKE_RC:-0}" -ne 0 ]; then
  [ "\${FZF_FAKE_PRINT:-0}" = 1 ] && awk -F'\t' -v n="\${FZF_FAKE_PICK:-1}" '\$1==n {print; exit}' "$BATS_TEST_TMPDIR/fzf.stdin"
  exit "\$FZF_FAKE_RC"
fi
awk -F'\t' -v n="\${FZF_FAKE_PICK:-1}" '\$1==n {print; exit}' "$BATS_TEST_TMPDIR/fzf.stdin"
STUB
    chmod +x "$BATS_TEST_TMPDIR/fbin/fzf"
    export PATH="$BATS_TEST_TMPDIR/fbin:$PATH"
}

@test "fzf: UI=fzf iken fzf arka ucu seçilir; seçilen numara döner" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=2
    run ui_pick_arrows 1 "Geri" "Bir" "İki" "Üç"
    [ "$output" = "2" ]
    # liste: numara<TAB>etiket, geri = 0
    [ "$(cat "$BATS_TEST_TMPDIR/fzf.stdin")" = $'1\tBir\n2\tİki\n3\tÜç\n0\tGeri' ]
}

@test "fzf: Esc/iptal (çıkış 130) ve eşleşmeme (1) geri (0) sayılır; geçersiz numara 0" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=fzf
    FZF_FAKE_RC=130 run ui_pick_arrows 1 "Geri" "A" "B"; [ "$output" = "0" ]
    FZF_FAKE_RC=1 run ui_pick_arrows 1 "Geri" "A" "B"; [ "$output" = "0" ]
    FZF_FAKE_PICK=9 run ui_pick_arrows 1 "Geri" "A" "B"; [ "$output" = "0" ]
}

@test "fzf: açıklamalar önizleme dosyasına satır satır yazılır (geri açıklaması ilk satır)" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=fzf
    UI_PICK_DESCS=("Birinci açıklama" "İkinci; \$(touch /tmp/pwned) \`id\`")
    UI_PICK_BACK_DESC="Geri açıklaması"
    run ui_pick_arrows 1 "Geri" "A" "B"
    [ "$(cat "$BATS_TEST_TMPDIR/fzf.desc")" = $'Geri açıklaması\nBirinci açıklama\nİkinci; $(touch /tmp/pwned) `id`' ]
    [ ! -e /tmp/pwned ]
    grep -q -- '--preview' "$BATS_TEST_TMPDIR/fzf.args"
    # açıklama metni komut satırına GİRMEZ (yalnızca dosya yolu)
    run ! grep -q 'Birinci açıklama' "$BATS_TEST_TMPDIR/fzf.args"
}

@test "fzf: UI=numbers/arrows iken fzf kullanılmaz; fzf yoksa kendi seçicimize düşer" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=numbers
    run ui_fzf_enabled; [ "$status" -ne 0 ]
    export SSHPOCKET_UI=arrows
    run ui_fzf_enabled; [ "$status" -ne 0 ]
    export SSHPOCKET_UI=fzf
    run ui_fzf_enabled; [ "$status" -eq 0 ]
    rm "$BATS_TEST_TMPDIR/fbin/fzf"
    SSHPOCKET_FZF_DIRS="$BATS_TEST_TMPDIR/yok" PATH="/usr/bin:/bin" run ui_fzf_enabled; [ "$status" -ne 0 ]
}

@test "fzf: auto modda terminal yoksa (boru/test) fzf KULLANILMAZ" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=auto
    run ui_fzf_enabled
    [ "$status" -ne 0 ]
}

@test "fzf: etiketteki TAB/satır sonu liste biçimini bozmaz" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=1
    run ui_pick_arrows 1 "Geri" $'A\tB' $'C\nD'
    [ "$(cat "$BATS_TEST_TMPDIR/fzf.stdin")" = $'1\tA B\n2\tC D\n0\tGeri' ]
}

@test "açıklamalar: ok tuşlu seçici seçili satırın açıklamasını alt satırda gösterir" {
    UI_PICK_DESCS=("AÇIKLAMA-BİR" "AÇIKLAMA-İKİ")
    UI_PICK_BACK_DESC="AÇIKLAMA-GERİ"
    feed DOWN DOWN ENTER
    out=$(SSHPOCKET_UI=arrows ui_pick_arrows 1 "Geri" "A" "B" 2>&1 >/dev/null)
    [[ "$out" == *"AÇIKLAMA-BİR"* ]]
    [[ "$out" == *"AÇIKLAMA-İKİ"* ]]
    [[ "$out" == *"AÇIKLAMA-GERİ"* ]]
}

@test "açıklamalar: ana menü girdileri için açıklama tanımlı; bilinmeyen girdi boş" {
    local f
    for f in server:alpha AddServerAction ImportSSHConfigAction ManageServersAction UpdateAction SettingsAction ComponentsAction; do
        [ -n "$(ui_describe_entry "$f")" ] || { echo "AÇIKLAMA YOK: $f"; return 1; }
    done
    [ -z "$(ui_describe_entry YokBoyleBirSey)" ]
}

@test "ayar: UI=fzf kabul edilir ve panel döngüsü auto→numbers→arrows→fzf→auto" {
    export SSHPOCKET_CONF="$BATS_TEST_TMPDIR/c.conf"
    export SISTEM_PATH="$ROOT/sistem"
    local want cur=auto
    for want in numbers arrows fzf auto; do
        run bash -c 'source "'"$H"'/Settings.sh"; clear(){ :; }; SettingsAction' <<< $'3\nq'
        [ "$(grep '^UI=' "$SSHPOCKET_CONF" | tail -n1)" = "UI=$want" ]
    done
}

@test "fzf: hata kodu ile çıkan fzf çıktı bassa bile seçim sayılmaz" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=2 FZF_FAKE_PRINT=1
    FZF_FAKE_RC=130 run ui_pick_arrows 1 "Geri" "A" "B"
    [ "$output" = "0" ]
    FZF_FAKE_RC=2 run ui_pick_arrows 1 "Geri" "A" "B"
    [ "$output" = "0" ]
}

@test "fzf (gerçek pty): terminalde auto → fzf, numbers → fzf değil" {
    unset -f _ui_read_key
    make_fake_fzf
    run python3 -I - "$H" "$BATS_TEST_TMPDIR/fbin" <<'PY'
import os, pty, sys, select
H, fbin = sys.argv[1], sys.argv[2]
def run(mode):
    pid, fd = pty.fork()
    if pid == 0:
        env = {"PATH": fbin + ":/usr/bin:/bin", "TERM": "xterm", "SSHPOCKET_UI": mode, "SISTEM_PATH": H + "/.."}
        os.execve("/bin/bash", ["bash", "-c",
            'source "%s/Settings.sh"; source "%s/UI.sh"; if ui_fzf_enabled; then echo FZF=yes; else echo FZF=no; fi' % (H, H)], env)
    out = b""
    while True:
        r, _, _ = select.select([fd], [], [], 5)
        if not r: break
        try: d = os.read(fd, 4096)
        except OSError: break
        if not d: break
        out += d
    os.waitpid(pid, 0)
    return out.decode()
print("auto:" + run("auto").strip())
print("numbers:" + run("numbers").strip())
print("arrows:" + run("arrows").strip())
PY
    [[ "$output" == *"auto:FZF=yes"* ]]
    [[ "$output" == *"numbers:FZF=no"* ]]
    [[ "$output" == *"arrows:FZF=no"* ]]
}

@test "fzf: PATH'te yoksa Homebrew konumlarında aranır (Finder/start.command PATH'i dardır)" {
    unset -f _ui_read_key; source "$H/UI.sh"
    make_fake_fzf
    mkdir -p "$BATS_TEST_TMPDIR/hb"
    mv "$BATS_TEST_TMPDIR/fbin/fzf" "$BATS_TEST_TMPDIR/hb/fzf"
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=2
    PATH="/usr/bin:/bin" SSHPOCKET_FZF_DIRS="$BATS_TEST_TMPDIR/hb" run ui_fzf_bin
    [ "$output" = "$BATS_TEST_TMPDIR/hb/fzf" ]
    PATH="/usr/bin:/bin" SSHPOCKET_FZF_DIRS="$BATS_TEST_TMPDIR/hb" run ui_pick_arrows 1 "Geri" "A" "B"
    [ "$output" = "2" ]
    PATH="/usr/bin:/bin" SSHPOCKET_FZF_DIRS="$BATS_TEST_TMPDIR/yok" run ui_fzf_bin
    [ "$status" -ne 0 ]
}

# Dar terminalde uzun açıklama satırı sarılıp imleç hesabını kaydırırdı: her ok tuşunda listenin üst satırı çoğalıyordu.
@test "ok tuşlu seçici (gerçek pty, 60/80/100 sütun): ok tuşlarında satırlar ÇOĞALMAZ" {
    unset -f _ui_read_key
    run python3 -I - "$H" <<'PY'
import os, pty, sys, select, time, re, fcntl, termios, struct, unicodedata
H = sys.argv[1]
def play(cols):
    pid, fd = pty.fork()
    if pid == 0:
        env = {"PATH": "/usr/bin:/bin", "TERM": "xterm-256color", "SSHPOCKET_UI": "arrows", "SISTEM_PATH": H + "/..",
               "LANG": "en_US.UTF-8", "LC_ALL": "en_US.UTF-8"}
        script = r'''
source "%s/Settings.sh"; source "%s/UI.sh"
UI_PICK_DESCS=("Sunucu menüsünü açar: bağlan, key gönder, bilgileri güncelle, Keychain passphrase, knock ayarları ve daha fazlası." "İkinci açıklama da oldukça uzun bir metin, dar terminalde sarılırdı ve imleç hesabını bozardı bu satır." "Üçüncü")
UI_PICK_BACK_DESC="Uygulamadan çıkar."
r=$(ui_pick_arrows 1 "Çıkış" "BIRINCI-SATIR   kullanici@10.0.0.5" "IKINCI-SATIR" "UCUNCU-SATIR")
echo "RESULT=[$r]"
''' % (H, H)
        os.execve("/bin/bash", ["bash", "-c", script], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 30, cols, 0, 0))
    def rd(t):
        out = b""; end = time.time() + t
        while time.time() < end:
            r, _, _ = select.select([fd], [], [], 0.2)
            if r:
                try: out += os.read(fd, 65536)
                except OSError: break
        return out
    log = rd(1.0)
    for k in [b"\x1b[B", b"\x1b[B", b"\x1b[A", b"\x1b[B", b"\x1b[B"]:
        os.write(fd, k); log += rd(0.4)
    os.write(fd, b"\r"); log += rd(0.8)
    txt = log.decode(errors="replace")
    def width(ch):
        if unicodedata.combining(ch) or ch in "\ufe0f\u200d": return 0
        return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1
    screen = [[]]; row = col = 0; wrap = True
    def ensure(r):
        while len(screen) <= r: screen.append([])
    i = 0
    while i < len(txt):
        c = txt[i]
        if c == "\x1b":
            m = re.match(r"\x1b\[([0-9;?]*)([A-Za-z])", txt[i:])
            if m:
                arg, cmd = m.group(1), m.group(2)
                if arg == "?7" and cmd in "lh": wrap = (cmd == "h")
                elif cmd == "A": row = max(0, row - int(arg or 1))
                elif cmd == "K": ensure(row); screen[row] = []; col = 0
                i += m.end(); continue
        if c == "\n": row += 1; col = 0; ensure(row)
        elif c == "\r": col = 0
        else:
            w = width(c)
            if col + w > cols:
                if wrap: row += 1; col = 0; ensure(row)
                else: i += 1; continue
            ensure(row); screen[row].append(c); col += w
        i += 1
    text = "\n".join("".join(l) for l in screen)
    return {k: text.count(k) for k in ("BIRINCI-SATIR", "IKINCI-SATIR", "UCUNCU-SATIR")}, re.search(r"RESULT=\[([^\]]*)\]", re.sub(r"\x1b\[[0-9;?]*[A-Za-z]", "", txt))
bad = False
for cols in (100, 80, 60):
    counts, res = play(cols)
    print(cols, counts, res.group(1) if res else "?")
    if any(v != 1 for v in counts.values()): bad = True
print("COGALMA=" + ("EVET" if bad else "HAYIR"))
PY
    [[ "$output" == *"COGALMA=HAYIR"* ]]
}


@test "fzf sürümü: 0.74.5 → 7405; okunamazsa 0" {
    make_fake_fzf
    FZF_FAKE_VERSION="0.74.5 (Homebrew)" run ui_fzf_version "$BATS_TEST_TMPDIR/fbin/fzf"
    [ "$output" = "7405" ]
    FZF_FAKE_VERSION="0.58.0" run ui_fzf_version "$BATS_TEST_TMPDIR/fbin/fzf"
    [ "$output" = "5800" ]
    run ui_fzf_version "$BATS_TEST_TMPDIR/yok"
    [ "$output" = "0" ]
}

@test "fzf geniş düzen: sağda önizleme, yol çubuğu kenarlık etiketi, başlıkta özet ve kısayol ipucu" {
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=2 FZF_FAKE_VERSION="0.74.5 (t)"
    UI_PICK_WIDE=1 UI_PICK_TITLE="Sunucu › s1 › Değişkenler" UI_PICK_HEADER="Grup: Tümü · 2/2" UI_PICK_EXPECT_HINT="Ctrl-O: grup seç" \
        run ui_pick_arrows 1 Geri "Bir" "İki"
    [ "$output" = "2" ]
    grep -q -- "right,48%,wrap" "$BATS_TEST_TMPDIR/fzf.args"
    grep -qxF -- " Sunucu › s1 › Değişkenler " <(sed -n '/^--border-label=/{s/^--border-label=//;p;}' "$BATS_TEST_TMPDIR/fzf.args")
    grep -q -- "--list-border=rounded" "$BATS_TEST_TMPDIR/fzf.args"
    grep -q "Grup: Tümü · 2/2" "$BATS_TEST_TMPDIR/fzf.args"
    grep -q "Ctrl-O: grup seç" "$BATS_TEST_TMPDIR/fzf.args"
}

@test "fzf geniş düzen: eski fzf'te etiketli çerçeveler kullanılmaz (yalnız sağ önizleme)" {
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=1 FZF_FAKE_VERSION="0.30.0"
    UI_PICK_WIDE=1 UI_PICK_TITLE="Yol" run ui_pick_arrows 1 Geri "Bir"
    [ "$output" = "1" ]
    grep -q -- "right,48%,wrap" "$BATS_TEST_TMPDIR/fzf.args"
    run ! grep -q -- "--border-label\|--list-border\|--input-border" "$BATS_TEST_TMPDIR/fzf.args"
}

@test "fzf kısayol: UI_PICK_EXPECT ile basılan tuş 'key:<ad>' döner; Enter'da seçilen numara" {
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=2
    UI_PICK_EXPECT=ctrl-o FZF_FAKE_KEY=ctrl-o run ui_pick_arrows 1 Geri "Bir" "İki"
    [ "$output" = "key:ctrl-o" ]
    grep -qxF -- "--expect=ctrl-o" "$BATS_TEST_TMPDIR/fzf.args"
    UI_PICK_EXPECT=ctrl-o FZF_FAKE_KEY="" run ui_pick_arrows 1 Geri "Bir" "İki"
    [ "$output" = "2" ]
}

@test "fzf varsayılan düzen değişmedi: UI_PICK_WIDE yokken altta önizleme ve --border" {
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=1 FZF_FAKE_VERSION="0.74.5 (t)"
    run ui_pick_arrows 1 Geri "Bir"
    grep -q -- "down,3,wrap,border-top" "$BATS_TEST_TMPDIR/fzf.args"
    run ! grep -q -- "--border-label\|right,48%" "$BATS_TEST_TMPDIR/fzf.args"
}


@test "fzf kart: çok satırlı açıklama \\037 ile tek satırda saklanır; ok tuşlu seçicide tek satıra indirilir" {
    make_fake_fzf
    export SSHPOCKET_UI=fzf FZF_FAKE_PICK=1
    UI_PICK_DESCS=($'Birinci\nİkinci\nÜçüncü')
    run ui_pick_arrows 1 Geri "Bir"
    [ "$(sed -n 2p "$BATS_TEST_TMPDIR/fzf.desc")" = $'Birinci\037İkinci\037Üçüncü' ]
    grep -q -- "gsub" "$BATS_TEST_TMPDIR/fzf.args"
}


# ---------- bölgeli ekran (ui_region_*) ve genel liste editörü (ui_edit_list): paketten bağımsız ----------
@test "ui_region: kaydırma bölgesi kurulur, alt çerçeve sonuçla güncellenir, sıfırlanınca bölge kalkar" {
    export SSHPOCKET_ROWS=30 SSHPOCKET_COLS=80
    clear() { :; }
    f() { ui_region_begin "Başlık" $'satır bir\nsatır iki' "Etiket" && ui_region_end "Bitti" && ui_region_reset; }
    run f
    [ "$status" -eq 0 ]
    [[ "$output" == *$'\033[13;30r'* ]] || false                      # üst=30*4/10=12 → bölge 13..30
    [[ "$output" == *"Başlık"* && "$output" == *"satır bir"* && "$output" == *"satır iki"* ]] || false
    [[ "$output" == *"Etiket"* && "$output" == *"Bitti"* ]] || false
    [[ "$output" == *$'\033[r'* ]] || false
}

@test "ui_region: küçük ya da bilinmeyen terminalde kurulmaz (1); sıfırlama kurulmamışken sessiz" {
    clear() { :; }
    SSHPOCKET_ROWS=20 SSHPOCKET_COLS=80 run ui_region_begin "T" "g" "E"
    [ "$status" -eq 1 ]
    SSHPOCKET_ROWS=30 SSHPOCKET_COLS=40 run ui_region_begin "T" "g" "E"
    [ "$status" -eq 1 ]
    SSHPOCKET_ROWS=abc SSHPOCKET_COLS=80 run ui_region_begin "T" "g" "E"
    [ "$status" -eq 1 ]
    run ui_region_reset
    [ "$status" -eq 0 ]; [ -z "$output" ]
}

@test "ui_wrap_text: sözcük sınırından sarar, devam satırı girintili; uzun sözcük kesilir; boş satır korunur" {
    out=$(ui_wrap_text 12 $'aaa bbb ccc ddd\n\nxxxxxxxxxxxxxxx')
    [ "$out" = $'aaa bbb ccc\n   ddd\n\nxxxxxxxxxxxx\n   xxx' ] || { printf '%s\n' "$out"; false; }
}

@test "ui_region_begin2: geniş terminalde (254x66) iki sütun tam genişlikte çizilir; bölge içeriğe göre başlar" {
    export SSHPOCKET_ROWS=66 SSHPOCKET_COLS=254
    clear() { :; }
    f() { ui_region_begin2 "Başlık" $'sol bir\nsol iki' "sağ bir" "Eylem" "Açıklama" "Çıktı"; }
    run f
    [ "$status" -eq 0 ]
    [[ "$output" == *$'\033[10;66r'* ]] || { echo "$output" | cat -v | head -c 600; false; }    # içerik 2 satır → alt sınır 9 → bölge 10..66
    [[ "$output" == *"Eylem"* && "$output" == *"Açıklama"* && "$output" == *"sol iki"* && "$output" == *"sağ bir"* && "$output" == *"│"* ]] || false
    [[ "$output" == *"$(printf '─%.0s' $(seq 1 250))"* ]] || false                                   # çizgi 254 sütuna yayılır
    [[ "$output" == *$'\033[3;116H'* ]] || false                                                     # sağ sütun: 254*45/100=114 → sütun 116
}

@test "ui_region_begin2: dar terminalde (80x24) sütun sığar, yükseklik terminalin %40'ını aşmaz; küçükte 1 döner" {
    export SSHPOCKET_ROWS=24 SSHPOCKET_COLS=80
    clear() { :; }
    f() { ui_region_begin2 "T" "$(seq 1 30)" "$(seq 1 30)" "Sol" "Sağ" "Çıktı"; }
    run f
    [ "$status" -eq 0 ]
    [[ "$output" == *$'\033[10;24r'* ]] || { echo "$output" | cat -v | head -c 400; false; }      # 24*4/10=9 → bölge 10..24
    SSHPOCKET_ROWS=20 SSHPOCKET_COLS=80 run ui_region_begin2 "T" "a" "b" "L" "R" "E"
    [ "$status" -eq 1 ]
    SSHPOCKET_ROWS=30 SSHPOCKET_COLS=50 run ui_region_begin2 "T" "a" "b" "L" "R" "E"
    [ "$status" -eq 1 ]
}

# Motor testleri: ui_pick sahte seçiciyle (kuyruk); satırlar: öğeler, ➕ Ekle, (ek satırlar), 🧹 Temizle (öğe varsa), 💾 Kaydet
liste_motoru() {
    : > "$BATS_TEST_TMPDIR/q"; for x in "$@"; do echo "$x" >> "$BATS_TEST_TMPDIR/q"; done
    clear() { :; }
    ui_arrows_enabled() { return 0; }
    ui_pick_arrows() { local a; a=$(head -n 1 "$BATS_TEST_TMPDIR/q"); sed -i.bak '1d' "$BATS_TEST_TMPDIR/q"; rm -f "$BATS_TEST_TMPDIR/q.bak"; printf '%s' "${a:-0}"; }
    sayi_dogrula() { [[ "$1" =~ ^[0-9]+$ ]] || { echo "sayı olmalı"; return 1; }; }
}

@test "ui_edit_list: ekle + kaydet (0) son listeyi verir; geçersiz öğe reddedilir ve neden gösterilir" {
    liste_motoru 3 6
    f() { UI_LIST_ITEMS=(1 2); UI_LIST_VALIDATE=sayi_dogrula; ui_edit_list; local rc=$?; echo "rc=$rc items=${UI_LIST_ITEMS[*]} dirty=$UI_LIST_DIRTY"; }
    run f <<<"7"
    [[ "$output" == *"rc=0 items=1 2 7 dirty=1"* ]] || false
    liste_motoru 3 3 6
    run f <<<$'abc\n9'
    [[ "$output" == *"sayı olmalı"* ]] || false
    [[ "$output" == *"items=1 2 9"* ]] || false
}

@test "ui_edit_list: Vazgeç (1) listeyi kaydetmez; zorunlu boş liste kaydedilemez; EOF (2)" {
    liste_motoru 0
    f() { UI_LIST_ITEMS=(1); UI_LIST_VALIDATE=sayi_dogrula; UI_LIST_REQUIRED=${REQ:-no}; ui_edit_list; echo "rc=$?"; }
    run f
    [[ "$output" == *"rc=1"* ]] || false
    liste_motoru 3 2 0                                   # tek öğe: •=1 Ekle=2 Temizle=3 Kaydet=4 → 3=Temizle, boş: Ekle=1 Kaydet=2
    liste_motoru 3 2 0                                   # (aynı kuyruk) Temizle → Kaydet(zorunlu boş) → Vazgeç
    REQ=yes run f
    [[ "$output" == *"Liste boş olamaz"* ]] || false
    liste_motoru 2
    run f </dev/null                                      # Ekle → istem EOF
    [[ "$output" == *"rc=1"* || "$output" == *"rc=2"* ]] || false
}

@test "ui_edit_list: ek satırlar, ekleme ve kaydetme geri çağrımları paketten bağımsız çalışır" {
    liste_motoru 2 4
    ek_satir() { UI_LIST_X_LABELS+=("✚ ekstra"); UI_LIST_X_ACTS+=(ex1); UI_LIST_X_DESCS+=("ek açıklama"); }
    ek_calistir() { UI_LIST_MSG="ek eylem çalıştı: $1"; UI_LIST_ITEMS+=("eklenen"); UI_LIST_DIRTY=1; }
    f() { UI_LIST_ITEMS=(); UI_LIST_VALIDATE=true; UI_LIST_EXTRAS=ek_satir; UI_LIST_EXTRA_RUN=ek_calistir; ui_edit_list; echo "rc=$? items=${UI_LIST_ITEMS[*]}"; }
    # boş liste satırları: Ekle=1 ✚ ekstra=2 Kaydet=3 → 2 = ek eylem; sonra tek öğe: •=1 Ekle=2 ✚=3 Temizle=4 Kaydet=5
    liste_motoru 2 5
    run f
    [[ "$output" == *"rc=0 items=eklenen"* ]] || false
    [[ "$output" == *"ek eylem çalıştı: ex1"* ]] || false
    # kaydetme denetimi 1 dönerse düzenlemeye döner
    ret1() { return 1; }
    g() { UI_LIST_ITEMS=(a); UI_LIST_VALIDATE=true; UI_LIST_SAVE_CHECK=ret1; ui_edit_list; echo "rc=$?"; }
    liste_motoru 4 0                                      # Kaydet(=4: •=1 Ekle=2 Temizle=3 Kaydet=4) → denetim reddeder → Vazgeç
    run g
    [[ "$output" == *"rc=1"* ]] || false
}
