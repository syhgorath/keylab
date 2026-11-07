#!/usr/bin/env bash
# ComponentsExec.sh - Paket eylemi yürütücüsü (Aşama 3, PR C): kapılar → değişkenler → onay → çalıştır → sonuç → kayıt
# TEK giriş noktası comp_exec_action: sonraki katmanlar (CLI/menü) hiçbir adımı atlayamaz.
# Çıkış kodları: 0 başarı · playbook'un kendi kodu · 190 çalıştırılmadan reddedildi (kapı/değişken/onay/hazırlık)
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsManifest.sh, ComponentsRun.sh, ComponentsGates.sh
#
# Bağlam (çağıran ayarlar; hepsi zorunlu, VM'e bağlanan ilk katman D'dir):
#   COMP_CTX_PKGDIR   paket kökü (component.json, ansible.cfg)      COMP_CTX_BINDIR   venv bin (ansible-playbook burada)
#   COMP_CTX_SERVER   sunucu adı   COMP_CTX_HOST   COMP_CTX_USER   COMP_CTX_PORT   COMP_CTX_KEY (mutlak yol)
#   COMP_CTX_PREFIX   Keychain modül öneki   COMP_CTX_PROFILE   algılanan OS profili   COMP_CTX_VALUES   değerler dosyası (yoksa boş)
#   COMP_CTX_BECOME_EXE   boş ya da /usr/bin/sudo.ws (Ubuntu'da sudo-rs + parolalı sudo için; envantere ansible_become_exe olarak yazılır)
# shellcheck disable=SC2034  # COMP_EXEC_RESULT çağıran tarafından okunur

COMP_EXEC_REFUSED=190
COMP_EXEC_RESULT=""

