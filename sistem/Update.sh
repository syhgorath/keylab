#!/usr/bin/env bash
# Update.sh - İmzalı güncelleme (elle başlatılır; otomatik değildir)
#
#   bash sistem/Update.sh            kontrol et, onaylarsanız indir-doğrula-kur
#   bash sistem/Update.sh --check    yalnızca bak (çıkış kodu: 0 güncel, 10 yeni sürüm var, 1 hata)
#
# Güvenlik modeli:
#  * Yalnızca bu reponun GitHub Release'leri, yalnızca HTTPS.
#  * Arşiv, kurulu kopyadaki sistem/release_signers içindeki anahtarla imzalı olmalı
#    (ssh-keygen -Y verify). İmza doğrulanmadan hiçbir şey açılmaz. Anahtar yoksa güncelleme KAPALIDIR.
#  * Arşivdeki VERSION, istenen etiketle aynı olmalı (eski imzalı sürümün yeniden sunulmasını engeller).
#  * Geri sürüme düşmez. '..', mutlak yol ve symlink içeren arşiv reddedilir.
#  * Kullanıcı verisine (sistem/Modules/<ad>/, .env, key'ler) dokunulmaz; önce yedek alınır, hata olursa geri yüklenir.
#
# NOT: Bu dosya kendini güncelleyebildiği için, tüm mantık fonksiyonlarda ve dosyanın son satırı
# tek bir birleşik komuttur. Bash, çalışırken dosyanın geri kalanını okumaz.

UPDATE_REPO="syhgorath/sshpocket"
UPDATE_NAMESPACE="sshpocket-release"
UPDATE_SIGNER_ID="sshpocket-release"

UPDATE_SISTEM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPDATE_ROOT_DIR="$(dirname "$UPDATE_SISTEM_DIR")"
UPDATE_TMP=""

