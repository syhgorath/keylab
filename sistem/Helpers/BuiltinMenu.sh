#!/usr/bin/env bash
# BuiltinMenu.sh - Ana menüdeki yerleşik eylemler (sunucu ekle / içe aktar / yönet)
# Gerektirir: Main.sh değişkenleri (SISTEM_PATH), RegisterModules, Autoload

# Yerleşik eylemleri menüye kaydeder (sunucu modüllerinden SONRA çağrılmalı)
RegisterBuiltinMenu() {
    RegisterModules "➕ Yeni sunucu ekle" "MainMenu" "AddServerAction"
    RegisterModules "📥 ~/.ssh/config'ten içe aktar" "MainMenu" "ImportSSHConfigAction"
    RegisterModules "🛠️  Sunucuları yönet (listele/sil/yeniden adlandır)" "MainMenu" "ManageServersAction"
    RegisterModules "⬆️  Güncellemeleri kontrol et / güncelle" "MainMenu" "UpdateAction"
    # Paketler yalnızca kilit dosyasında paket varsa menüde görünür
    if declare -F comp_ids >/dev/null && [ -n "$(comp_ids 2>/dev/null)" ]; then
        RegisterModules "📦 Paketler (durum / kur)" "MainMenu" "ComponentsAction"
    fi
    RegisterModules "⚙️  Ayarlar" "MainMenu" "SettingsAction"
}

# Kayıt defterini sıfırlayıp modülleri yeniden yükler (yeni/silinen modüller hemen yansısın)
ReloadModules() {
    # shellcheck disable=SC2034  # RegisterModules.sh içinde kullanılır
    MODULE_REGISTRY=""
    # shellcheck disable=SC2034
    MODULE_COUNT=0
    RegisterServers
    # Eski Modules/ klasörü yeni kurulumlarda yoktur; yokluğu uyarı gerektirmez (Main.sh de aynı korumayı yapar)
    if [ -d "$SISTEM_PATH/Modules" ]; then
        Autoload "$SISTEM_PATH/Modules" >/dev/null
    fi
    RegisterBuiltinMenu
}

# Fonksiyon adı yerleşik eylem mi? (--auto modunda atlanır)
IsBuiltinAction() {
    case "$1" in
        AddServerAction|ImportSSHConfigAction|ManageServersAction|UpdateAction|SettingsAction|ComponentsAction) return 0 ;;
        *) return 1 ;;
    esac
}

AddServerAction() {
    clear
    bash "$SISTEM_PATH/ModuleGenerator.sh"
    ReloadModules
}

ImportSSHConfigAction() {
    clear
    bash "$SISTEM_PATH/ImportSSHConfig.sh"
    ReloadModules
}

# _manage_pick_server → ok tuşlu / fzf ile bir sunucu seçtirir (stdout: ad; vazgeçilirse boş; seçilecek sunucu yoksa dönüş 1 = elle yazdır)
_manage_pick_server() {
    local n pick
    local -a names=()
    declare -F srv_names >/dev/null || return 1
    # shellcheck disable=SC2034  # ui_pick (dinamik kapsam) okur; üst menünün açıklamaları sunucu adlarına karışmasın
    local -a UI_PICK_DESCS=()
    # shellcheck disable=SC2034
    local UI_PICK_BACK_DESC="Hiçbir şey yapılmadan döner."
    while IFS= read -r n; do [ -n "$n" ] && names+=("$n"); done < <(srv_names)
    [ "${#names[@]}" -gt 0 ] || return 1
    pick=$(ui_pick "Vazgeç" "${names[@]}")
    if [ "${pick:-0}" -ge 1 ] 2>/dev/null && [ "$pick" -le "${#names[@]}" ]; then printf '%s' "${names[$((pick - 1))]}"; fi
    return 0
}

