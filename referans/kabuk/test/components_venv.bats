#!/usr/bin/env bats
# Aşama 2.3: Python seçimi, hash'li requirements, venv önbelleği (çevrimdışı; ağ yok), --doctor
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsVenv.sh"
    source "$H/Doctor.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_CACHE_DIR="$T/cache"
    export SSHPOCKET_OFFLINE=1
}

# fake_python <ad> <sürüm> → $T/bin/<ad> (yalnızca sürüm bildirir)
fake_python() {
    mkdir -p "$T/bin"
    printf '#!/bin/sh\ncase "$*" in *version_info*) echo %s ;; *) exit 1 ;; esac\n' "$2" > "$T/bin/$1"
    chmod +x "$T/bin/$1"
}

# make_wheel → $T/wh/demo_pkg-1.0-py3-none-any.whl ve $T/req/requirements.txt (hash'li)
make_wheel() {
    mkdir -p "$T/wh" "$T/req"
    python3 -I - "$T" <<'PY'
import zipfile, hashlib, base64, sys, os
T = sys.argv[1]
name = "demo_pkg-1.0-py3-none-any.whl"
files = {
  "demo_pkg/__init__.py": b"VALUE = 42\n",
  "demo_pkg-1.0.dist-info/METADATA": b"Metadata-Version: 2.1\nName: demo-pkg\nVersion: 1.0\n",
  "demo_pkg-1.0.dist-info/WHEEL": b"Wheel-Version: 1.0\nGenerator: test\nRoot-Is-Purelib: true\nTag: py3-none-any\n",
}
rec = []
for n, d in files.items():
    h = base64.urlsafe_b64encode(hashlib.sha256(d).digest()).rstrip(b"=").decode()
    rec.append("%s,sha256=%s,%d" % (n, h, len(d)))
rec.append("demo_pkg-1.0.dist-info/RECORD,,")
files["demo_pkg-1.0.dist-info/RECORD"] = ("\n".join(rec) + "\n").encode()
p = os.path.join(T, "wh", name)
with zipfile.ZipFile(p, "w") as z:
    for n, d in files.items():
        z.writestr(n, d)
digest = hashlib.sha256(open(p, "rb").read()).hexdigest()
open(os.path.join(T, "req", "requirements.txt"), "w").write(
    "demo-pkg==1.0 \\\n    --hash=sha256:%s\n" % digest)
PY
}

@test "python aralığı: sınırlar ve geçersiz girdiler" {
    comp_python_in_range 3.12.0 ">=3.12"
    comp_python_in_range 3.14.8 ">=3.12"
    run ! comp_python_in_range 3.11.9 ">=3.12"
    comp_python_in_range 3.12.5 ">=3.12,<3.13"
    run ! comp_python_in_range 3.13.0 ">=3.12,<3.13"
    run ! comp_python_in_range 3.9.6 ">=3.12"
    run ! comp_python_in_range 3.12 ">=3.12"  # tam sürüm değil
    run ! comp_python_in_range 3.12.0 ">3.12"  # desteklenmeyen biçim
    run ! comp_python_in_range 3.12.0 "*"
    comp_python_in_range 10.0.0 ">=9.9"           # sayısal karşılaştırma (dizge değil)
}

@test "python seçimi: aralığa uyan ilk aday seçilir, eski olan atlanır" {
    fake_python pyold 3.9.6
    fake_python pynew 3.13.1
    export PATH="$T/bin:$PATH" SSHPOCKET_PYTHON_CANDIDATES="pyold pynew"
    run comp_pick_python ">=3.12"
    [ "$status" -eq 0 ]
    [ "$output" = "$T/bin/pynew" ]
    run comp_pick_python ">=3.14"
    [ "$status" -ne 0 ]
}

@test "python seçimi: IFS değişmiş bağlamda da çalışır" {
    fake_python pynew 3.13.1
    export PATH="$T/bin:$PATH" SSHPOCKET_PYTHON_CANDIDATES="yok pynew"
    IFS=$'\n\t'
    run comp_pick_python ">=3.12"
    [ "$status" -eq 0 ]
}

@test "requirements: yalnızca hash'li 'paket==sürüm' kabul edilir" {
    local H64; H64=$(printf 'a%.0s' $(seq 1 64))
    printf 'foo==1.0 \\\n    --hash=sha256:%s \\\n    --hash=sha256:%s\n# via x\nbar==2.0 ; python_version >= "3.9" \\\n    --hash=sha256:%s\n' "$H64" "$H64" "$H64" > "$T/r1.txt"
    run comp_requirements_hashed "$T/r1.txt"; [ "$status" -eq 0 ]
    local bad
    for bad in 'foo==1.0' 'foo>=1.0 \\\n    --hash=sha256:'"$H64" '-r other.txt' '--index-url https://evil.example/simple\nfoo==1.0 \\\n    --hash=sha256:'"$H64" \
               'git+https://github.com/x/y.git' 'foo==1.0 \\\n    --hash=md5:abc' '-e .' '    --hash=sha256:'"$H64" ''; do
        printf -- "$bad\n" > "$T/r2.txt"
        run comp_requirements_hashed "$T/r2.txt"
        [ "$status" -ne 0 ] || { echo "KABUL: [$bad]"; return 1; }
    done
    printf 'foo==1.0 \\\n    --hash=sha256:%s\nbar==2.0\n' "$H64" > "$T/r3.txt"      # ikincisi hash'siz
    run comp_requirements_hashed "$T/r3.txt"; [ "$status" -ne 0 ]
}