# Sürüm biçimi: X.Y.Z
update_version_valid() {
    [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

# İki sürümü karşılaştırır; 1 (a>b), 0 (eşit), -1 (a<b) yazdırır
update_version_cmp() {
    local -a a b
    local i
    IFS=. read -r -a a <<< "$1"
    IFS=. read -r -a b <<< "$2"
    for i in 0 1 2; do
        if [ "${a[$i]}" -gt "${b[$i]}" ]; then echo 1; return 0; fi
        if [ "${a[$i]}" -lt "${b[$i]}" ]; then echo -1; return 0; fi
    done
    echo 0
}

# HTTPS başlık isteği; Location değerini yazdırır (test için PATH'teki curl ile değiştirilebilir)
update_http_head() {
    curl -sSI --proto '=https' --tlsv1.2 --max-time 15 "$1" 2>/dev/null \
        | tr -d '\r' | awk 'tolower($1)=="location:" {print $2; exit}'
}

# HTTPS indirme (yönlendirmelerde de yalnızca HTTPS, boyut sınırlı)
update_download() {
    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 \
        --max-time 180 --max-filesize 52428800 -o "$2" "$1"
}

# En son yayınlanmış sürümü (v'siz) yazdırır; geçersizse hata
update_latest_version() {
    local loc tag
    loc=$(update_http_head "https://github.com/${UPDATE_REPO}/releases/latest")
    [ -n "$loc" ] || return 1
    tag="${loc##*/}"
    [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    # Yönlendirme tam olarak bu reponun etiket adresi olmalı
    [ "$loc" = "https://github.com/${UPDATE_REPO}/releases/tag/${tag}" ] || return 1
    printf '%s' "${tag#v}"
}

# İmza doğrulama. Parametreler: arşiv, imza dosyası, allowed_signers dosyası
update_verify() {
    local archive="$1" sig="$2" signers="$3"
    if [ ! -s "$signers" ] || ! grep -q '^[^#].*ssh-' "$signers" 2>/dev/null; then
        echo "imza anahtarı tanımlı değil ($signers)" >&2
        return 1
    fi
    ssh-keygen -Y verify -f "$signers" -I "$UPDATE_SIGNER_ID" -n "$UPDATE_NAMESPACE" \
        -s "$sig" < "$archive" >/dev/null 2>&1
}

# Arşiv içeriğini kurmadan önce denetler. Parametreler: arşiv, sürüm
update_check_archive() {
    local archive="$1" version="$2" prefix names verbose
    prefix="sshpocket-${version}"
    names=$(tar -tzf "$archive" 2>/dev/null) || { echo "arşiv okunamadı" >&2; return 1; }
    [ -n "$names" ] || { echo "arşiv boş" >&2; return 1; }

    local line
    while IFS= read -r line; do
        case "$line" in
            /*)          echo "mutlak yol: $line" >&2; return 1 ;;
            *..*)        # '..' bileşeni (dosya adında '..' geçenleri de reddetmek güvenli taraf)
                         echo "şüpheli yol: $line" >&2; return 1 ;;
            "$prefix"|"$prefix"/*) ;;
            *)           echo "beklenmeyen üst dizin: $line" >&2; return 1 ;;
        esac
    done <<< "$names"

    verbose=$(tar -tvzf "$archive" 2>/dev/null | cut -c1)
    case "$verbose" in
        *l*|*h*) echo "arşivde symlink/hardlink var" >&2; return 1 ;;
    esac
    return 0
}

# CHANGELOG'dan ilgili sürümün bölümünü yazdırır. Parametreler: changelog, sürüm
update_release_notes() {
    awk -v v="$2" '
        $0 ~ "^## \\[" v "\\]" { on=1; print; next }
        on && /^## \[/ { exit }
        on { print }
    ' "$1" | head -n 40
}

# Kilit dosyasındaki paket sürümleri: "kimlik sürüm" satırları (yorum satırları ve bozuk bölümler atlanır). Parametre: components.lock
update_lock_versions() {
    [ -f "$1" ] || return 0
    awk '
        /^\[[a-z][a-z0-9-]*\][[:space:]]*$/ { id=$0; gsub(/[][[:space:]]/, "", id); next }
        id != "" && /^version[[:space:]]*=/ { v=$0; sub(/^version[[:space:]]*=[[:space:]]*/, "", v); gsub(/[[:space:]\r]+$/, "", v); print id " " v; id="" }
    ' "$1"
}

# Eski ve yeni kilit arasındaki fark: "kimlik eski yeni" satırları (olmayan taraf "-"). Parametreler: eski kilit, yeni kilit
update_lock_diff() {
    { update_lock_versions "$1" | sed 's/^/o /'; update_lock_versions "$2" | sed 's/^/n /'; } | awk '
        $1 == "o" { o[$2]=$3; ids[$2]=1 }
        $1 == "n" { n[$2]=$3; ids[$2]=1 }
        END { for (i in ids) print i " " ((i in o) ? o[i] : "-") " " ((i in n) ? n[i] : "-") }
    ' | sort
}

# CHANGELOG'daki ilgili sürümün "### Dikkat" bölümü (en çok 12 satır; denetim karakterleri atılır). Parametreler: changelog, sürüm
update_attention() {
    awk -v v="$2" '
        $0 ~ "^## \\[" v "\\]" { in_v=1; next }
        in_v && /^## \[/ { exit }
        in_v && /^### / { on = ($0 ~ /^### Dikkat/); next }
        in_v && on && NF { print }
    ' "$1" | tr -d '\000-\010\013-\037' | head -n 12
}

# Özetin sol sütunu (düz metin satırları): sürüm, imza, paket farkları. Parametreler: yeni ağaç, hedef kök, eski sürüm, yeni sürüm
update_summary_text() {
    local tree="$1" root="$2" oldver="$3" newver="$4" id o n same=0
    printf '%-9s %s → %s\n' "sshpocket" "$oldver" "$newver"
    printf '%-9s %s\n' "imza" "✅ doğrulandı"
    while read -r id o n; do
        [ -n "$id" ] || continue
        if [ "$o" = "$n" ]; then same=$((same + 1)); continue; fi
        case "$o" in -) printf '%-9s %s %s (yeni)\n' "paket" "$id" "$n" ;; *)
            case "$n" in -) printf '%-9s %s %s (kilitten çıkıyor)\n' "paket" "$id" "$o" ;; *) printf '%-9s %s %s → %s\n' "paket" "$id" "$o" "$n" ;; esac ;;
        esac
    done < <(update_lock_diff "$root/components.lock" "$tree/components.lock")
    [ "$same" -eq 0 ] || printf '%-9s %s paket değişmiyor\n' "paket" "$same"
}

