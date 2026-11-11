#!/usr/bin/env bash
# ComponentsManifest.sh - Paket bildiriminin (component.json) okunması ve doğrulanması (Aşama 2.2)
# Doğrulama component_manifest.py'de (python3 -I, stdlib json); burada yalnızca çağrı ve erişimciler var.
# Çıktı düz "anahtar=değer" satırlarıdır; hiçbir değer eval/source edilmez.
# Gerektirir: Components.sh, ComponentsInstall.sh

COMP_MANIFEST_PY="${SISTEM_PATH:-.}/Helpers/component_manifest.py"

# comp_manifest_info <component.json> [beklenen-id] [beklenen-sürüm] → bilgi satırları; geçersizse hata
comp_manifest_info() {
    command -v python3 >/dev/null 2>&1 || { echo "python3 gerekli (bildirim doğrulama)" >&2; return 2; }
    [ -f "$COMP_MANIFEST_PY" ] || { echo "doğrulayıcı bulunamadı" >&2; return 2; }
    python3 -I "$COMP_MANIFEST_PY" "$@"
}

# comp_info_get <bilgi> <anahtar> → ilk eşleşen değer (örn. runner, version, inventory_group)
comp_info_get() {
    printf '%s\n' "$1" | awk -v k="$2" 'index($0, k "=") == 1 { print substr($0, length(k) + 2); exit }'
}

# comp_info_all <bilgi> <anahtar> → tüm eşleşen değerler (örn. action, profile, var)
comp_info_all() {
    printf '%s\n' "$1" | awk -v k="$2" 'index($0, k "=") == 1 { print substr($0, length(k) + 2) }'
}

# comp_action_get <bilgi> <eylem> <alan>  (alan: cmd effects sudo interactive confirm confirm_raised warnings requires requires_vars platforms)
comp_action_get() {
    comp_info_get "$1" "action.$2.$3"
}

# comp_action_on_platform <bilgi> <eylem> <platform> → 0 ise eylem bu platformda geçerli (platforms yoksa ya da platform bilinmiyorsa her zaman 0)
comp_action_on_platform() {
    local pl
    pl=$(comp_action_get "$1" "$2" platforms)
    { [ -z "$pl" ] || [ -z "$3" ]; } && return 0
    case ",$pl," in *",$3,"*) return 0 ;; esac
    return 1
}

# comp_prepare <id> → kurulu etkin sürümü bütünlükle doğrular, bildirimi doğrular, bilgi satırlarını yazar
comp_prepare() {
    local id="$1" ver
    comp_id_valid "$id" || { echo "geçersiz paket kimliği" >&2; return 1; }
    comp_lock_validate || return 1
    comp_known "$id" || { echo "paket kilit dosyasında yok: $id" >&2; return 1; }
    ver=$(comp_installed_version "$id")
    [ -n "$ver" ] || { echo "paket kurulu değil: $id" >&2; return 1; }
    if [ "$(comp_version_cmp "$ver" "$(comp_get "$id" version)")" = "-1" ]; then
        echo "kurulu sürüm kilit tabanının altında ($ver)" >&2; return 1
    fi
    comp_verify_installed "$id" "$ver" || return 1
    comp_manifest_info "$(comp_dir)/$id/$ver/component.json" "$id" "$ver"
}