# comp_progress_parse <log-dosyası> → alan başına son durum, sekmeyle ayrılmış satırlar (alan, sıra, toplam, durum, degisen, atlanan, kesin, uyari, bekle); yoksa "-"
# Paketin `SSHPOCKET_PROGRESS alan=<ad> sira=<n>/<toplam> durum=basladi|bitti [degisen= atlanan= kesin= uyari= bekle_tahmin=]` satırları (docs/package-standard.md).
# Satır başı ve tırnak varsayılmaz (Ansible `msg:` çıktısında girintili/tırnaklı gelir); bilinmeyen anahtarlar yok sayılır, bozuk satır atlanır.
# `bitti` olmadan kalan alan "basladi" görünür (hata ya da hâlâ çalışıyor). Satır yoksa çıktı boş (ilerleme bilgisi yok) ve dönüş 1.
comp_progress_parse() {
    [ -f "$1" ] && grep -q 'SSHPOCKET_PROGRESS' "$1" 2>/dev/null || return 1
    grep 'SSHPOCKET_PROGRESS' "$1" | tr -d '\r' | awk '
        {
            i = index($0, "SSHPOCKET_PROGRESS"); line = substr($0, i + 18)
            n = split(line, tok, /[ \t]+/); delete kv
            for (j = 1; j <= n; j++) {
                t = tok[j]; gsub(/["\\,}]+$/, "", t)
                e = index(t, "="); if (e > 1) kv[substr(t, 1, e - 1)] = substr(t, e + 1)
            }
            a = kv["alan"]; d = kv["durum"]
            if (a !~ /^[a-z][a-z0-9_]*$/ || (d != "basladi" && d != "bitti")) next
            if (kv["sira"] !~ /^[0-9]+\/[0-9]+$/) next
            if (!(a in seen)) { seen[a] = 1; order[++cnt] = a }
            split(kv["sira"], sp, "/"); sira[a] = sp[1]; top[a] = sp[2]
            if (d == "bitti" || !(a in dur)) {
                dur[a] = d
                if (d == "bitti") { dg[a] = kv["degisen"]; at[a] = kv["atlanan"]; ks[a] = kv["kesin"]; uy[a] = kv["uyari"] }
            }
            if (kv["bekle_tahmin"] ~ /^[0-9]+$/) bk[a] = kv["bekle_tahmin"]
        }
        function v(x) { return (x == "" ? "-" : x) }
        END { for (j = 1; j <= cnt; j++) { a = order[j]; printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", a, sira[a], top[a], dur[a], v(dg[a]), v(at[a]), v(ks[a]), v(uy[a]), v(bk[a]) } }'
}

# comp_exec_parse_result <bilgi> <eylem> <log-dosyası> → COMP_EXEC_RESULT ("k=v" satırları); geçersizse boş ve 1
# Satır başı varsayılmaz (Ansible yaml/json çıktısında girintili/tırnaklı gelir): son 'SSHPOCKET_RESULT' işaretinden itibaren okunur.
comp_exec_parse_result() {
    local info="$1" action="$2" log="$3" id line rest tok keys k v seen=""
    local -a toks
    COMP_EXEC_RESULT=""
    [ "$(comp_action_get "$info" "$action" outputs.format)" = "keyvalue-line" ] || return 0
    id=$(comp_info_get "$info" id)
    keys=",$(comp_action_get "$info" "$action" outputs.keys),"
    line=$(grep 'SSHPOCKET_RESULT' "$log" 2>/dev/null | tail -n 1 | tr -d '\r')   # ssh -t (script runner, sudo'lu yol) satır sonlarını CRLF yapar
    [ -n "$line" ] || { echo "Sonuç satırı bulunamadı (SSHPOCKET_RESULT)." >&2; return 1; }
    rest="SSHPOCKET_RESULT${line#*SSHPOCKET_RESULT}"
    rest="${rest%%\\n*}"; rest="${rest%%\"*}"; rest="${rest%%\'*}"
    IFS=' ' read -r -a toks <<< "$rest"
    [ "${toks[0]:-}" = "SSHPOCKET_RESULT" ] && [ "${toks[1]:-}" = "$id.$action" ] \
        || { echo "Sonuç satırı beklenen önekle ('SSHPOCKET_RESULT $id.$action') başlamıyor." >&2; return 1; }
    local i out=""
    for ((i = 2; i < ${#toks[@]}; i++)); do
        tok="${toks[$i]}"
        k="${tok%%=*}"; v="${tok#*=}"
        if [[ "$tok" != *=* ]] || [[ "$keys" != *",$k,"* ]] || ! [[ "$v" =~ ^[A-Za-z0-9_.:+-]{0,64}$ ]] || [[ "$seen" == *",$k,"* ]]; then
            echo "Sonuç satırı geçersiz ('$k' bilinmiyor, yinelenmiş ya da değeri uygunsuz)." >&2
            return 1
        fi
        seen="$seen,$k,"
        out="$out$k=$v"$'\n'
    done
    COMP_EXEC_RESULT="${out%$'\n'}"
    return 0
}

# --- script runner (docs/package-standard.md §7) ---
COMP_EXEC_REMOTE_TMP=""

# _comp_exec_remote_rm → sudo'lu yolda sunucuya yazılan geçici betiği (içinde değişken değerleri olabilir) siler; her çıkışta çağrılır, hata verirse sessiz geçer
_comp_exec_remote_rm() {
    [ -n "$COMP_EXEC_REMOTE_TMP" ] || return 0
    [[ "$COMP_EXEC_REMOTE_TMP" =~ ^/tmp/sshpocket-[A-Za-z0-9]{8}$ ]] || return 0
    ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 \
        -o PreferredAuthentications=publickey -o PasswordAuthentication=no \
        -p "$COMP_CTX_PORT" -i "$COMP_CTX_KEY" "$COMP_CTX_USER@$COMP_CTX_HOST" "rm -f $COMP_EXEC_REMOTE_TMP" >/dev/null 2>&1 || true
    COMP_EXEC_REMOTE_TMP=""
}

# _comp_exec_sha256 <dosya> → dosyanın SHA-256 özeti (hex); sha256sum ya da shasum yoksa 1
_comp_exec_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
    else return 1; fi
}

# _comp_exec_script <bilgi> <eylem> <vars.json> <yerel-iş-klasörü> <durum-klasörü>
# Betik paketin 'scripts/' klasöründen okunur; başına SSHPOCKET_* export satırları eklenir ve SUNUCUDA 'bash -s -- <argümanlar>' ile çalışır.
#  · sudo gerekmiyorsa: ssh stdin'inden akıtılır (betik stdin okuyamaz).
#  · requires_sudo ise: sunucuda 0600 geçici dosyaya yazılır; ssh -t ile (sudo istemi AYNI terminalde) root'a ait klasöre kopyalanıp SHA-256 doğrulanır ve çalıştırılır; her çıkışta silinir.
_comp_exec_script() {
    local info="$1" action="$2" vf="$3" work="$4" sdir="$5" refused="$COMP_EXEC_REFUSED"
    local cmd script payload log rc rpath sudo_prog rargs w hash rscript
    local -a words ssh_base
    cmd=$(comp_action_get "$info" "$action" cmd)
    IFS=' ' read -r -a words <<< "$cmd"
    script="$COMP_CTX_PKGDIR/${words[0]}"
    if [ ! -f "$script" ] || [ -L "$script" ] || [ "$(wc -c < "$script" | tr -d ' ')" -gt 262144 ]; then
        echo "Betik bulunamadı, sembolik bağ ya da çok büyük (en çok 256 KB): ${words[0]}" >&2; return "$refused"
    fi
    payload="$work/payload.sh"
    { python3 -I "$COMP_RUN_PY" env "$vf" "$COMP_CTX_SERVER" "$action" "$COMP_CTX_PROFILE" && cat "$script"; } > "$payload" \
        || { echo "Betik hazırlanamadı (değişkenler)." >&2; return "$refused"; }
    rargs=""
    for w in "${words[@]:1}"; do rargs="$rargs $(printf '%q' "$w")"; done
    ssh_base=(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10
              -o PreferredAuthentications=publickey -o PasswordAuthentication=no -p "$COMP_CTX_PORT" -i "$COMP_CTX_KEY")
    log="$sdir/last-run.log"
    ( umask 077; : > "$log" ) || return "$refused"
    printf 'komut: ssh -p %s %s@%s bash -s --%s  (betik: %s)\n' "$COMP_CTX_PORT" "$COMP_CTX_USER" "$COMP_CTX_HOST" "$rargs" "${words[0]}" >> "$log"
    # shellcheck disable=SC2064
    trap "rm -r -- $(printf '%q' "$work") 2>/dev/null; _comp_exec_remote_rm" EXIT

    if [ "$(comp_action_get "$info" "$action" sudo)" = "yes" ]; then
        # shellcheck disable=SC2016  # $f sunucuda genişler (yerelde değil)
        rpath=$("${ssh_base[@]}" "$COMP_CTX_USER@$COMP_CTX_HOST" 'umask 077; f=$(mktemp /tmp/sshpocket-XXXXXXXX) && cat > "$f" && printf %s "$f"' < "$payload" 2>/dev/null) \
            || { echo "Betik sunucuya yazılamadı." >&2; return "$refused"; }
        [[ "$rpath" =~ ^/tmp/sshpocket-[A-Za-z0-9]{8}$ ]] || { echo "Sunucudaki geçici betik yolu beklenmedik: çalıştırılmadı." >&2; return "$refused"; }
        COMP_EXEC_REMOTE_TMP="$rpath"
        sudo_prog="sudo"; [ -n "${COMP_CTX_BECOME_EXE:-}" ] && sudo_prog="$COMP_CTX_BECOME_EXE"
        hash=$(_comp_exec_sha256 "$payload") && [[ "$hash" =~ ^[0-9a-f]{64}$ ]] || { echo "Betiğin özeti hesaplanamadı (sha256sum/shasum yok)." >&2; return "$refused"; }
        {
            echo "🔑 Sudo parolası istenecek: aşağıdaki istem sunucudaki sudo parolasıdır (sudo parola istemiyorsa yalnızca Enter)."
            echo "   Parola yalnızca sunucunun sudo isteminde girilir; sshpocket'e, günlüğe ya da komut satırına yazılmaz."
        } >&2
        # Kullanıcı sahipli geçici dosya root olarak ÇALIŞTIRILMAZ: önce root'a ait 0700 klasöre kopyalanır, kopyanın SHA-256'sı yerelde hesaplanan
        # özetle karşılaştırılır (uyuşmazsa 97 ile durur), ancak sonra çalışır. Böylece yükleme ile sudo arasında dosya değiştirilse bile root yalnızca
        # imzalı paketteki betiği çalıştırır. Klasör her çıkışta silinir.
        # shellcheck disable=SC2016  # değişkenler SUNUCUDA genişler (yerelde değil)
        rscript='d=$(mktemp -d) || exit 96; trap "rm -rf \"$d\"" EXIT; install -o root -g root -m 0700 -- "$1" "$d/s.sh" || exit 96; h=$(sha256sum "$d/s.sh" | cut -d" " -f1); [ "$h" = "$2" ] || { echo "ÖZET UYUŞMUYOR: betik çalıştırılmadı (sunucudaki kopya imzalı paketteki betikle aynı değil)." >&2; exit 97; }; shift 2; bash "$d/s.sh" "$@"'
        "${ssh_base[@]}" -t "$COMP_CTX_USER@$COMP_CTX_HOST" "$sudo_prog bash -c '$rscript' bash $rpath $hash$rargs" 2>&1 | tee -a "$log"
        rc="${PIPESTATUS[0]}"
        if [ "$rc" -eq 97 ]; then echo "⛔ Sunucudaki betik özeti yerelle uyuşmadı: betik ÇALIŞTIRILMADI. Sunucuda /tmp'ye başka bir süreç yazmış olabilir; sunucuyu inceleyin." >&2; fi
        _comp_exec_remote_rm
    else
        "${ssh_base[@]}" "$COMP_CTX_USER@$COMP_CTX_HOST" "bash -s --$rargs" < "$payload" 2>&1 | tee -a "$log"
        rc="${PIPESTATUS[0]}"
    fi
    return "$rc"
}

# _comp_exec_body <bilgi> <paket> <sürüm> <eylem> → alt kabukta çalışır; geçici dosyalar HER çıkışta (Ctrl-C/TERM dahil) silinir
_comp_exec_body() {
    local info="$1" id="$2" ver="$3" action="$4" kv
    local server="$COMP_CTX_SERVER" pkg="$COMP_CTX_PKGDIR" bindir="${COMP_CTX_BINDIR:-}" runner work sdir vf inv group dir_var outdir log w prog rc line
    local -a sets warn_names applied runsets
    local rs
    local refused="$COMP_EXEC_REFUSED"

    runner=$(comp_info_get "$info" runner)
    [ -f "$pkg/component.json" ] || { echo "Paket dizini bulunamadı." >&2; return "$refused"; }
    case "$runner" in
        ansible) [ -d "$bindir" ] || { echo "Python ortamı (venv) bulunamadı." >&2; return "$refused"; } ;;
        script) ;;
        *) echo "Desteklenmeyen runner: $runner" >&2; return "$refused" ;;
    esac

    # ---- kapılar (hepsi yerel; biri durdurursa HİÇBİR ŞEY çalışmaz) ----
    comp_gate_not_root "$info" "$action" "$COMP_CTX_USER" || return "$refused"
    comp_gate_known_host "$COMP_CTX_HOST" "$COMP_CTX_PORT" || return "$refused"
    comp_gate_agent "$COMP_CTX_PREFIX" "$COMP_CTX_KEY" || return "$refused"
    comp_gate_profile "$info" "$COMP_CTX_PROFILE" || return "$refused"
    comp_gate_requires "$info" "$id" "$server" "$ver" "$action" || return "$refused"
    comp_gate_requires_vars "$info" "$action" "${COMP_CTX_VALUES:-/dev/null}" || return "$refused"

    sdir=$(comp_gate_state_ensure "$id" "$server") || return "$refused"
    work=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-run.XXXXXX") || { echo "Geçici klasör oluşturulamadı." >&2; return "$refused"; }
    chmod 700 "$work"
    # yol trap KURULURKEN genişletilir: 'work' yereldir, EXIT anında artık yoktur
    # shellcheck disable=SC2064
    trap "rm -r -- $(printf '%q' "$work") 2>/dev/null" EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    trap 'exit 129' HUP      # terminal penceresi kapanırsa da geçici dosyalar silinsin
    vf="$work/vars.json"; inv="$work/inventory"

    # ---- değişkenler: dir_var HER ZAMAN kullanıcı klasörüne yönlendirilir (hassas çıktı USB'ye yazılmaz) ----
    sets=()
    dir_var=$(comp_action_get "$info" "$action" outputs.dir_var)
    if [ -n "$dir_var" ]; then
        outdir="$sdir/out"
        if ! ( umask 077; mkdir -p "$outdir" ) || ! chmod 700 "$outdir"; then echo "Çıktı klasörü oluşturulamadı." >&2; return "$refused"; fi
        sets=(--set "$dir_var=$outdir")
    fi
    # eylem düzeyi sabit değişkenler (manifest action.set): kullanıcının .vars değerinin ÜSTÜNE yazılır (örn. *_preview eylemi pve_apply=false'u zorlar)
    while IFS= read -r kv; do
        [ -n "$kv" ] && sets+=(--set "$kv")
    done < <(comp_info_all "$info" "action.$action.set")
    # platform apply_vars: kurallar comp_gate_apply_vars'ta; metin onay sorusundan ÖNCE gösterilir, onay yoksa hiçbir şey çalışmaz
    applied=()
    while IFS= read -r kv; do [ -n "$kv" ] && applied+=("$kv"); done < <(comp_gate_apply_vars "$info" "$action" "${COMP_CTX_VALUES:-/dev/null}")
    for kv in ${applied[@]+"${applied[@]}"}; do sets+=(--set "$kv"); done
    # katalog (S2): seçilen yazılım/mod bu koşuya özel verilir (.vars'a yazılmaz); kapı: comp_gate_run_sets
    runsets=()
    if [ -n "${COMP_CTX_RUNSET:-}" ]; then
        if ! rs=$(comp_gate_run_sets "$info" "$action"); then return "$refused"; fi
        while IFS= read -r kv; do [ -n "$kv" ] && runsets+=("$kv"); done <<< "$rs"
        for kv in ${runsets[@]+"${runsets[@]}"}; do sets+=(--set "$kv"); done
        if [ "${#runsets[@]}" -gt 0 ]; then { echo "ℹ️  Bu koşuya özel (değişken dosyanıza YAZILMAZ):"; printf '    %s\n' "${runsets[@]}"; } >&2; fi
    fi
    warn_names=()
    # Salt okunur, başka eylemin ön koşulu olmayan eylemler (rollback_status vb.) zorunlu değişken istemez; değer verilmişse yine doğrulanır
    if ! comp_gate_required_applies "$info" "$action"; then sets+=(--no-required); fi
    if ! w=$(comp_run_vars "$pkg/component.json" "${COMP_CTX_VALUES:-/dev/null}" "$vf" ${sets[@]+"${sets[@]}"}); then
        echo "Değişkenler geçersiz; çalıştırılmadı." >&2; return "$refused"
    fi
    while IFS= read -r line; do
        case "$line" in warn=*) warn_names+=("${line#warn=}") ;; esac
    done <<< "$w"
    if comp_gate_warn_applies "$info" "$action"; then
        comp_gate_warn_vars ${warn_names[@]+"${warn_names[@]}"} || return "$refused"
    fi

    # ---- onay (effects + uyarılar gösterilir) ----
    comp_gate_apply_vars_notice "$info" "${COMP_CTX_VALUES:-/dev/null}" ${applied[@]+"${applied[@]}"}
    comp_gate_confirm "$info" "$action" "$server" || return "$refused"

    # ---- script runner: betik SUNUCUDA çalışır (Mac'te yabancı kod çalışmaz); kapılar/değişkenler/onay yukarıda ortak ----
    if [ "$runner" = "script" ]; then
        _comp_exec_script "$info" "$action" "$vf" "$work" "$sdir"
        return $?
    fi

    # ---- girdiler ----
    group=$(comp_info_get "$info" inventory_group); group="${group:-sshpocket}"
    comp_run_write_inventory "$inv" "$group" "$server" "$COMP_CTX_HOST" "$COMP_CTX_USER" "$COMP_CTX_PORT" "$COMP_CTX_KEY" "${COMP_CTX_BECOME_EXE:-}" || return "$refused"
    comp_run_build_argv "$info" "$action" "$inv" "$server" "$vf" || return "$refused"
    prog="${COMP_RUN_ARGV[0]}"
    if [[ "$prog" == */* ]] || [ ! -x "$bindir/$prog" ]; then echo "Çalıştırılacak program bulunamadı: $prog" >&2; return "$refused"; fi
    COMP_RUN_ARGV[0]="$bindir/$prog"

    # sudo parolası istemi: Ansible yalnızca "BECOME password:" yazar; ne olduğunu önceden açıkla
    for w in "${COMP_RUN_ARGV[@]}"; do
        if [ "$w" = "--ask-become-pass" ] || [ "$w" = "-K" ]; then
            {
                echo "🔑 Sudo parolası istenecek: aşağıdaki 'BECOME password:' sunucudaki sudo parolasıdır."
                echo "   Sudo parola istemiyorsa (NOPASSWD) yalnızca Enter'a basın."
                echo "   Parola yalnızca Ansible'ın kendi istemine girilir; sshpocket'e, günlüğe ya da komut satırına yazılmaz."
            } >&2
            break
        fi
    done

    # PYTHONDONTWRITEBYTECODE: paketin Python eklentileri (callback/filter) kurulu klasöre __pycache__ yazarsa
    # bir sonraki koşuda "kurulu paketin dosyaları değişmiş" bütünlük denetimi paketi reddederdi.
    # ---- çalıştır (günlük kullanıcı klasöründe, izin 600; kullanıcı terminali çıktıyı olduğu gibi görür) ----
    log="$sdir/last-run.log"
    ( umask 077; : > "$log" ) || return "$refused"
    printf 'komut: %s\n' "$(printf '%q ' "${COMP_RUN_ARGV[@]}")" >> "$log"
    for kv in ${applied[@]+"${applied[@]}"} ${runsets[@]+"${runsets[@]}"}; do printf 'bu koşuya özel: %s\n' "$kv" >> "$log"; done
    {
        cd "$pkg" && env PYTHONDONTWRITEBYTECODE=1 ANSIBLE_CONFIG="$pkg/ansible.cfg" ANSIBLE_HOST_KEY_CHECKING=True ANSIBLE_TIMEOUT="${ANSIBLE_TIMEOUT:-30}" PATH="$bindir:$PATH" "${COMP_RUN_ARGV[@]}"
    } 2>&1 | tee -a "$log"
    rc="${PIPESTATUS[0]}"
    if [ "$rc" -ne 0 ] && grep -q "waiting for privilege escalation prompt" "$log" 2>/dev/null; then
        {
            echo "  ↳ Sudo istemi beklenirken zaman aşımı: sudo parolası yanlış olabilir ya da sunucudaki sudo Ansible ile uyumsuz olabilir"
            echo "    (Ubuntu 26.04'te varsayılan sudo-rs parolalı kullanımda takılır). Parolayı kontrol edin; sudo'yu elle deneyin (ssh ile: sudo -v)."
        } >&2
    fi
    if [ "$rc" -ne 0 ] && grep -q "Incorrect sudo password\|Missing sudo password" "$log" 2>/dev/null; then
        echo "  ↳ Sudo parolası reddedildi. Aynı komutu yeniden çalıştırıp 'BECOME password:' istemine doğru parolayı yazın." >&2
    fi
    return "$rc"
}

# comp_exec_save_result <durum-klasörü> <eylem> → COMP_EXEC_RESULT'ı "result.<eylem>" dosyasına yazar (ilk satır: epoch; izin 600, atomik)
comp_exec_save_result() {
    local sdir="$1" action="$2" tmp
    [[ "$action" =~ ^[a-z][a-z0-9_]{0,63}$ ]] || return 1
    [ -n "$COMP_EXEC_RESULT" ] || return 0
    tmp=$(mktemp "$sdir/result.XXXXXX") || return 1
    chmod 600 "$tmp"
    if ! { date +%s; printf '%s\n' "$COMP_EXEC_RESULT"; } > "$tmp" || ! mv -f -- "$tmp" "$sdir/result.$action"; then
        rm -f -- "$tmp"; return 1
    fi
}

# comp_exec_last_result <paket> <sunucu> <eylem> → ilk satır epoch, sonra "anahtar=değer" satırları; kayıt yoksa 1
comp_exec_last_result() {
    local f
    f="$(_comp_gate_state_dir)/$1/$2/result.$3"
    [[ "$3" =~ ^[a-z][a-z0-9_]{0,63}$ ]] && _comp_gate_name_ok "$2" && comp_id_valid "$1" && [ -f "$f" ] || return 1
    head -n 40 "$f"
}

# _comp_exec_mask (BSD sed'de s///I yok: büyük/küçük harf köşeli ayraçla) → stdin'deki günlükten parola/anahtar/token izlerini siler (ansible/ssh çıktısına sızarsa kalıcı günlüğe geçmesin)
_comp_exec_mask() {
    LC_ALL=C tr -d '\000' | LC_ALL=C sed -E \
        -e '/-----BEGIN [A-Z ]*PRIVATE KEY-----/,/-----END [A-Z ]*PRIVATE KEY-----/c\
[özel anahtar bloğu gizlendi]' \
        -e 's/(([Bb][Ee][Cc][Oo][Mm][Ee]|[Ss][Uu][Dd][Oo]|[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]|[Pp][Aa][Ss][Ss][Ww][Dd]|[Pp][Aa][Rr][Oo][Ll][Aa]|[Tt][Oo][Kk][Ee][Nn]|[Ss][Ee][Cc][Rr][Ee][Tt]|[Aa][Pp][Ii][_-]?[Kk][Ee][Yy])[A-Za-z_ ]{0,20}[:=][ ]*)[^ ].*/\1****/' \
        -e 's/(ghp_|github_pat_|glpat-|xox[bp]-)[A-Za-z0-9_-]{8,}/\1****/g'
}

# _comp_exec_fmt <epoch> <biçim> [-u] → BSD (macOS) ve GNU date ile aynı çıktı
_comp_exec_fmt() {
    local u=""; [ "${3:-}" = "-u" ] && u="-u"
    # shellcheck disable=SC2086
    date $u -r "$1" "$2" 2>/dev/null || date $u -d "@$1" "$2"
}

# comp_exec_archive_log <durum-klasörü> <paket> <sürüm> <eylem> <sunucu> <çıkış-kodu> <başlangıç-epoch>
# last-run.log'u kalıcı arşive kopyalar: <durum>/logs/<YYYYmmdd-HHMMSS>-<eylem>.log (dizin 0700, dosya 0600); başlık + maskelenmiş gövde + sonuç.
# Yolu COMP_EXEC_LOGFILE'a koyar; başarısızlıkta <durum>/last-fail-log'a yazar (başarıda siler). results biçimi DEĞİŞMEZ.
# Eski günlükler: sunucu/paket başına son COMP_LOG_KEEP (varsayılan 50) kalır.
comp_exec_archive_log() {
    local sdir="$1" id="$2" ver="$3" action="$4" server="$5" rc="$6" t0="$7" ldir name f n t1 keep="${COMP_LOG_KEEP:-50}" res cmdline
    COMP_EXEC_LOGFILE=""
    [ -f "$sdir/last-run.log" ] || return 0
    [[ "$action" =~ ^[a-z][a-z0-9_]{0,63}$ ]] || return 1
    [[ "$keep" =~ ^[0-9]+$ ]] && [ "$keep" -ge 1 ] || keep=50
    ldir="$sdir/logs"
    ( umask 077; mkdir -p "$ldir" ) && chmod 700 "$ldir" || return 1
    t1=$(date +%s)
    name="$(_comp_exec_fmt "$t0" +%Y%m%d-%H%M%S)-$action"
    f="$ldir/$name.log"; n=1
    while [ -e "$f" ]; do n=$((n + 1)); f="$ldir/$name-$n.log"; done
    res=$([ "$rc" -eq 0 ] && echo ok || echo fail)
    # 129/130/143 = HUP/INT/TERM ile kesildi (terminal kapandı, Ctrl-C, kill)
    case "$rc" in 129|130|143) res="fail — kesildi (sinyal $((rc - 128)))" ;; esac
    cmdline=$(sed -n 's/^komut: //p' "$sdir/last-run.log" | head -n 1 | _comp_exec_mask)
    {
        echo "# sshpocket eylem günlüğü"
        printf '# zaman: %s (yerel) / %s (UTC)\n' "$(_comp_exec_fmt "$t0" '+%Y-%m-%d %H:%M:%S %z')" "$(_comp_exec_fmt "$t0" '+%Y-%m-%dT%H:%M:%SZ' -u)"
        printf '# paket: %s %s\n# eylem: %s\n# sunucu: %s\n' "$id" "$ver" "$action" "$server"
        printf '# komut: %s\n# süre: %ss\n' "${cmdline:-?}" "$((t1 - t0))"
        echo "# ----"
        grep -v '^komut: ' "$sdir/last-run.log" | _comp_exec_mask
        echo "# ----"
        printf '# çıkış kodu: %s\n# sonuç: %s\n' "$rc" "$res"
    } > "$f" 2>/dev/null || { rm -f -- "$f"; return 1; }
    chmod 600 "$f"
    COMP_EXEC_LOGFILE="$f"
    if [ "$rc" -ne 0 ]; then ( umask 077; printf '%s\n' "$f" > "$sdir/last-fail-log" ) || true
    else rm -f -- "$sdir/last-fail-log"; fi
    # temizlik: ad zaman damgasıyla başladığı için sözlük sırası = zaman sırası
    n=$(find "$ldir" -maxdepth 1 -type f -name '*.log' | wc -l | tr -d ' ')
    if [ "$n" -gt "$keep" ]; then
        find "$ldir" -maxdepth 1 -type f -name '*.log' | sort | head -n "$((n - keep))" | while IFS= read -r f; do rm -f -- "$f"; done
    fi
    return 0
}

# _comp_exec_on_signal <çıkış-kodu> → HUP/TERM'de (alt kabuk zaten sonlandırılmış olabilir) günlüğü arşivleyip çıkar
_comp_exec_on_signal() {
    local rc="$1" id ver action server t0 sdir
    IFS='|' read -r id ver action server t0 <<< "${COMP_EXEC_SIG_CTX:-}"
    if [ -n "$id" ] && sdir=$(comp_gate_state_ensure "$id" "$server" 2>/dev/null); then
        comp_exec_archive_log "$sdir" "$id" "$ver" "$action" "$server" "$rc" "$t0" || true
        comp_gate_record "$id" "$server" "$ver" "$action" fail || true
    fi
    exit "$rc"
}

# comp_exec_action <bilgi> <paket> <sürüm> <eylem> → kapılar, çalıştırma, sonuç ayrıştırma ve kayıt
comp_exec_action() {
    local info="$1" id="$2" ver="$3" action="$4" rc sdir t0
    COMP_EXEC_RESULT=""; COMP_EXEC_LOGFILE=""
    t0=$(date +%s)
    # terminal paneli kapanırsa (HUP) ya da kill (TERM) gelirse üst kabuk da düşer: günlük yine arşivlensin
    local old_hup old_term
    old_hup=$(trap -p HUP); old_term=$(trap -p TERM)
    COMP_EXEC_SIG_CTX="$id|$ver|$action|$COMP_CTX_SERVER|$t0"
    trap '_comp_exec_on_signal 129' HUP
    trap '_comp_exec_on_signal 143' TERM
    ( _comp_exec_body "$info" "$id" "$ver" "$action" )
    rc=$?
    trap - HUP TERM
    [ -z "$old_hup" ] || eval "$old_hup"
    [ -z "$old_term" ] || eval "$old_term"
    COMP_EXEC_SIG_CTX=""
    [ "$rc" -eq "$COMP_EXEC_REFUSED" ] && return "$rc"            # hiçbir şey çalışmadı: kayıt yok
    sdir=$(comp_gate_state_ensure "$id" "$COMP_CTX_SERVER") || return "$rc"
    comp_exec_parse_result "$info" "$action" "$sdir/last-run.log" || true
    comp_exec_archive_log "$sdir" "$id" "$ver" "$action" "$COMP_CTX_SERVER" "$rc" "$t0" || true
    comp_exec_save_result "$sdir" "$action" || true
    if [ "$rc" -eq 0 ]; then comp_gate_record "$id" "$COMP_CTX_SERVER" "$ver" "$action" ok || true
    else comp_gate_record "$id" "$COMP_CTX_SERVER" "$ver" "$action" fail || true; fi
    return "$rc"
}