# Kurulum ÖNCESİ özet, düz akış (imza ve arşiv denetimi geçtikten sonra; hiçbir şey yazmaz). Parametreler update_summary_text ile aynı
update_summary() {
    local att line
    echo ""
    echo "──── Güncelleme özeti ────"
    while IFS= read -r line; do echo "  $line"; done < <(update_summary_text "$@")
    att=$(update_attention "$1/CHANGELOG.md" "$4")
    if [ -n "$att" ]; then
        echo "  ⚠️  Dikkat:"
        while IFS= read -r line; do echo "     $line"; done <<< "$att"
    fi
    echo "───────────────────────────"
}

# Bölgeli panel (ortak UI): üstte sabit özet (solda özet, sağda Dikkat), altta notlar/sorular/kurulum kaydırma bölgesinde.
# Yalnızca terminalde (ya da SSHPOCKET_ROWS/COLS testte) ve UI.sh yüklenebiliyorsa; dar/kısa terminalde ya da tty yoksa 1 döner (düz akış).
# UI.sh burada, kurulum başlamadan ÖNCE okunur (kurulum dosyaları değiştirse de bellekteki işlevler çalışır).
update_panel_begin() {
    local left right
    { [ -t 1 ] || [ -n "${SSHPOCKET_ROWS:-}" ]; } || return 1
    if ! declare -F ui_region_begin2 >/dev/null; then
        # shellcheck source=/dev/null
        source "$UPDATE_SISTEM_DIR/Helpers/UI.sh" 2>/dev/null || return 1
        declare -F ui_region_begin2 >/dev/null || return 1
    fi
    left=$(update_summary_text "$1" "$2" "$3" "$4")
    right=$(update_attention "$1/CHANGELOG.md" "$4")
    [ -n "$right" ] || right="Bu sürümde ayrıca dikkat edilecek not yok."
    ui_region_begin2 "Güncelleme · sshpocket $3 → $4" "$left" "$right" "Özet" "Dikkat" "Sürüm notları ve kurulum" || return 1
    # Özet üst bölüme sığmadıysa hiçbir bilgi kaybolmasın: tamamı kaydırma bölgesinin başında düz yazılır
    [ "${UI_REGION_TRUNC:-0}" != 1 ] || update_summary "$1" "$2" "$3" "$4"
}

update_panel_end() { declare -F ui_region_reset >/dev/null && ui_region_reset; return 0; }

