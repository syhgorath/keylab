#!/usr/bin/env bash
# Components.sh - Paket (bileşen) kilit dosyası ve imza doğrulama (docs/package-standard.md, Aşama 2.0)
#
# components.lock biçimi (bash 3.2 ile güvenle okunur; YAML değil):
#   [linux-hardening]
#   repo    = syhgorath/linux-hardening
#   version = 0.1.0
#   sha256  = <arşivin 64 haneli özeti>
#   signers = sshpocket-component namespaces="sshpocket-component" ssh-ed25519 AAAA...
#
# Kilit dosyasında olmayan paket çalıştırılmaz. Dosya ASLA source edilmez; her değer doğrulanır.
# Gerektirir: Update.sh'den bağımsızdır (yalnızca ssh-keygen ve shasum/sha256sum).

COMP_NAMESPACE="sshpocket-component"

comp_lock_file() {
    if [ -n "${SSHPOCKET_COMPONENTS_LOCK:-}" ]; then
        printf '%s' "$SSHPOCKET_COMPONENTS_LOCK"
    else
        printf '%s' "${SISTEM_PATH:-.}/../components.lock"
    fi
}

comp_id_valid() { [[ "$1" =~ ^[a-z][a-z0-9-]{0,63}$ ]]; }
comp_version_valid() { [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; }
comp_repo_valid() {
    [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] && [[ "$1" != *..* ]]
}
comp_sha256_valid() { [[ "$1" =~ ^[0-9a-f]{64}$ ]]; }
# signers: "<principal> namespaces=\"sshpocket-component\" ssh-ed25519 <base64>"; principal = ad alanı
comp_signers_valid() {
    local re='^sshpocket-component namespaces="sshpocket-component" ssh-ed25519 [A-Za-z0-9+/]+=*( [^[:cntrl:]]*)?$'
    [[ "$1" =~ $re ]]
}

# Dosyayı ayrıştırır; "id|anahtar|değer" satırları yazar. Hata varsa mesaj stderr, dönüş 1.
# (Tek geçiş, katı: bilinmeyen anahtar, yinelenen id/anahtar, bozuk değer = hata.)
comp_lock_parse() {
    local f line lineno=0 id="" key val seen_ids="" seen_keys="" rc=0
    local re_header='^\[([^]]*)\]$' re_kv='^([a-z0-9_]+)[[:space:]]*=[[:space:]]*(.*)$'
    f=$(comp_lock_file)
    [ -f "$f" ] || return 0
    while IFS= read -r line || [ -n "$line" ]; do
        lineno=$((lineno + 1))
        line="${line%$'\r'}"
        case "$line" in ''|'#'*) continue ;; esac
        if [[ "$line" =~ $re_header ]]; then
            id="${BASH_REMATCH[1]}"
            if ! comp_id_valid "$id"; then echo "components.lock:$lineno: geçersiz paket kimliği" >&2; return 1; fi
            case " $seen_ids " in *" $id "*) echo "components.lock:$lineno: yinelenen paket: $id" >&2; return 1 ;; esac
            seen_ids="$seen_ids $id"; seen_keys=""
            continue
        fi
        if [ -z "$id" ]; then echo "components.lock:$lineno: paket başlığı yok" >&2; return 1; fi
        if [[ "$line" =~ $re_kv ]]; then
            key="${BASH_REMATCH[1]}"; val="${BASH_REMATCH[2]}"
            val="${val%"${val##*[![:space:]]}"}"
        else
            echo "components.lock:$lineno: anlaşılmayan satır" >&2; return 1
        fi
        case " $seen_keys " in *" $key "*) echo "components.lock:$lineno: yinelenen anahtar: $key" >&2; return 1 ;; esac
        seen_keys="$seen_keys $key"
        case "$key" in
            repo)    comp_repo_valid "$val" || rc=1 ;;
            version) comp_version_valid "$val" || rc=1 ;;
            sha256)  comp_sha256_valid "$val" || rc=1 ;;
            signers) comp_signers_valid "$val" || rc=1 ;;
            *) echo "components.lock:$lineno: bilinmeyen anahtar: $key" >&2; return 1 ;;
        esac
        if [ "$rc" -ne 0 ]; then echo "components.lock:$lineno: geçersiz değer ($key)" >&2; return 1; fi
        printf '%s|%s|%s\n' "$id" "$key" "$val"
    done < "$f"
}

# Tam doğrulama: ayrıştırılır ve her paketin 4 anahtarı da bulunur
comp_lock_validate() {
    local parsed id key
    parsed=$(comp_lock_parse) || return 1
    for id in $(printf '%s\n' "$parsed" | cut -d'|' -f1 | sort -u); do
        [ -n "$id" ] || continue
        for key in repo version sha256 signers; do
            if ! printf '%s\n' "$parsed" | grep -q "^$id|$key|"; then
                echo "components.lock: $id için '$key' eksik" >&2; return 1
            fi
        done
    done
}

# Paket kimlikleri (kilitte olanlar)
comp_ids() { comp_lock_parse 2>/dev/null | cut -d'|' -f1 | sort -u; }
comp_known() { comp_ids | grep -qx "$1"; }

# comp_get <id> <anahtar>
comp_get() {
    # Girdinin tamamını okur (erken exit yok): pipefail altında üretici SIGPIPE ile ölüp 141 döndürmesin
    comp_lock_parse 2>/dev/null | awk -F'|' -v id="$1" -v k="$2" '
        $1==id && $2==k && !done { sub(/^[^|]*\|[^|]*\|/, ""); val=$0; done=1 }
        END { if (done) print val }'
}

comp_sha256_of() {
    if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
    else sha256sum "$1" | cut -d' ' -f1; fi
}

# comp_verify <id> <arşiv> <imza> [beklenen-sha256]
# Sırayla: kilit geçerli ve paket kilitte → arşiv özeti beklenenle aynı (verilmediyse kilitteki) → imza (ad alanı sshpocket-component)
# beklenen-sha256: yalnızca DOĞRULANMIŞ indeksten gelir (docs/package-index.md); imzacı yine KİLİTTEN alınır.
comp_verify() {
    local id="$1" archive="$2" sig="$3" want got signers tmp rc
    comp_id_valid "$id" || { echo "geçersiz paket kimliği" >&2; return 1; }
    comp_lock_validate || return 1
    comp_known "$id" || { echo "paket kilit dosyasında yok: $id" >&2; return 1; }
    want="${4:-}"
    if [ -n "$want" ]; then
        comp_sha256_valid "$want" || { echo "beklenen özet geçersiz ($id)" >&2; return 1; }
    else
        want=$(comp_get "$id" sha256)
    fi
    got=$(comp_sha256_of "$archive")
    if [ "$want" != "$got" ]; then echo "özet beklenenle uyuşmuyor ($id)" >&2; return 1; fi
    signers=$(comp_get "$id" signers)
    comp_signers_valid "$signers" || { echo "signers satırı geçersiz ($id)" >&2; return 1; }
    tmp=$(mktemp "${TMPDIR:-/tmp}/sshpocket-signers.XXXXXX") || return 1
    printf '%s\n' "$signers" > "$tmp"
    ssh-keygen -Y verify -f "$tmp" -I "$COMP_NAMESPACE" -n "$COMP_NAMESPACE" -s "$sig" < "$archive" >/dev/null 2>&1
    rc=$?
    rm -f "$tmp"
    if [ "$rc" -ne 0 ]; then echo "imza doğrulanamadı ($id)" >&2; return 1; fi
    return 0
}