@test "ortam kimliği: requirements, python sürümü değişince değişir; aynıysa sabittir" {
    printf 'a\n' > "$T/r.txt"; printf 'b\n' > "$T/r2.txt"
    a=$(comp_env_id "$T/r.txt" 3.14.8); b=$(comp_env_id "$T/r.txt" 3.14.8)
    c=$(comp_env_id "$T/r.txt" 3.13.1); d=$(comp_env_id "$T/r2.txt" 3.14.8)
    [ "$a" = "$b" ]
    [ "$a" != "$c" ]
    [ "$a" != "$d" ]
    [ "${#a}" -eq 16 ]
}

@test "venv: hash'li wheel çevrimdışı venv'e kurulur, ikinci çağrı HİÇBİR ŞEY kurmaz (önbellek)" {
    make_wheel
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -eq 0 ]
    dir="$output"
    [[ "$dir" == "$T/cache/venv/demo-"* ]]
    run "$dir/bin/python" -c 'import demo_pkg; print(demo_pkg.VALUE)'
    [ "$output" = "42" ]
    # ikinci çağrı yeniden KURMAMALI: venv içine bırakılan işaret dosyası korunur (yeniden kurulsaydı dizin silinirdi).
    # (Zaman damgası/stat kullanılmaz: macOS ve Linux'ta biçimi farklı, güvenilmez.)
    touch "$dir/.test-sentinel"
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -eq 0 ]
    [ "$output" = "$dir" ]
    [ -f "$dir/.test-sentinel" ]
}

@test "venv: sağlık işareti uyuşmuyorsa (python çalışsa bile) venv yeniden kurulur" {
    make_wheel
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -eq 0 ]
    dir="$output"
    printf 'baska-kimlik\n' > "$dir/.sshpocket-env"                        # işaret bozuldu
    rm -r "$dir"/lib/python*/site-packages/demo_pkg                          # içerik de eksik
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -eq 0 ]
    run "$dir/bin/python" -c 'import demo_pkg; print(demo_pkg.VALUE)'
    [ "$output" = "42" ]
}

@test "venv: pip --require-hashes ve --only-binary :all: ile, yalnızca venv'in python'uyla çağrılır" {
    make_wheel
    mkdir -p "$T/fakevenv"
    printf '#!/bin/sh\necho "$@" > "%s/pip.args"\nexit 0\n' "$T" > "$T/fakevenv/python"
    chmod +x "$T/fakevenv/python"
    SSHPOCKET_OFFLINE=0 run _comp_pip_install "$T/fakevenv/python" "$T/req/requirements.txt" ""
    [ "$status" -eq 0 ]
    grep -q -- '-m pip install' "$T/pip.args"
    grep -q -- '--require-hashes' "$T/pip.args"
    grep -q -- '--only-binary :all:' "$T/pip.args"
    grep -q -- '--no-input' "$T/pip.args"
}

@test "venv: yanlış hash'li gereksinim reddedilir, yarım venv bırakılmaz" {
    make_wheel
    sed -i.bak 's/--hash=sha256:[0-9a-f]\{64\}/--hash=sha256:0000000000000000000000000000000000000000000000000000000000000000/' "$T/req/requirements.txt"
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -ne 0 ]
    [ -z "$(ls -A "$T/cache/venv" 2>/dev/null)" ]
}

@test "venv: hash'siz requirements pip'e hiç verilmez" {
    mkdir -p "$T/req"; printf 'demo-pkg==1.0\n' > "$T/req/requirements.txt"
    run comp_venv_ensure demo "$T/req" ">=3.9" ""
    [ "$status" -ne 0 ]
    [ ! -d "$T/cache/venv" ]
}

@test "venv: yarım kalmış (işaretsiz) venv silinip yeniden kurulur" {
    make_wheel
    pyver=$(comp_python_version "$(command -v python3)")
    envid=$(comp_env_id "$T/req/requirements.txt" "$pyver")
    mkdir -p "$T/cache/venv/demo-$envid"; printf 'yarim' > "$T/cache/venv/demo-$envid/bozuk"
    export SSHPOCKET_PYTHON_CANDIDATES="python3"
    run comp_venv_ensure demo "$T/req" ">=3.9" "$T/wh"
    [ "$status" -eq 0 ]
    [ ! -e "$T/cache/venv/demo-$envid/bozuk" ]
    [ -x "$T/cache/venv/demo-$envid/bin/python" ]
}