# Yedek: kod dosyalarını tar.gz olarak saklar. Parametreler: hedef kök, sürüm
update_backup() {
    local root="$1" version="$2" f dest
    local -a items=()
    dest="$root/.update-backup"
    mkdir -p "$dest" || return 1
    for f in start.sh start.command VERSION CHANGELOG.md README.md README.en.md LICENSE ROADMAP.md \
             CONTRIBUTING.md SECURITY.md RELEASING.md sistem/Helpers sistem/templates sistem/release_signers; do
        [ -e "$root/$f" ] && items+=("$f")
    done
    for f in "$root"/sistem/*.sh; do
        [ -e "$f" ] && items+=("sistem/$(basename "$f")")
    done
    # .env ve key'ler yedeğe girmez (yedek yalnızca kod içindir)
    tar -czf "$dest/code-${version}.tar.gz" --exclude='.env' --exclude='id_*' --exclude='*.pem' \
        -C "$root" "${items[@]}" || return 1
    printf '%s' "$dest/code-${version}.tar.gz"
}

# Yeni ağacı kurar. Kod (sistem/*.sh, Helpers, templates) bütünüyle değiştirilir.
# sistem/Modules/ (henüz taşınmamış eski sunucular) ve servers/ kullanıcı verisidir: ASLA silinmez/ezilmez.
# Parametreler: yeni ağaç, hedef kök
update_install_tree() {
    local tree="$1" root="$2" f d name
    # kök dosyalar ve kod dışı klasörler
    for f in "$tree"/*; do
        name=$(basename "$f")
        if [ -f "$f" ]; then
            cp -p "$f" "$root/$name" || return 1
        elif [ -d "$f" ]; then
            case "$name" in
                sistem|.git|usb_logs|.update-backup|servers) ;;
                *) rm -r -- "${root:?}/${name:?}" 2>/dev/null; cp -R "$f" "$root/$name" || return 1 ;;
            esac
        fi
    done
    # sistem/: eski kodu temizle, yenisini kur (Modules/ hariç)
    if [ -d "$tree/sistem" ]; then
        mkdir -p "$root/sistem" || return 1
        rm -r -- "${root:?}/sistem/Helpers" 2>/dev/null
        rm -r -- "${root:?}/sistem/templates" 2>/dev/null
        for f in "$root"/sistem/*.sh; do [ -e "$f" ] && rm -f -- "$f"; done
        for f in "$tree"/sistem/*; do
            name=$(basename "$f")
            if [ "$name" = "Modules" ]; then
                # Arşivde eski modül varsa (eski sürümler) birleştir; kullanıcı dosyaları silinmez
                for d in "$f"/*; do
                    [ -d "$d" ] || continue
                    mkdir -p "$root/sistem/Modules/$(basename "$d")" || return 1
                    cp -R "$d/." "$root/sistem/Modules/$(basename "$d")/" || return 1
                done
            else
                cp -R "$f" "$root/sistem/$name" || return 1
            fi
        done
    fi
    update_cleanup_stale_example "$root"
    return 0
}

# Eski sürümlerin gönderdiği, hiç yapılandırılmamış 'example' şablonunu (yalnızca ExampleModule.sh/.env.example) kaldırır.
# İçinde .env, key (id_*) ya da başka bir kullanıcı dosyası varsa DOKUNMAZ.
update_cleanup_stale_example() {
    local d="$1/sistem/Modules/example" f extra=0
    [ -d "$d" ] || return 0
    for f in "$d"/* "$d"/.[!.]*; do
        [ -e "$f" ] || continue
        case "$(basename "$f")" in
            ExampleModule.sh|.env.example|.DS_Store) ;;
            *) extra=1 ;;
        esac
    done
    if [ "$extra" = "0" ]; then
        rm -r -- "${d:?}"
        rmdir "$1/sistem/Modules" 2>/dev/null || true
    fi
}

# Parametreler: yeni ağaç, hedef kök, eski sürüm. Hata olursa yedekten geri yükler.
update_apply() {
    local tree="$1" root="$2" oldver="$3" backup
    if [ ! -w "$root" ] || { [ -d "$root/sistem" ] && [ ! -w "$root/sistem" ]; }; then
        echo "hedef yazılabilir değil (USB yazma korumalı olabilir): $root" >&2
        return 1
    fi
    backup=$(update_backup "$root" "$oldver") || { echo "yedek alınamadı" >&2; return 1; }
    if ! update_install_tree "$tree" "$root"; then
        echo "⚠️  Kurulum başarısız, yedekten geri yükleniyor..." >&2
        tar -xzf "$backup" -C "$root" 2>/dev/null && echo "↩️  Eski sürüm geri yüklendi." >&2
        return 1
    fi
    echo "💾 Yedek: $backup"
    return 0
}

update_cleanup() {
    if [ -n "$UPDATE_TMP" ] && [ -d "$UPDATE_TMP" ]; then
        rm -r -- "${UPDATE_TMP:?}"
    fi
}

update_confirm() {
    local answer
    read -r -p "$1 (y/n) " answer || return 1
    case "$answer" in y|Y) return 0 ;; *) return 1 ;; esac
}

# Çıkış kodları: 0 güncel/başarılı, 10 yeni sürüm var (--check), 1 hata/iptal
update_main() {
    local check_only=0
    case "${1:-}" in --check|--check-only) check_only=1 ;; esac

    local cur latest cmp
    cur=$(head -n 1 "$UPDATE_ROOT_DIR/VERSION" 2>/dev/null | tr -d '[:space:]')
    if ! update_version_valid "$cur"; then
        echo "⚠️  Mevcut sürüm okunamadı (VERSION)"; return 1
    fi

    echo "Mevcut sürüm: $cur"
    echo "🔍 Son sürüm kontrol ediliyor (github.com/${UPDATE_REPO})..."
    if ! latest=$(update_latest_version); then
        echo "⚠️  Son sürüm öğrenilemedi (ağ yok ya da yayınlanmış sürüm yok)."
        return 1
    fi
    cmp=$(update_version_cmp "$latest" "$cur")
    if [ "$cmp" = "0" ]; then echo "✅ Güncelsiniz ($cur)."; return 0; fi
    if [ "$cmp" = "-1" ]; then
        echo "ℹ️  Uzaktaki sürüm ($latest) kurulu olandan eski; geri sürüme düşülmez."
        return 0
    fi
    echo "🆕 Yeni sürüm var: $latest (mevcut: $cur)"
    if [ "$check_only" = "1" ]; then return 10; fi

    update_confirm "İndirilip imzası doğrulansın mı?" || { echo "İptal edildi."; return 1; }

    UPDATE_TMP=$(mktemp -d) || return 1
    trap update_cleanup EXIT
    local base archive sig
    base="https://github.com/${UPDATE_REPO}/releases/download/v${latest}"
    archive="$UPDATE_TMP/sshpocket-${latest}.tar.gz"
    sig="${archive}.sig"

    echo "⬇️  İndiriliyor..."
    if ! update_download "$base/sshpocket-${latest}.tar.gz" "$archive" \
        || ! update_download "$base/sshpocket-${latest}.tar.gz.sig" "$sig"; then
        echo "⚠️  İndirme başarısız"; return 1
    fi

    echo "🔐 İmza doğrulanıyor..."
    if ! update_verify "$archive" "$sig" "$UPDATE_SISTEM_DIR/release_signers"; then
        echo "❌ İMZA DOĞRULANAMADI. Paket KURULMADI. (Sahte veya bozuk paket olabilir.)"
        return 1
    fi
    echo "✅ İmza geçerli."

    update_check_archive "$archive" "$latest" || { echo "❌ Arşiv güvenli değil, kurulmadı."; return 1; }
    mkdir "$UPDATE_TMP/x"
    if ! tar -xzf "$archive" -C "$UPDATE_TMP/x"; then
        echo "⚠️  Arşiv açılamadı"; return 1
    fi
    local tree="$UPDATE_TMP/x/sshpocket-${latest}" inner
    inner=$(head -n 1 "$tree/VERSION" 2>/dev/null | tr -d '[:space:]')
    if [ "$inner" != "$latest" ]; then
        echo "❌ Arşivdeki sürüm ($inner) istenenle ($latest) uyuşmuyor, kurulmadı."
        return 1
    fi

    local panel=0 rc
    if update_panel_begin "$tree" "$UPDATE_ROOT_DIR" "$cur" "$latest"; then panel=1; else update_summary "$tree" "$UPDATE_ROOT_DIR" "$cur" "$latest"; fi
    update_install_flow "$tree" "$cur" "$latest"
    rc=$?
    [ "$panel" = 0 ] || update_panel_end
    return "$rc"
}

# Notlar, kurulum onayı ve kurulum (tek çıkış noktası: panel her durumda kapatılabilsin). Parametreler: yeni ağaç, eski sürüm, yeni sürüm
update_install_flow() {
    local tree="$1" cur="$2" latest="$3"
    echo ""
    echo "──── Sürüm notları ────"
    update_release_notes "$tree/CHANGELOG.md" "$latest"
    echo "───────────────────────"
    update_confirm "Kurulsun mu? (sunucularınız/key'leriniz korunur, yedek alınır)" || { echo "İptal edildi."; return 1; }

    update_apply "$tree" "$UPDATE_ROOT_DIR" "$cur" || { echo "❌ Güncelleme başarısız."; return 1; }
    echo ""
    echo "✅ Güncellendi: $cur → $latest. sshpocket'ı yeniden başlatın."
    return 0
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then update_main "$@"; exit $?; fi