ManageServersAction() {
    local choice name new
    clear
    local fzf_card=0 srv_list=""
    if declare -F ui_card_on >/dev/null && ui_card_on; then fzf_card=1; fi
    if declare -F ui_screen >/dev/null; then ui_screen "Ana menü › Sunucuları yönet"; else echo "=== Sunucuları yönet ==="; fi
    srv_list=$(bash "$SISTEM_PATH/ModuleManager.sh" list)
    if [ "$fzf_card" = 0 ]; then echo ""; printf '%s\n' "$srv_list"; echo ""; fi
    if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=("Sunucu klasörünü (ayarlar ve içindeki key dosyaları) siler; sunucu adını yazarak onaylarsınız. Keychain kaydı için ayrıca sorulur." "Sunucuya yeni bir ad verir ve key dosyalarını yeni ada çevirir. Keychain kayıtları otomatik taşınmaz, yeniden girmeniz gerekir.")
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local UI_PICK_BACK_DESC="Ana menüye döner."
        if declare -F ui_card_on >/dev/null && ui_card_on; then
            # fzf: renkli kart, geniş düzen
            local names
            names=$(srv_names 2>/dev/null | tr '\n' ' ')
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            UI_PICK_DESCS=("$(ui_card_head '🗑️  Sil'; ui_card_row Sunucu "${names:-(yok)}"; printf '\n⚠ Sunucu klasörü (ayarlar ve key dosyaları) silinir.\nSunucu adını yazarak onaylarsınız; Keychain kaydı için ayrıca sorulur.\n')"
                "$(ui_card_head '✏️  Yeniden adlandır'; ui_card_row Sunucu "${names:-(yok)}"; printf '\nSunucuya yeni ad verir, key dosyaları yeni ada çevrilir.\n⚠ Keychain kayıtları otomatik taşınmaz.\n')")
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            local UI_PICK_WIDE=1 UI_PICK_HEIGHT=16 UI_PICK_HEADER=""
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            UI_PICK_HEADER=$(printf '%s\n' "$srv_list" | sed 's/^ *//' | head -n 8)   # sunucu listesi çizimin içinde (kısa terminalde ui_fit_wide kırpar)
            ui_fit_wide 3 6
        fi
        choice=$(ui_pick_arrows 1 "Geri" "Sil" "Yeniden adlandır")
        echo ""
    else
        echo "  1) Sil"
        echo "  2) Yeniden adlandır"
        echo "  0) Geri (q)"
        if ! read -r -p "Seçim [0]: " choice; then ui_no_pause; return 0; fi
    fi
    case "${choice:-0}" in
        0|q|Q) ui_no_pause ;;   # geri: okunacak çıktı yok
        1)
            if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled && name=$(_manage_pick_server); then :
            else read -r -p "Silinecek modül adı: " name; fi
            if [ -n "$name" ]; then bash "$SISTEM_PATH/ModuleManager.sh" remove "$name"; fi
            ;;
        2)
            if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled && name=$(_manage_pick_server); then :
            else read -r -p "Eski ad: " name; fi
            new=""
            if [ -n "$name" ]; then read -r -p "Yeni ad: " new; fi
            if [ -n "$name" ] && [ -n "$new" ]; then
                bash "$SISTEM_PATH/ModuleManager.sh" rename "$name" "$new"
            fi
            ;;
    esac
    ReloadModules
}

# Paketler: durumu göster, eksik/eski olanı onayla kur (kilitteki sürüm)
ComponentsAction() {
    clear
    if declare -F ui_header >/dev/null; then ui_header "Ana menü › Paketler"; fi
    echo ""
    comp_cli_list
    echo ""
    comp_cli_sync
}

# İmzalı güncelleme (ayrı süreç). Sürüm değiştiyse temiz bir yeniden başlatma yapılır.
UpdateAction() {
    local after
    clear
    bash "$SISTEM_PATH/Update.sh"
    after=$(head -n 1 "$SISTEM_PATH/../VERSION" 2>/dev/null | tr -d '[:space:]')
    if [ -n "$after" ] && [ "$after" != "$SCRIPT_VERSION" ]; then
        echo ""
        bash "$SISTEM_PATH/Main.sh" --components sync
        # Eski paket sürümleri güncellemelerde yan yana birikir: listeyi gösterir ve silinmesini sorar (varsayılan: silme)
        echo ""
        echo "🧹 Eski paket sürümleri"
        bash "$SISTEM_PATH/Main.sh" --components prune || true
        echo ""
        read -r -p "Yeni sürümle yeniden başlatmak için Enter..." _ || true
        exec bash "$SISTEM_PATH/Main.sh"
    fi
}