@test "venv: uygun python yoksa anlaşılır hata, hiçbir şey oluşturulmaz" {
    make_wheel
    run comp_venv_ensure demo "$T/req" ">=99.0" "$T/wh"
    [ "$status" -ne 0 ]
    [[ "$output" == *"uygun Python"* ]]
    [ ! -d "$T/cache/venv" ]
}

@test "doctor: hazır ortamda çıkış 0 ve bölümler görünür" {
    SSHPOCKET_DOCTOR_OFFLINE=1 run doctor_run --python-range ">=3.9"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Temel araçlar"* ]]
    [[ "$output" == *"✅ python "* ]]
    [[ "$output" == *"Sonuç: hazır"* ]]
}

@test "doctor: uygun Python yoksa çıkış 1, çözüm önerisi gösterilir ama hiçbir şey kurulmaz" {
    fake_python pyold 3.9.6
    export PATH="$T/bin:$PATH" SSHPOCKET_PYTHON_CANDIDATES="pyold"
    SSHPOCKET_DOCTOR_OFFLINE=1 run doctor_run --python-range ">=3.12"
    [ "$status" -eq 1 ]
    [[ "$output" == *"uygun Python yok"* ]]
    [[ "$output" == *"3.9.6"* ]]
    [[ "$output" == *"brew install python@3.13"* ]]
    [[ "$output" == *"ÇALIŞTIRMAZ"* ]]
}

@test "doctor: IFS değişmiş bağlamda da çalışır (Main.sh ortamı)" {
    fake_python pyold 3.9.6
    export PATH="$T/bin:$PATH" SSHPOCKET_PYTHON_CANDIDATES="pyold"
    IFS=$'\n\t'
    SSHPOCKET_DOCTOR_OFFLINE=1 run doctor_run --python-range ">=3.12"
    [[ "$output" == *"3.9.6"* ]]
}

@test "CLI: ./start.sh --doctor çalışır (Main.sh)" {
    SSHPOCKET_DOCTOR_OFFLINE=1 run bash "$ROOT/sistem/Main.sh" --doctor --python-range ">=3.9"
    [ "$status" -eq 0 ]
    [[ "$output" == *"sshpocket --doctor"* ]]
}

# ---------- macOS /usr/bin/python3 saplaması (Command Line Tools yoksa pencere açar) ----------
make_stub_env() {   # $1: xcode-select çıkış kodu ("" = xcode-select yok)
    mkdir -p "$T/bin"
    printf '#!/bin/sh\necho CALISTI >> "%s/stub.ran"\ncase "$*" in *version_info*) echo 3.13.1 ;; esac\n' "$T" > "$T/bin/py3stub"
    chmod +x "$T/bin/py3stub"
    export SSHPOCKET_PY_STUB_PATH="$T/bin/py3stub" SSHPOCKET_PYTHON_CANDIDATES="py3stub"
    if [ -n "$1" ]; then printf '#!/bin/sh\nexit %s\n' "$1" > "$T/bin/xcode-select"; chmod +x "$T/bin/xcode-select"; fi
    export PATH="$T/bin:$PATH"
}

@test "python saplaması: Command Line Tools yoksa saplama ÇALIŞTIRILMAZ ve atlanır" {
    make_stub_env 2                              # xcode-select -p başarısız
    run comp_pick_python ">=3.12"
    [ "$status" -ne 0 ]
    [ ! -e "$T/stub.ran" ]
}

@test "python saplaması: Command Line Tools kuruluysa aynı yol normal aday sayılır" {
    make_stub_env 0
    run comp_pick_python ">=3.12"
    [ "$status" -eq 0 ]
    [ "$output" = "$T/bin/py3stub" ]
}

@test "python saplaması: xcode-select yoksa (Linux) saplama sayılmaz" {
    make_stub_env ""
    PATH="$T/bin:/usr/bin:/bin"; rm -f "$T/bin/xcode-select"
    command -v xcode-select >/dev/null 2>&1 && skip "bu makinede xcode-select /usr/bin altında"
    run comp_python_is_stub "$T/bin/py3stub"
    [ "$status" -ne 0 ]
}

@test "doctor: saplama atlanır, çalıştırılmaz ve nedeni açıklanır" {
    make_stub_env 2
    SSHPOCKET_DOCTOR_OFFLINE=1 run doctor_run --python-range ">=3.12"
    [ "$status" -eq 1 ]
    [[ "$output" == *"Command Line Tools kurulunca gerçek Python"* ]]
    [ ! -e "$T/stub.ran" ]
}

@test "python saplaması: yalnızca saplama yolu atlanır, başka yollar etkilenmez" {
    make_stub_env 2
    run comp_python_is_stub "/usr/local/bin/python3.14"
    [ "$status" -ne 0 ]
    run comp_python_is_stub "$T/bin/py3stub"
    [ "$status" -eq 0 ]
}
