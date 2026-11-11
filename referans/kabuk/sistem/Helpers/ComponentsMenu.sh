#!/usr/bin/env bash
# ComponentsMenu.sh - Sunucu menüsünde "🛡️ Paketler": paket seç → eylem seç → comp_cli_run (Aşama 3, PR E1)
# Eylemin kendisi comp_cli_run → comp_exec_action zincirinde çalışır (kapılar/onaylar orada); burada yalnızca seçim vardır.
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsManifest.sh, ComponentsGates.sh (comp_gate_required_applies), ComponentsRunCLI.sh, Servers.sh, UI.sh (isteğe bağlı: ui_pick_arrows)

# comp_menu_installed → kurulu paket kimlikleri (satır satır)
comp_menu_installed() {
    local id cat="${1:-}" ord
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        [ -n "$(comp_installed_version "$id")" ] || continue
        if [ -n "$cat" ] && [ "$(comp_menu_category "$id")" != "$cat" ]; then continue; fi
        ord=$(comp_info_get "$(_comp_menu_info_of "$id")" menu.order 2>/dev/null)
        printf '%s|%s\n' "${ord:-500}" "$id"
    done < <(comp_ids 2>/dev/null) | sort -t'|' -k1,1n -s | cut -d'|' -f2
    return 0
}

# Sunucu menüsündeki sabit kategoriler (sıra numarası, kimlik, simge+ad). Bildirimde menu.category yoksa "paketler".
COMP_MENU_CATEGORIES=(
    "10|hardening|🛡️  Sertleştirme"
    "20|kurulum|🧰 Kurulum merkezi"
    "90|paketler|🛡️  Paketler (sertleştirme vb.)"
)

# _comp_menu_info_of <paket> → kurulu sürümün bildirim satırları (bütünlük doğrulaması yok: yalnız sınıflandırma/ad; eylemler comp_prepare ile doğrulanır)
_comp_menu_info_of() {
    local ver
    ver=$(comp_installed_version "$1")
    [ -n "$ver" ] && [ -f "$(comp_dir)/$1/$ver/component.json" ] || return 1
    comp_manifest_info "$(comp_dir)/$1/$ver/component.json" "$1" "$ver" 2>/dev/null
}

# comp_menu_category <paket> → kategori kimliği (menu.category ya da paketler)
comp_menu_category() {
    local c
    c=$(comp_info_get "$(_comp_menu_info_of "$1")" menu.category 2>/dev/null)
    printf '%s' "${c:-paketler}"
}

# comp_menu_title <paket> → menu.title ya da paket kimliği
comp_menu_title() {
    local t
    t=$(comp_info_get "$(_comp_menu_info_of "$1")" menu.title 2>/dev/null)
    printf '%s' "${t:-$1}"
}

# comp_menu_categories → kurulu paketi olan kategoriler, sıra numarasına göre: "kimlik|simge+ad" satırları
comp_menu_categories() {
    local row cat
    for row in "${COMP_MENU_CATEGORIES[@]}"; do
        cat=$(printf '%s' "$row" | cut -d'|' -f2)
        [ -n "$(comp_menu_installed "$cat")" ] && printf '%s|%s\n' "$cat" "${row#*|*|}"
    done
    return 0
}

# comp_menu_available <sunucu-klasörü> → 0: yeni biçimli sunucu ve en az bir kurulu paket var
comp_menu_available() {
    srv_is_current "$1" 2>/dev/null || return 1
    [ -n "$(comp_menu_installed)" ]
}

# _comp_menu_badge <bilgi> <eylem> → "🔍 salt okunur" | "✎ değişiklik yapar" | "☠ yıkıcı" (değişkenlik seçicisi olmayan düz semboller: emoji genişliği sarmayı bozar)
_comp_menu_badge() {
    local eff
    eff=$(comp_action_get "$1" "$2" effects)
    case ",$eff," in
        ",,") printf '🔍 salt okunur' ;;
        *,destructive,*) printf '☠ yıkıcı' ;;
        *) printf '✎ değişiklik yapar' ;;
    esac
}

# _comp_menu_pick <başlık-geri> <öğe>... → UI.sh ui_pick'e devreder (seçilen numara; geri/hata = 0)
_comp_menu_pick() { ui_pick "$@"; }

# _comp_menu_ctx <paket> <sunucu> <bilgi> → COMP_MENU_VER/PKG/MF/DIR/VARS (dinamik kapsam)
_comp_menu_ctx() {
    COMP_MENU_VER=$(comp_info_get "$3" version)
    COMP_MENU_PKG="$(comp_dir)/$1/$COMP_MENU_VER"
    COMP_MENU_MF="$COMP_MENU_PKG/component.json"
    COMP_MENU_DIR="$(servers_dir)/$2"
    COMP_MENU_VARS=$(comp_vars_file "$COMP_MENU_DIR" "$1")
}

# _comp_menu_forced <bilgi> → çalıştırıcının zorladığı değişkenler (outputs.dir_var), satır satır
_comp_menu_forced() {
    local n d
    while IFS= read -r n; do
        [ -n "$n" ] || continue
        d=$(comp_action_get "$1" "$n" outputs.dir_var)
        [ -n "$d" ] && printf '%s\n' "$d"
    done < <(comp_info_all "$1" action) | sort -u
}

# _comp_menu_field <TSV-satırı> <n> → n. alan (boş alanlar korunur)
_comp_menu_field() { printf '%s' "$1" | awk -F'\t' -v n="$2" '{ print $n }'; }

# _comp_menu_unq <değer> → JSON dizgesinin çevreleyen tırnaklarını soyar
_comp_menu_unq() { local v="$1"; if [[ "$v" == \"*\" ]]; then v="${v#\"}"; v="${v%\"}"; fi; printf '%s' "$v"; }

# _comp_menu_age <saniye> → "N sn/dk/sa/gün önce"
_comp_menu_age() {
    local s="$1"
    if [ "$s" -lt 60 ]; then printf '%s sn önce' "$s"
    elif [ "$s" -lt 3600 ]; then printf '%s dk önce' $((s / 60))
    elif [ "$s" -lt 86400 ]; then printf '%s sa önce' $((s / 3600))
    else printf '%s gün önce' $((s / 86400)); fi
}

# _comp_menu_req_label <paket> <sunucu> <sürüm> <requires-csv> → "preview ✅ 12 dk önce, ..." (taze), "⏳ süresi doldu" ya da yalnızca ad
_comp_menu_req_label() {
    local id="$1" server="$2" ver="$3" csv="$4" r age out="" sep=""
    local -a reqs
    IFS=',' read -r -a reqs <<< "$csv"
    for r in ${reqs[@]+"${reqs[@]}"}; do
        if age=$(comp_gate_requires_age "$id" "$server" "$ver" "$r"); then
            if [ "$age" -le "$COMP_REQUIRES_TTL" ]; then out="$out$sep$r ✅ $(_comp_menu_age "$age")"
            else out="$out$sep$r ⏳ süresi doldu"; fi
        else out="$out$sep$r"; fi
        sep=", "
    done
    printf '%s' "$out"
}

# _comp_menu_status_lines <paket> <sunucu> <bilgi> → durum (outputs.format) eylemlerinin son bilinen sonucu, satır satır
_comp_menu_status_lines() {
    local id="$1" server="$2" info="$3" name last ts rest now
    now=$(date +%s)
    while IFS= read -r name; do
        [ -n "$name" ] || continue
        [ "$(comp_action_get "$info" "$name" outputs.format)" = "keyvalue-line" ] || continue
        if last=$(comp_exec_last_result "$id" "$server" "$name"); then
            ts=$(printf '%s\n' "$last" | head -n 1)
            rest=$(printf '%s\n' "$last" | tail -n +2 | tr '\n' ' ')
            if [[ "$ts" =~ ^[0-9]+$ ]] && [ "$ts" -le "$now" ]; then
                printf '  ℹ️  %s (%s): %s\n' "$name" "$(_comp_menu_age $((now - ts)))" "${rest% }"
            else printf '  ℹ️  %s: %s\n' "$name" "${rest% }"; fi
        else
            printf '  ℹ️  %s: henüz çalıştırılmadı\n' "$name"
        fi
    done < <(comp_info_all "$info" action)
}

# _comp_menu_score_lines <paket> <sunucu> → araç başına son denetim puanı ve bir öncekiyle fark (yalnızca puan); veri yoksa hiçbir şey yazmaz
# Veri: <durum>/<paket>/<sunucu>/**/summary.json (component_scores.py; ortak biçim docs/package-standard.md "Denetim özeti")
_comp_menu_score_lines() {
    local id="$1" server="$2" py tool epoch score now delta latest tool_prev out=""
    py="${COMP_SCORES_PY:-${SISTEM_PATH:-.}/Helpers/component_scores.py}"
    command -v python3 >/dev/null 2>&1 && [ -f "$py" ] || return 0
    now=$(date +%s)
    tool_prev=""
    while IFS=$'\t' read -r tool epoch score; do
        [ -n "$tool" ] || continue
        if [ "$tool" != "$tool_prev" ]; then
            [ -n "$out" ] && printf '%s\n' "$out"
            latest="$score"; out="  📊 $tool: $score"
            if [[ "$epoch" =~ ^[0-9]+$ ]] && [ "$epoch" -le "$now" ]; then out="$out ($(_comp_menu_age $((now - epoch))))"; fi
            tool_prev="$tool"
        else
            delta=$(awk -v a="$latest" -v b="$score" 'BEGIN{d=a-b; if (d>0) printf "+%g", d; else if (d<0) printf "%g", d; else printf "±0"}')
            out="$out · önceki $score ($delta)"
        fi
    done < <(python3 -I "$py" "$(_comp_gate_state_dir)" "$id" "$server" 2>/dev/null)
    [ -n "$out" ] && printf '%s\n' "$out"
    return 0
}

# ---- Bulgular (denetim eylemi bulgular.tsv yazarsa): sayı özeti hazırlıkta, liste ve kartlar "🔎 Bulgular" ekranında ----
# _comp_menu_findings_file <bilgi> → manifestte outputs.findings bildiren ilk eylemin dosya adı (yoksa boş)
_comp_menu_findings_file() {
    local a f
    while IFS= read -r a; do
        [ -n "$a" ] || continue
        f=$(comp_action_get "$1" "$a" outputs.findings)
        [ -z "$f" ] || { printf '%s\n' "$f"; return 0; }
    done < <(comp_info_all "$1" action)
    return 0
}
# _comp_menu_findings_load <paket> <sunucu> <bilgi> → COMP_FINDINGS_META (başlık satırı alanları) ve COMP_FINDINGS_ROWS (10 sütunlu satırlar)
# dönüş: 0 veri var · 1 bulgu dosyası yok/bildirilmemiş · 3 dosya var ama başlık bozuk
COMP_FINDINGS_META=""; COMP_FINDINGS_ROWS=()
_comp_menu_findings_load() {
    local f py out rc line first=1
    COMP_FINDINGS_META=""; COMP_FINDINGS_ROWS=()
    f=$(_comp_menu_findings_file "$3"); [ -n "$f" ] || return 1
    py="${COMP_FINDINGS_PY:-${SISTEM_PATH:-.}/Helpers/component_findings.py}"
    command -v python3 >/dev/null 2>&1 && [ -f "$py" ] || return 1
    out=$(python3 -I "$py" "$(_comp_gate_state_dir)" "$1" "$2" "$f" 2>/dev/null); rc=$?
    [ "$rc" -eq 3 ] && return 3
    [ -n "$out" ] || return 1
    while IFS= read -r line; do
        if [ "$first" -eq 1 ]; then COMP_FINDINGS_META="$line"; first=0; else COMP_FINDINGS_ROWS+=("$line"); fi
    done <<< "$out"
    return 0
}
# _comp_menu_findings_lines <paket> <sunucu> <bilgi> → hazırlık özetine tek satır; bulgu yoksa hiçbir şey yazmaz
_comp_menu_findings_lines() {
    local rc _h epoch total skipped trunc mapped now txt
    _comp_menu_findings_load "$1" "$2" "$3"; rc=$?
    if [ "$rc" -eq 3 ]; then echo "  🔎 Bulgular okunamadı (dosyanın başlık satırı beklenen biçimde değil)"; return 0; fi
    [ "$rc" -eq 0 ] || return 0
    IFS=$'\037' read -r _h epoch total skipped trunc mapped _ <<< "$COMP_FINDINGS_META"
    now=$(date +%s)
    txt="  🔎 Bulgular: $total"
    [ "$mapped" = 0 ] || txt="$txt ($mapped tanesi paket seçeneğiyle eşleşir)"
    if [[ "$epoch" =~ ^[0-9]+$ ]] && [ "$epoch" -le "$now" ]; then txt="$txt · $(_comp_menu_age $((now - epoch)))"; fi
    [ "$skipped" = 0 ] || txt="$txt · $skipped satır atlandı"
    [ "$trunc" = 0 ] || txt="$txt · liste kesildi"
    printf '%s\n' "$txt"
}
# _comp_menu_findings_card <satır> → tek bulgunun kartı (çok satırlı)
_comp_menu_findings_card() {
    local kimlik risk tur acik ayr oneri alan deger kapanir platform
    IFS=$'\037' read -r kimlik risk tur acik ayr oneri alan deger kapanir platform <<< "$1"
    printf '%s — %s · önem: %s\n' "$kimlik" "$([ "$tur" = uyari ] && echo uyarı || echo öneri)" "$risk"
    _comp_menu_wrap 46 "  " "$acik"
    [ -z "$ayr" ] || { printf 'Ayrıntı:\n'; _comp_menu_wrap 46 "  " "  $ayr"; }
    [ -z "$oneri" ] || { printf 'Öneri:\n'; _comp_menu_wrap 46 "  " "  ${oneri#text:}"; }
    if [ -n "$alan" ]; then
        printf 'Paket karşılığı: %s' "$alan"; [ -z "$deger" ] || printf ' → %s' "$deger"; printf '\n'
        case "$kapanir" in
            evet) printf '  Paket varsayılanıyla kapanır (apply yeterli)\n' ;;
            hayir) printf '  Kapanması için değişkeni elle ayarlayın (📝 Değişkenler)\n' ;;
        esac
    else printf 'Paket karşılığı: yok (elle ele alınır)\n'; fi
    [ "$platform" = hepsi ] || printf 'Platform: %s\n' "$platform"
}
# comp_menu_findings <paket> <sunucu> <bilgi> → bulgu listesi (fzf: sağ bölmede kart; yalnız eşlemeli süzgeci)
comp_menu_findings() {
    local id="$1" server="$2" info="$3" rc row pick i n only=0 acik kimlik risk alan
    local -a sel labels
    while :; do
        _comp_menu_findings_load "$id" "$server" "$info"; rc=$?
        [ "$rc" -eq 0 ] || { echo "Bulgu listesi yok (önce denetim eylemini çalıştırın)."; return 0; }
        sel=(); labels=(); n=0
        for row in "${COMP_FINDINGS_ROWS[@]}"; do
            IFS=$'\037' read -r kimlik risk _ acik _ _ alan _ <<< "$row"
            [ "$only" -eq 0 ] || [ -n "$alan" ] || continue
            sel+=("$row"); n=$((n + 1))
            labels+=("$(printf '%-9s %-10s %s%s' "$kimlik" "$risk" "${acik:0:60}" "$([ -z "$alan" ] || echo '  ✎')")")
        done
        # shellcheck disable=SC2034  # ui_pick (dinamik kapsam) okur
        local -a UI_PICK_DESCS=("Yalnızca paket seçeneğiyle eşleşen (✎) bulguları gösterir ya da hepsine döner.")
        for row in "${sel[@]}"; do UI_PICK_DESCS+=("$(_comp_menu_findings_card "$row")"); done
        # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
        local UI_PICK_WIDE=1 UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_START=2 UI_PICK_TITLE="Sunucu › $server › Paketler › $id › Bulgular" UI_PICK_HEADER=""
        UI_PICK_HEADER=$(_comp_menu_findings_lines "$id" "$server" "$info" | sed 's/^ *//')
        ui_fit_wide "$((n + 2))" 10   # liste en az 10 satır; başlık kısa terminalde kırpılır
        pick=$(ui_pick "Geri" "Yalnız eşlemeli: $([ "$only" -eq 1 ] && echo açık || echo kapalı) ($n gösterilen)" "${labels[@]}")
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        if [ "$pick" -eq 1 ]; then only=$((1 - only)); continue; fi
        if ! _comp_menu_interactive; then
            i=$((pick - 2)); echo ""; _comp_menu_findings_card "${sel[$i]}"; echo ""
            read -r -p "Devam için Enter..." _ || return 0
        fi
    done
}

# _comp_menu_action_blocked <bilgi> <eylem> <eksik-değişkenler> → 0 ise eylem kapalı: zorunlu değişken eksik VE eylem değiştirici ya da
# başka bir eylemin ön koşulu. Salt okunur durum eylemleri (rollback_status) zorunlu değişken olmadan da açıktır (CLI ile aynı kural).
_comp_menu_action_blocked() {
    { [ -n "$3" ] && comp_gate_required_applies "$1" "$2"; } || [ -n "$(comp_gate_unmet_vars "$1" "$2" "${COMP_MENU_VARS:-/dev/null}")" ]
}

# _comp_menu_wrap <genişlik> <girinti> <metin> → metni sözcük sınırından satırlara böler; devam satırları girintilenir
_comp_menu_wrap() {
    local w="$1" ind="$2" line="" word out=""
    set -f
    for word in $3; do
        if [ -n "$line" ] && [ $((${#line} + ${#word} + 1)) -gt "$w" ]; then out+="$line"$'\n'; line="$ind$word"
        else line="${line:+$line }$word"; fi
    done
    set +f
    printf '%s\n' "$out$line"
}

# _comp_menu_action_card_head <bilgi> <paket> <sunucu> <eylem> <eksik> → kartın üst yarısı: ad, etkiler, onay, ön koşul, kapalı uyarısı
_comp_menu_action_card_head() {
    local info="$1" id="$2" server="$3" name="$4" missing="$5" eff req conf um
    eff=$(comp_action_get "$info" "$name" effects); req=$(comp_action_get "$info" "$name" requires); conf=$(comp_action_get "$info" "$name" confirm)
    printf '%s  —  %s\n' "$name" "$(_comp_menu_badge "$info" "$name")"
    printf 'Etkiler: %s\n' "${eff:-yok (salt okunur)}"
    printf 'Onay: %s\n' "${conf:-yok}"
    [ -z "$req" ] || printf 'Ön koşul: %s\n' "$(_comp_menu_req_label "$id" "$server" "$COMP_MENU_VER" "$req")"
    if _comp_menu_action_blocked "$info" "$name" "$missing"; then printf '⛔ Kapalı: önce zorunlu değişkenleri tamamlayın\n'; fi
    um=$(comp_gate_unmet_vars "$info" "$name" "${COMP_MENU_VARS:-/dev/null}" | tr '\n' ' ')
    [ -z "${um// }" ] || printf '   Boş: %s (Değişkenler ekranından girin)\n' "${um% }"
}
# _comp_menu_action_card_tail <bilgi> <paket> <sunucu> <eylem> [sarma-genişliği] → kartın alt yarısı: son sonuç ve uyarılar (genişlik 0: sarma yapılmaz)
_comp_menu_action_card_tail() {
    local info="$1" id="$2" server="$3" name="$4" ww="${5:-44}" last ts rest now w n=0
    if last=$(comp_exec_last_result "$id" "$server" "$name" 2>/dev/null); then
        now=$(date +%s); ts=$(printf '%s\n' "$last" | head -n 1); rest=$(printf '%s\n' "$last" | tail -n +2 | tr '\n' ' ')
        if [[ "$ts" =~ ^[0-9]+$ ]] && [ "$ts" -le "$now" ]; then printf 'Son sonuç (%s): %s\n' "$(_comp_menu_age $((now - ts)))" "${rest% }"
        else printf 'Son sonuç: %s\n' "${rest% }"; fi
    fi
    while IFS= read -r w; do
        [ -n "$w" ] || continue
        n=$((n + 1)); [ "$n" -le 6 ] || break
        if [ "$ww" -gt 0 ]; then _comp_menu_wrap "$ww" "   " "⚠ $w"; else printf '⚠ %s\n' "$w"; fi
    done < <(comp_info_all "$info" "action.$name.warning")
}
# _comp_menu_action_card <bilgi> <paket> <sunucu> <eylem> <eksik> → seçili eylemin kartı (çok satırlı; fzf sağ bölmesi)
_comp_menu_action_card() {
    _comp_menu_action_card_head "$@"
    _comp_menu_action_card_tail "$1" "$2" "$3" "$4"
}

# _comp_menu_vars_card <component.json> <vars-dosyası> → "Değişkenler" satırının kartı: kaç değişken yazılı, YAZILI değerler (⚠ = paketin uyardığı değer)
# Yalnızca okur (tek awk geçişi). Dosyada yazılı olmayan değişkenler varsayılanda kalır ve listelenmez.
_comp_menu_vars_card() {
    local mf="$1" vf="$2" max=12
    [ -f "$vf" ] || { printf 'Henüz değer yazılmadı: tüm değişkenler varsayılanda.'; return 0; }
    comp_vars_describe "$mf" "$vf" 2>/dev/null | awk -F'\t' -v VF="$vf" -v MAX="$max" '
        BEGIN { while ((getline l < VF) > 0) { i = index(l, "="); if (i > 1) v[substr(l, 1, i - 1)] = substr(l, i + 1) } }
        { total++ }
        ($1 in v) { w++; val = v[$1]; if (length(val) > 44) val = substr(val, 1, 43) "…"
                    mark = ($7 != "" && v[$1] == $7) ? "⚠" : "•"
                    if (mark == "⚠") warn++
                    if (w <= MAX) line[w] = mark " " $1 " = " val }
        END { printf "Yazılı: %d / %d değişken%s\n", w, total, (warn > 0 ? " · ⚠ " warn " riskli değer" : "")
              for (i = 1; i <= w && i <= MAX; i++) print line[i]
              if (w > MAX) printf "… ve %d tane daha\n", w - MAX
              print "(Yazılmayanlar varsayılanda; ⚠ = paketin uyardığı değer)" }'
}

# _comp_menu_history_card <paket> <sunucu> → Geçmiş satırının kartı: son 5 çalıştırma ve son günlüğün son 8 satırı (fzf sağ bölmesi)
_comp_menu_history_card() {
    local id="$1" server="$2" log t v act res sdir n=0
    log="$COMP_MENU_DIR/components.log"
    printf 'Son çalıştırmalar\n'
    if [ -f "$log" ] && grep -qF "|$id|" "$log"; then
        while IFS='|' read -r t _ v act res; do
            printf '  %s %s  %s %s\n' "${t:5:5}" "${t:11:5}" "$act" "$([ "$res" = ok ] && echo '✅' || echo '❌')"
        done < <(grep -F "|$id|" "$log" | tail -n 5)
    else printf '  (kayıt yok)\n'; fi
    sdir="$(_comp_gate_state_dir)/$id/$server"
    if [ -f "$sdir/last-run.log" ]; then
        printf '\nSon günlüğün sonu\n'
        tail -n 8 "$sdir/last-run.log" | while IFS= read -r t; do n=$((n + 1)); printf '  %s\n' "${t:0:46}"; done
    fi
}

# _comp_menu_prep_text <paket> <sunucu> <bilgi> <eksik> → hazırlık özeti (başlık için; baştaki boşluklar atılır)
_comp_menu_prep_text() {
    local id="$1" server="$2" info="$3" missing="$4" m
    if [ -n "$missing" ]; then
        while IFS= read -r m; do [ -n "$m" ] && echo "❌ $m (zorunlu, yazılmadı)"; done <<< "$missing"
    else echo "✅ Zorunlu değişkenler tamam"; fi
    # önem sırası: puan, bulgular, sonra eylem durumları (başlık kısa terminalde kırpılırsa son satırlar gider)
    { _comp_menu_score_lines "$id" "$server"; _comp_menu_findings_lines "$id" "$server" "$info"; _comp_menu_status_lines "$id" "$server" "$info"; } | sed 's/^ *//'
}

# ---- Çalışma paneli (fzf): üst bölüm sabit çizilir, alt bölüm terminalin KAYDIRMA BÖLGESİ olur (genel parça: UI.sh ui_region_*) ----
# _comp_menu_progress_label <günlük> → çalışan alanın kısa ifadesi ("aide 9/14 · bekleniyor ~300 sn"); SSHPOCKET_PROGRESS satırı yoksa boş
_comp_menu_progress_label() {
    local a sira top dur bk last_a="" last_s="" last_t="" cur=""
    while IFS=$'\t' read -r a sira top dur _ _ _ _ bk; do
        last_a="$a"; last_s="$sira"; last_t="$top"
        [ "$dur" = basladi ] && cur="$a|$sira|$top|$bk"
    done < <(comp_progress_parse "$1")
    [ -n "$last_a" ] || return 0
    if [ -n "$cur" ]; then
        IFS='|' read -r a sira top bk <<< "$cur"
        printf '%s %s/%s' "$a" "$sira" "$top"
        [ "$bk" = "-" ] || printf ' · bekleniyor ~%s sn' "$bk"
    else printf '%s %s/%s tamam' "$last_a" "$last_s" "$last_t"; fi
}
# _comp_menu_watch_start <paket> <sunucu> <eylem> → arka planda saniyede bir günlüğe bakıp çıktı çerçevesinin başlığına ilerlemeyi yazar
# (ana akışa dokunmaz: parola istemleri gibi satırsız çıktılar gecikmez). Günlük bu çalıştırmadan önceyse (eski koşu) yok sayılır.
_comp_menu_watch_start() {
    local log marker parent=$$ name="$3" server="$2"
    log="$(_comp_gate_state_dir)/$1/$2/last-run.log"
    marker=$(mktemp "${TMPDIR:-/tmp}/sshpocket-watch.XXXXXX") || return 0
    (
        trap 'rm -f -- "$marker"' EXIT
        shown=""
        while kill -0 "$parent" 2>/dev/null; do
            if [ -f "$log" ] && [ ! "$marker" -nt "$log" ]; then
                l=$(_comp_menu_progress_label "$log")
                if [ "$l" != "$shown" ]; then
                    shown="$l"
                    ui_region_end "Çıktı: $name — $server (çalışıyor)${l:+ · $l}"
                fi
            fi
            sleep "${SSHPOCKET_WATCH_SECS:-1}"
        done
    ) 2>/dev/null &
    COMP_MENU_WATCH_PID=$!
}
_comp_menu_watch_stop() {
    [ -z "${COMP_MENU_WATCH_PID:-}" ] || { kill "$COMP_MENU_WATCH_PID" 2>/dev/null; wait "$COMP_MENU_WATCH_PID" 2>/dev/null || true; COMP_MENU_WATCH_PID=""; }
}
# _comp_menu_panel_begin <paket> <sunucu> <bilgi> <eylem> <eksik> → 0: panel kuruldu; 1: terminal küçük/bilinmiyor (eski akış)
_comp_menu_panel_begin() {
    local id="$1" server="$2" info="$3" name="$4" missing="$5" tail
    tail=$(_comp_menu_action_card_tail "$info" "$id" "$server" "$name" 0); [ -n "$tail" ] || tail="(henüz sonuç ya da uyarı yok)"
    ui_region_begin2 "Sunucu › $server › Paketler › $id › $name" "$(_comp_menu_action_card_head "$info" "$id" "$server" "$name" "$missing")" \
        "$tail" "Eylem" "Son sonuç ve uyarılar" "Çıktı: $name — $server (çalışıyor)" || return 1
    _comp_menu_watch_start "$id" "$server" "$name"
}
# _comp_menu_panel_end <eylem> <sunucu> <çıkış-kodu> <saniye> → çıktı çerçevesinin başlığını sonuçla günceller
_comp_menu_panel_end() {
    local name="$1" server="$2" rc="$3" secs="$4" label
    _comp_menu_watch_stop
    if [ "$rc" -eq 0 ]; then label="✅ $name — $server: başarılı · $secs sn"
    elif [ "$rc" -eq "${COMP_EXEC_REFUSED:-190}" ]; then label="⛔ $name — $server: çalıştırılmadı"
    else label="❌ $name — $server: başarısız (çıkış $rc) · $secs sn"; fi
    ui_region_end "$label"
}
_comp_menu_panel_reset() { _comp_menu_watch_stop; ui_region_reset; }

# _comp_menu_progress_lines <bilgi> <eylem> <günlük> → paketin SSHPOCKET_PROGRESS satırlarından alan tablosu (satır yoksa hiçbir şey yazmaz)
# Salt okunur eylemde (önizleme) "değişen" = uygulanırsa "değişecek"; kesin=0 ise sayı tahminidir (~). degisen/atlanan yoksa "?".
_comp_menu_progress_lines() {
    local info="$1" name="$2" log="$3" a top dur dg at ks uy bk icon word total=0 n=0 preview=0 cell
    [ -z "$(comp_action_get "$info" "$name" effects)" ] && preview=1
    word="değişen"; [ "$preview" -eq 0 ] || word="değişecek"
    while IFS=$'\t' read -r a _ top dur dg at ks uy bk; do
        [ "$n" -gt 0 ] || printf '  Alanlar (%s açık):\n' "$top"
        n=$((n + 1))
        if [ "$dur" = bitti ]; then
            icon="✅"; cell="$word ${dg/#-/?}"
            [ "$ks" != 0 ] || cell="$word ~${dg/#-/?}"
            [ "$at" = "-" ] || cell="$cell · atlanan $at"
            [ "$uy" = "-" ] || cell="$cell · ⚠ uyarı $uy"
        else
            icon="⏳"; cell="bitmedi"
            [ "$bk" = "-" ] || cell="bitmedi (tahmini bekleme $bk sn)"
        fi
        printf '    %s %-14s %s\n' "$icon" "$a" "$cell"
        [[ "$dg" =~ ^[0-9]+$ ]] && total=$((total + dg))
    done < <(comp_progress_parse "$log")
    if [ "$n" -gt 0 ] && [ "$preview" -eq 1 ]; then printf '    Toplam %s: %s\n' "$word" "$total"; fi
}

# _comp_menu_fail_reason <günlük> → başarısız Ansible koşusunun NEDENİ (en çok 3 satır, satır başına en çok 150 karakter); bulunamazsa hiçbir şey yazmaz.
# Son `fatal: [...]: FAILED! => {json}` satırından önce `stdout` (betiğin kendi iletisi, örn. "HATA: ...") sonra `msg` alınır; yalnız okur.
_comp_menu_fail_reason() {
    [ -f "$1" ] && command -v python3 >/dev/null 2>&1 || return 0
    python3 -I - "$1" <<'PY' 2>/dev/null || true
import json, sys
last = None
with open(sys.argv[1], encoding="utf-8", errors="replace") as f:
    for line in f:
        if line.startswith("fatal:") and "=> " in line:
            last = line.split("=> ", 1)[1]
if last:
    try:
        d = json.loads(last)
    except ValueError:
        sys.exit(0)
    text = (d.get("stdout") or "").strip() or (d.get("msg") or "").strip()
    out = []
    for l in text.splitlines():
        l = "".join(ch for ch in l if ch.isprintable()).strip()
        if l:
            out.append(l if len(l) <= 150 else l[:149] + "…")
    for l in out[:3]:
        print(l)
PY
}

# _comp_menu_run_panel <paket> <sunucu> <bilgi> <eylem> <çıkış-kodu> <saniye> → çalışma bitince tek bakışta sonuç paneli (ok/changed/failed, sonuç anahtarları, günlük, sıradaki adım)
_comp_menu_run_panel() {
    local id="$1" server="$2" info="$3" name="$4" rc="$5" secs="$6" log recap last rest nxt a r status reason
    log="$(_comp_gate_state_dir)/$id/$server/last-run.log"
    if [ "$rc" -eq 0 ]; then status="✅ Başarılı"
    elif [ "$rc" -eq "${COMP_EXEC_REFUSED:-190}" ]; then status="⛔ Çalıştırılmadı (kapı/onay/hazırlık; ayrıntı yukarıdaki iletide)"
    else status="❌ Başarısız (çıkış $rc)"; fi
    echo ""
    echo "$UI_RULE"
    printf '  Sonuç: %s — %s\n' "$name" "$server"
    echo "$UI_RULE"
    printf '  %s · %s sn\n' "$status" "$secs"
    if [ -f "$log" ]; then
        recap=$(grep -E '^[^ ]+ +: ok=' "$log" 2>/dev/null | tail -n 1 | sed -E 's/^[^:]+: *//; s/ +/ /g')
        [ -z "$recap" ] || printf '  Özet: %s\n' "$recap"
    fi
    if [ "$rc" -ne 0 ] && [ "$rc" -ne "${COMP_EXEC_REFUSED:-190}" ]; then
        # ❌ başarısızsa: betiğin kendi nedeni (örn. "HATA: …") özette de görünsün; ayrıntı günlükte
        reason=$(_comp_menu_fail_reason "$log")
        [ -z "$reason" ] || { printf '  Neden: %s\n' "$(printf '%s\n' "$reason" | head -n 1)"; printf '%s\n' "$reason" | tail -n +2 | sed 's/^/         /'; }
    fi
    _comp_menu_progress_lines "$info" "$name" "$log"
    if last=$(comp_exec_last_result "$id" "$server" "$name" 2>/dev/null); then
        rest=$(printf '%s\n' "$last" | tail -n +2 | tr '\n' ' ')
        [ -z "${rest// /}" ] || printf '  Anahtarlar: %s\n' "${rest% }"
    fi
    [ ! -f "$log" ] || printf '  Günlük: %s\n' "$log"
    # sıradaki adım: bu eylemi ön koşul sayan eylemler (örn. preview → apply); başarısızsa günlüğe bak
    if [ "$rc" -eq 0 ]; then
        nxt=""
        while IFS= read -r a; do
            [ -n "$a" ] || continue
            for r in $(comp_action_get "$info" "$a" requires | tr ',' ' '); do [ "$r" = "$name" ] && nxt="${nxt:+$nxt, }$a"; done
        done < <(comp_info_all "$info" action)
        [ -z "$nxt" ] || printf '  ▶ Sıradaki adım: %s (bu eylemden sonra açılır)\n' "$nxt"
    elif [ "$rc" -ne "${COMP_EXEC_REFUSED:-190}" ]; then
        printf '  ▶ Sıradaki adım: günlüğe bakın, sorunu giderin ve eylemi yeniden çalıştırın\n'
    fi
    echo "$UI_RULE"
}

# _comp_menu_yn <istem> → 0: e/E/evet; girdi yoksa (EOF) ve diğer her şey: 1 (varsayılan H)
_comp_menu_yn() {
    local ans
    printf '%s' "$1" >&2
    IFS= read -r ans || { echo >&2; return 1; }
    case "$ans" in e|E|evet|Evet|EVET) return 0 ;; *) return 1 ;; esac
}

# _comp_menu_unmet <bilgi> <paket> <sunucu> <eylem> → eylemin taze olmayan (yok/eski/başarısız) ön koşullarını satır satır yazar
_comp_menu_unmet() {
    local info="$1" id="$2" server="$3" name="$4" r age
    for r in $(comp_action_get "$info" "$name" requires | tr ',' ' '); do
        if age=$(comp_gate_requires_age "$id" "$server" "$COMP_MENU_VER" "$r") && [ "$age" -le "$COMP_REQUIRES_TTL" ]; then :; else printf '%s\n' "$r"; fi
    done
}

# _comp_menu_run_one <paket> <sunucu> <bilgi> <eylem> <eksik> <fzf-mi> → tek eylemi panelle çalıştırır; çıkış kodu COMP_MENU_RC'de (işlev hep 0 döner)
_comp_menu_run_one() {
    local id="$1" server="$2" info="$3" name="$4" missing="$5" fz="$6" t0=$SECONDS
    COMP_MENU_RC=0
    if [ "$fz" = 1 ] && _comp_menu_panel_begin "$id" "$server" "$info" "$name" "$missing"; then :; else echo ""; fi
    comp_cli_run "$id" "$name" "$server" && COMP_MENU_RC=0 || COMP_MENU_RC=$?
    if _comp_menu_interactive; then _comp_menu_run_panel "$id" "$server" "$info" "$name" "$COMP_MENU_RC" "$((SECONDS - t0))"; fi
    _comp_menu_panel_end "$name" "$server" "$COMP_MENU_RC" "$((SECONDS - t0))"
    return 0
}

# _comp_menu_run_chain <paket> <sunucu> <bilgi> <eylem> <eksik> <fzf-mi> <eylemler...> → eylemi çalıştırır; taze olmayan ön koşulu (örn. apply için preview)
# kullanıcı onaylarsa önce çalıştırır, sonucu gösterir ve devam için ikinci kez sorar. Kapı (comp_gate_requires) değişmez: çalıştırma kolaylığıdır, atlama değil.
_comp_menu_run_chain() {
    local id="$1" server="$2" info="$3" hedef="$4" missing="$5" fz="$6" r; shift 6
    local pre
    pre=$(_comp_menu_unmet "$info" "$id" "$server" "$hedef" | grep -Fxf <(printf '%s\n' "$@") || true)
    for r in $pre; do
        if ! _comp_menu_yn "↪ '$hedef' için önce '$r' gerekli (yok ya da süresi dolmuş). '$r' şimdi çalıştırılsın mı? [e/H]: "; then
            echo "Vazgeçildi: '$hedef' çalıştırılmadı."; return 0
        fi
        _comp_menu_run_one "$id" "$server" "$info" "$r" "$missing" "$fz"
        if [ "$COMP_MENU_RC" -ne 0 ]; then echo ""; echo "⛔ '$r' tamamlanamadı; '$hedef' çalıştırılmadı."; return 0; fi
        if ! _comp_menu_yn "'$r' sonucunu gördünüz. '$hedef' ile devam edilsin mi? [e/H]: "; then echo "Vazgeçildi: '$hedef' çalıştırılmadı."; return 0; fi
        _comp_menu_panel_reset
    done
    _comp_menu_run_one "$id" "$server" "$info" "$hedef" "$missing" "$fz"
}

# _comp_menu_kontrol <paket> <sunucu> <bilgi> <eksik> <fzf-mi> → preview, sonra audit; sonunda tek özet (değişecek sayısı, Lynis, bulgu)
_comp_menu_kontrol() {
    local id="$1" server="$2" info="$3" missing="$4" fz="$5" total="" prc audit_note=""
    _comp_menu_run_one "$id" "$server" "$info" preview "$missing" "$fz"; prc=$COMP_MENU_RC
    if [ "$prc" -eq 0 ]; then
        total=$(_comp_menu_progress_lines "$info" preview "$(_comp_gate_state_dir)/$id/$server/last-run.log" | sed -n 's/.*Toplam değişecek: *//p' | tail -n 1)
        _comp_menu_panel_reset
        _comp_menu_run_one "$id" "$server" "$info" audit "$missing" "$fz"
        if [ "$COMP_MENU_RC" -eq "${COMP_EXEC_REFUSED:-190}" ]; then audit_note="audit çalıştırılmadı (kapı/onay; ayrıntı yukarıda)"
        elif [ "$COMP_MENU_RC" -ne 0 ]; then audit_note="audit başarısız (çıkış $COMP_MENU_RC)"; fi
    else
        audit_note="preview tamamlanamadı; audit çalıştırılmadı"
    fi
    echo ""
    echo "${UI_RULE:-────────}"
    printf '  Kontrol özeti — %s\n' "$server"
    printf '  preview: %s\n' "$([ "$prc" -eq 0 ] && printf '✅ apply için hazır · uygulanırsa değişecek: %s' "${total:-?}" || printf '❌ başarısız')"
    if [ -n "$audit_note" ]; then printf '  audit:   ⚠ %s\n' "$audit_note"; else printf '  audit:   ✅ tamam\n'; fi
    _comp_menu_score_lines "$id" "$server"
    _comp_menu_findings_lines "$id" "$server" "$info"
    echo "${UI_RULE:-────────}"
}

# comp_menu_actions <paket> <sunucu> → hazırlık durumu + (📝 Değişkenler | eylemler); seçileni çalıştırır; geri dönene kadar döngü
_comp_menu_actions_panel() {
    local id="$1" server="$2" info name pick i act eff req missing m plat hidden fz=0 rc hedef ko=0
    local -a names labels
    info=$(comp_prepare "$id") || { echo "Paket doğrulanamadı: $id (./start.sh --components list)" >&2; return 1; }
    _comp_menu_ctx "$id" "$server" "$info"
    # Eski sunucu / ilk açılış: platform kayıtlı değilse (ve paket platform listesi bildiriyorsa) bir kez sorulur; "Şimdi değil" hiçbir şey yazmaz,
    # soru bir sonraki girişte yine çıkar. allow_from boşsa ayrıca her eylemde ⛔ (requires_vars kapısı) sürer.
    if declare -F comp_onboard_needed >/dev/null && comp_onboard_needed "$id" "$COMP_MENU_VARS"; then
        echo "ℹ️  Bu sunucu için $id'ın sunucu türü henüz seçilmemiş."
        comp_onboard_package "$id" "$COMP_MENU_DIR" || true
        IFS= read -r -p "Devam için Enter..." _ || true
    fi
    names=()
    hidden=0
    plat=$(comp_vars_platform "$COMP_MENU_MF" "$COMP_MENU_VARS" 2>/dev/null) || plat=""
    while IFS= read -r name; do
        [ -n "$name" ] || continue
        if [ -n "${COMP_MENU_ITEM_ACTIONS:-}" ]; then case ",$COMP_MENU_ITEM_ACTIONS," in *",$name,"*) ;; *) continue ;; esac; fi
        if comp_action_on_platform "$info" "$name" "$plat"; then names+=("$name"); else hidden=$((hidden + 1)); fi
    done < <(comp_info_all "$info" action)
    [ "${#names[@]}" -gt 0 ] || { echo "Pakette eylem yok." >&2; return 1; }
    # 🧪 Kontrol (preview + audit art arda): yalnızca ikisi de bu platformda varsa
    if printf '%s\n' "${names[@]}" | grep -qx preview && printf '%s\n' "${names[@]}" | grep -qx audit; then ko=1; fi
    while true; do
        missing=$(comp_vars_missing "$COMP_MENU_MF" "$COMP_MENU_VARS")
        clear
        fz=0; declare -F ui_fzf_enabled >/dev/null && ui_fzf_enabled && fz=1
        if [ "$fz" = 0 ]; then
        if declare -F ui_header >/dev/null; then ui_header "Sunucu › $server › Paketler › $id $COMP_MENU_VER"; fi
        echo ""
        echo "Hazırlık"
        if [ -n "$missing" ]; then
            while IFS= read -r m; do [ -n "$m" ] && echo "  ❌ $m  (zorunlu, henüz yazılmadı)"; done <<< "$missing"
            echo "  Değiştiren eylemler ve ön koşul eylemleri (preview), zorunlu değişkenler tamamlanana kadar kapalıdır: önce 📝 Değişkenler. Salt okunur durum eylemleri açıktır."
        else
            echo "  ✅ Zorunlu değişkenler tamam"
        fi
        _comp_menu_status_lines "$id" "$server" "$info"
        _comp_menu_score_lines "$id" "$server"
        _comp_menu_findings_lines "$id" "$server" "$info"
        [ "$hidden" -eq 0 ] || echo "  ℹ️  $hidden eylem bu sunucunun platformuna uygun olmadığı için gizlendi."
        echo ""
        fi
        labels=("📝 Değişkenler${missing:+  ← önce burası}")
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=("Paket değişkenlerini görüntüler ve düzenler (${COMP_MENU_VARS##*/}); değerler kaydedilmeden önce doğrulanır.")
        for name in "${names[@]}"; do
            req=$(comp_action_get "$info" "$name" requires)
            labels+=("$name — $(_comp_menu_badge "$info" "$name")${req:+  (önce: $(_comp_menu_req_label "$id" "$server" "$COMP_MENU_VER" "$req"))}$(if _comp_menu_action_blocked "$info" "$name" "$missing"; then printf '  ⛔'; fi)")
            act=$(comp_info_all "$info" "action.$name.warning" | head -n 1)
            eff=$(comp_action_get "$info" "$name" confirm)
            UI_PICK_DESCS+=("${act:-Ek uyarı yok.} [onay: ${eff:-none}]")
        done
        if [ "$ko" = 1 ]; then
            labels+=("🧪 Kontrol (preview + audit)")
            UI_PICK_DESCS+=("preview'ı sonra audit'i art arda çalıştırır; sonunda tek özet: apply'da değişecek sayısı, Lynis puanı, bulgular. Salt okunur (audit hedefe yalnız lynis kurabilir).")
        fi
        labels+=("📜 Geçmiş ve son sonuçlar")
        UI_PICK_DESCS+=("Bu sunucudaki son çalıştırmalar, ön koşulların tazeliği (örn. apply için preview) ve son çalıştırma günlüğü.")
        if _comp_menu_findings_load "$id" "$server" "$info"; then
            labels+=("🔎 Bulgular ($(cut -d$'\037' -f3 <<< "$COMP_FINDINGS_META"))")
            UI_PICK_DESCS+=("Son denetimin bulguları: önem sırası, ayrıntı ve paketteki karşılığı.")
        fi
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local UI_PICK_BACK_DESC="Sunucu menüsüne döner."
        if [ "$fz" = 1 ]; then
            # fzf paneli: yol çubuğu, hazırlık özeti başlıkta, seçili eylemin kartı sağ bölmede
            local -a UI_PICK_DESCS=()
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            local UI_PICK_WIDE=1 UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_START=1 UI_PICK_TITLE="Sunucu › $server › Paketler › $id $COMP_MENU_VER" UI_PICK_HEADER="" UI_PICK_HEIGHT=""
            UI_PICK_HEADER=$(_comp_menu_prep_text "$id" "$server" "$info" "$missing" | while IFS= read -r m; do _comp_menu_wrap "$(( ${SSHPOCKET_COLS:-$(tput cols 2>/dev/null || echo 80)} - 6 ))" "   " "$m"; done)
            [ "$hidden" -eq 0 ] || UI_PICK_HEADER="$UI_PICK_HEADER"$'\n'"ℹ️  $hidden eylem bu platforma uygun olmadığı için gizlendi"
            ui_fit_wide "$(( ${#labels[@]} + 1 ))" 10   # liste en az 10 satır görünsün; başlık kısa terminalde kısaltılır
            UI_PICK_DESCS+=("Paket değişkenlerini görüntüler ve düzenler (${COMP_MENU_VARS##*/}); değerler kaydedilmeden önce doğrulanır."$'\n'"$(printf 'Zorunlu eksik: %s\n' "$(printf '%s' "${missing:-yok}" | tr '\n' ' ')" | ui_card_colorize)"$'\n'"$(_comp_menu_vars_card "$COMP_MENU_MF" "$COMP_MENU_VARS" | ui_card_colorize)")
            for name in "${names[@]}"; do UI_PICK_DESCS+=("$(_comp_menu_action_card "$info" "$id" "$server" "$name" "$missing" | ui_card_colorize)"); done
            [ "$ko" = 0 ] || UI_PICK_DESCS+=("preview'ı sonra audit'i art arda çalıştırır; sonunda tek özet (değişecek sayısı, Lynis puanı, bulgular)."$'\n'"Her ikisinin kendi onayı/kapısı aynen geçerlidir.")
            UI_PICK_DESCS+=("$(_comp_menu_history_card "$id" "$server" | ui_card_colorize)")
            if _comp_menu_findings_load "$id" "$server" "$info"; then UI_PICK_DESCS+=("$(_comp_menu_findings_lines "$id" "$server" "$info" | sed 's/^ *//' | ui_card_colorize)"$'\n'"Enter: listeyi aç"); fi
        fi
        pick=$(_comp_menu_pick "Geri" "${labels[@]}")
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        if [ "$pick" -eq 1 ]; then
            comp_menu_vars "$id" "$server" "$info"
            continue
        fi
        if [ "$ko" = 1 ] && [ "$pick" -eq $((${#names[@]} + 2)) ]; then
            if _comp_menu_action_blocked "$info" preview "$missing" || _comp_menu_action_blocked "$info" audit "$missing"; then
                echo ""; echo "⛔ Önce zorunlu değişkenleri tamamlayın (📝 Değişkenler)."
                read -r -p "Devam için Enter..." _ || return 0
                continue
            fi
            _comp_menu_kontrol "$id" "$server" "$info" "$missing" "$fz"
            echo ""
            read -r -p "Devam için Enter..." _ || { _comp_menu_panel_reset; return 0; }
            _comp_menu_panel_reset
            continue
        fi
        if [ "$pick" -eq $((${#names[@]} + 2 + ko)) ]; then
            comp_menu_history "$id" "$server" "$info" || return 0
            continue
        fi
        if [ "$pick" -eq $((${#names[@]} + 3 + ko)) ]; then
            comp_menu_findings "$id" "$server" "$info"
            continue
        fi
        if _comp_menu_action_blocked "$info" "${names[$((pick - 2))]}" "$missing"; then
            echo ""
            echo "⛔ Önce zorunlu değişkenleri tamamlayın (📝 Değişkenler)."
            read -r -p "Devam için Enter..." _ || return 0
            continue
        fi
        hedef="${names[$((pick - 2))]}"
        _comp_menu_run_chain "$id" "$server" "$info" "$hedef" "$missing" "$fz" "${names[@]}"
        echo ""
        read -r -p "Devam için Enter..." _ || { _comp_menu_panel_reset; return 0; }
        _comp_menu_panel_reset
    done
}

# comp_menu_history <paket> <sunucu> <bilgi> → son çalıştırmalar, ön koşul tazeliği, son sonuçlar, günlük
comp_menu_history() {
    local id="$1" server="$2" info="$3" log name req r age sdir now a ans t v act res
    log="$COMP_MENU_DIR/components.log"
    now=$(date +%s)
    clear
    if declare -F ui_header >/dev/null; then ui_header "Sunucu › $server › Paketler › $id › Geçmiş"; fi
    echo ""
    echo "Son çalıştırmalar (${log##*/}, en yeni altta)"
    if [ -f "$log" ] && grep -qF "|$id|" "$log"; then
        grep -F "|$id|" "$log" | tail -n 10 | while IFS='|' read -r t _ v act res; do
            printf '  %s  %-16s %s  (%s)\n' "$t" "$act" "$([ "$res" = ok ] && echo '✅ ok' || echo '❌ başarısız')" "$v"
        done
    else
        echo "  (kayıt yok)"
    fi
    echo ""
    echo "Ön koşullar (tazelik süresi $((COMP_REQUIRES_TTL / 60)) dk)"
    a=0
    while IFS= read -r name; do
        [ -n "$name" ] || continue
        comp_action_on_platform "$info" "$name" "$(comp_vars_platform "$COMP_MENU_MF" "$COMP_MENU_VARS" 2>/dev/null)" || continue
        req=$(comp_action_get "$info" "$name" requires)
        [ -n "$req" ] || continue
        a=1
        for r in ${req//,/ }; do
            if age=$(comp_gate_requires_age "$id" "$server" "$COMP_MENU_VER" "$r"); then
                if [ "$age" -le "$COMP_REQUIRES_TTL" ]; then
                    echo "  $name için $r: ✅ taze ($(_comp_menu_age "$age"); $(( (COMP_REQUIRES_TTL - age) / 60 )) dk daha geçerli)"
                else
                    echo "  $name için $r: ⏳ süresi doldu ($(_comp_menu_age "$age")); önce $r'i yeniden çalıştırın"
                fi
            else
                echo "  $name için $r: ❌ başarılı kayıt yok; önce $r çalıştırın"
            fi
        done
    done < <(comp_info_all "$info" action)
    [ "$a" -eq 1 ] || echo "  (ön koşullu eylem yok)"
    echo ""
    echo "Son bilinen durum"
    if [ -n "$(_comp_menu_status_lines "$id" "$server" "$info")" ]; then _comp_menu_status_lines "$id" "$server" "$info"; else echo "  (durum eylemi yok)"; fi
    echo ""
    sdir="$(_comp_gate_state_dir)/$id/$server"
    if [ -f "$sdir/last-run.log" ]; then
        echo "Son çalıştırma günlüğü: $sdir/last-run.log"
        IFS= read -r -p "Son 30 satırı gösterilsin mi? [e/H]: " ans || return 1
        case "$ans" in e|E|evet|Evet) echo ""; tail -n 30 "$sdir/last-run.log" | LC_ALL=C tr -d '\000-\010\013-\037\177' ;; esac
    fi
    _comp_menu_log_browser "$sdir" || return 1
    echo ""
    read -r -p "Devam için Enter..." _ || return 1
}

# _comp_menu_log_pager <dosya> → günlüğü tamamen gösterir (etkileşimli terminalde less, değilse düz çıktı); denetim karakterleri atılır
_comp_menu_log_pager() {
    if [ -t 1 ] && command -v less >/dev/null 2>&1; then
        LC_ALL=C tr -d '\000-\010\013-\037\177' < "$1" | less -S
    else
        LC_ALL=C tr -d '\000-\010\013-\037\177' < "$1"
    fi
}

# _comp_menu_log_browser <durum-klasörü> → kalıcı eylem günlüklerini (en yeni üstte) listeler; seçilen tam açılır
_comp_menu_log_browser() {
    local sdir="$1" ldir f n i ans res base
    local -a files
    ldir="$sdir/logs"
    [ -d "$ldir" ] || return 0
    files=()
    while IFS= read -r f; do files+=("$f"); done < <(find "$ldir" -maxdepth 1 -type f -name '*.log' | sort -r | head -n 15)
    [ "${#files[@]}" -gt 0 ] || return 0
    echo ""
    echo "Eylem günlükleri ($ldir; son ${#files[@]}, en yeni üstte)"
    i=0
    for f in "${files[@]}"; do
        i=$((i + 1)); base="${f##*/}"
        res=$(sed -n 's/^# sonuç: //p' "$f" | tail -n 1)
        printf '  %2d) %s %s\n' "$i" "$([ "$res" = ok ] && echo '✅' || echo '❌')" "${base%.log}"
    done
    while true; do
        IFS= read -r -p "Tam açmak için numara (Enter: geç): " ans || return 1
        [ -n "$ans" ] || return 0
        if [[ "$ans" =~ ^[0-9]+$ ]] && [ "$ans" -ge 1 ] && [ "$ans" -le "${#files[@]}" ]; then
            n="${files[$((ans - 1))]}"
            _comp_menu_log_pager "$n"
        else
            echo "Geçersiz numara."
        fi
    done
}

# Büyük paketlerde (çok değişken) değişkenler ekranının başına arama/süzgeç satırları eklenir; küçük paketlerde numaralandırma eskisi gibidir.
COMP_MENU_VARS_FILTER_MIN="${COMP_MENU_VARS_FILTER_MIN:-25}"

# _comp_menu_clip <metin> [en-çok] → uzun değeri kısaltır (…)
_comp_menu_clip() {
    local m="${2:-40}"
    if [ "${#1}" -gt "$m" ]; then printf '%s…' "${1:0:$((m - 1))}"; else printf '%s' "$1"; fi
}

# _comp_menu_group_name <süzgeç> → grup süzgecinin ekrandaki adı ("" = tümü, "-" = grupsuz)
_comp_menu_group_name() {
    if [ -z "$1" ]; then printf 'tümü'; elif [ "$1" = "-" ]; then printf '%s' '(grupsuz)'; else printf '%s' "$1"; fi
}

# _comp_menu_pick_group → grup seçtirir; f_group'u (çağıranın dinamik kapsamı) günceller. Tümü="" · grupsuz="-"
_comp_menu_pick_group() {
    local g pick
    local -a gl gpick
    gl=(); while IFS= read -r g; do [ -n "$g" ] && gl+=("$g"); done <<< "$glist"
    gpick=("Tümü" "${gl[@]}" "(grupsuz)")
    # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
    local -a UI_PICK_DESCS=("Süzgeci kaldırır: tüm değişkenler listelenir.")
    for g in "${gl[@]}"; do UI_PICK_DESCS+=("Yalnızca '$g' grubundaki değişkenleri listeler."); done
    UI_PICK_DESCS+=("Hiçbir gruba bağlı olmayan değişkenleri listeler.")
    # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
    local UI_PICK_WIDE=1 UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_HEADER="" UI_PICK_HEIGHT="" UI_PICK_BACK_DESC="Grubu değiştirmeden listeye döner."
    UI_PICK_HEADER="Şimdiki grup: $(_comp_menu_group_name "$f_group")"
    # shellcheck disable=SC2034
    local UI_PICK_TITLE="Sunucu › $server › Paketler › $id › Değişkenler › Grup"
    ui_fit_wide "$(( ${#gpick[@]} + 1 ))" 8
    pick=$(_comp_menu_pick "Geri" "${gpick[@]}")
    if [ "$pick" -ge 1 ] 2>/dev/null; then
        if [ "$pick" -eq 1 ]; then f_group=""
        elif [ "$pick" -eq $((${#gpick[@]})) ]; then f_group="-"
        else f_group="${gl[$((pick - 2))]}"
        fi
    fi
}

# _comp_menu_var_match <ad> <açıklama> <grup> <değişmiş:0|1> <metin> <grup-süzgeci> <yalnız-değişenler:0|1> → 0 ise satır gösterilir
# grup-süzgeci: boş = hepsi, "-" = grupsuzlar, aksi halde grup adı. Metin aramasında büyük/küçük harf ayrımı yoktur (ad ve açıklama).
_comp_menu_var_match() {
    local hay needle
    if [ "$7" = 1 ] && [ "$4" != 1 ]; then return 1; fi
    case "$6" in
        "") ;;
        "-") [ -z "$3" ] || return 1 ;;
        *) [ "$3" = "$6" ] || return 1 ;;
    esac
    if [ -n "$5" ]; then
        hay=$(printf '%s %s' "$1" "$2" | tr '[:upper:]' '[:lower:]')
        needle=$(printf '%s' "$5" | tr '[:upper:]' '[:lower:]')
        case "$hay" in *"$needle"*) ;; *) return 1 ;; esac
    fi
    return 0
}

# comp_menu_vars <paket> <sunucu> <bilgi> → değişken listesi; seçileni düzenletir
comp_menu_vars() {
    local id="$1" server="$2" info="$3" line n t r d lo hi desc cur forced pick grp gp total chg g nc kind mark
    local f_text="" f_group="" f_changed=0 nchg show_ctl glist
    local -a lines labels all_lines ctl_kinds ctl_labels ctl_descs var_labels var_descs
    forced=$(_comp_menu_forced "$info")
    while true; do
        all_lines=(); lines=(); var_labels=(); var_descs=(); ctl_kinds=(); ctl_labels=(); ctl_descs=(); nchg=0; glist=""
        while IFS= read -r line; do
            [ -n "$line" ] || continue
            # katalog formu (S4): COMP_MENU_VARS_ONLY doluysa yalnız seçili yazılımın params alanları gösterilir
            if [ -n "${COMP_MENU_VARS_ONLY:-}" ]; then case ",$COMP_MENU_VARS_ONLY," in *",$(_comp_menu_field "$line" 1),"*) ;; *) continue ;; esac; fi
            all_lines+=("$line")
        done < <(comp_vars_describe "$COMP_MENU_MF" "$COMP_MENU_VARS")
        [ "${#all_lines[@]}" -gt 0 ] || { echo "Pakette değişken yok."; read -r -p "Devam için Enter..." _ || true; return 0; }
        # değişen sayısı ve grup listesi (süzgeçten bağımsız)
        for line in "${all_lines[@]}"; do
            n=$(_comp_menu_field "$line" 1); grp=$(_comp_menu_field "$line" 9)
            if ! printf '%s\n' "$forced" | grep -qxF -- "$n"; then
                cur=$(comp_vars_get "$COMP_MENU_VARS" "$n" 2>/dev/null) || cur=""
                [ -z "$cur" ] || nchg=$((nchg + 1))
            fi
            if [ -n "$grp" ] && ! printf '%s\n' "$glist" | grep -qxF -- "$grp"; then glist="${glist:+$glist$'\n'}$grp"; fi
        done
        for line in "${all_lines[@]}"; do
            n=$(_comp_menu_field "$line" 1); t=$(_comp_menu_field "$line" 2); r=$(_comp_menu_field "$line" 3)
            d=$(_comp_menu_unq "$(_comp_menu_field "$line" 4)"); lo=$(_comp_menu_field "$line" 5); hi=$(_comp_menu_field "$line" 6)
            desc=$(_comp_menu_field "$line" 8)
            grp=$(_comp_menu_field "$line" 9); gp=""; [ -n "$grp" ] && gp="[$grp] "
            cur=""; chg=0
            if ! printf '%s\n' "$forced" | grep -qxF -- "$n"; then
                cur=$(comp_vars_get "$COMP_MENU_VARS" "$n" 2>/dev/null) || cur=""
                [ -z "$cur" ] || chg=1
            fi
            _comp_menu_var_match "$n" "$desc" "$grp" "$chg" "$f_text" "$f_group" "$f_changed" || continue
            lines+=("$line")
            mark=""; [ "$chg" = 1 ] && mark="✏️  "
            if printf '%s\n' "$forced" | grep -qxF -- "$n"; then
                var_labels+=("$gp$n — otomatik (çalıştırıcı kullanıcı klasörüne yönlendirir; değiştirilemez)")
            elif [ -n "$cur" ]; then
                var_labels+=("$mark$gp$n = $(_comp_menu_clip "$cur")")
            elif [ -n "$d" ]; then
                var_labels+=("$gp$n = (varsayılan: $(_comp_menu_clip "$d"))")
            else
                var_labels+=("$gp$n = (boş)${r:+$([ "$r" = yes ] && echo '  ❌ zorunlu')}")
            fi
            var_descs+=("$t${lo:+ · en az $lo}${hi:+ · en çok $hi} — ${desc:-açıklama yok}")
        done
        # arama/süzgeç satırları: büyük paketlerde ya da süzgeç açıkken
        show_ctl=0
        if [ "${#all_lines[@]}" -gt "$COMP_MENU_VARS_FILTER_MIN" ] || [ -n "$f_text$f_group" ] || [ "$f_changed" = 1 ]; then show_ctl=1; fi
        if [ "$show_ctl" = 1 ]; then
            ctl_kinds+=(search); ctl_labels+=("🔎 Ara: ${f_text:-(yok)}"); ctl_descs+=("Değişken adı ya da açıklamasında arar (büyük/küçük harf fark etmez). Boş bırakırsanız aramayı temizler.")
            if [ -n "$glist" ]; then
                ctl_kinds+=(group); ctl_labels+=("📂 Grup: $(_comp_menu_group_name "$f_group")"); ctl_descs+=("Değişkenleri paketin grup etiketine göre süzer.")
            fi
            ctl_kinds+=(changed); ctl_labels+=("✏️  Yalnızca değiştirdiklerim: $([ "$f_changed" = 1 ] && echo açık || echo kapalı) ($nchg)"); ctl_descs+=("Bu sunucu için dosyada değeri yazılı olanları gösterir (✏️ işaretli satırlar).")
            if [ -n "$f_text$f_group" ] || [ "$f_changed" = 1 ]; then
                ctl_kinds+=(clear); ctl_labels+=("↩ Süzgeci temizle"); ctl_descs+=("Arama, grup ve 'değiştirdiklerim' süzgecini kaldırır.")
            fi
        fi
        nc=${#ctl_kinds[@]}
        labels=(${ctl_labels[@]+"${ctl_labels[@]}"} ${var_labels[@]+"${var_labels[@]}"})
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=(${ctl_descs[@]+"${ctl_descs[@]}"} ${var_descs[@]+"${var_descs[@]}"})
        clear
        if declare -F ui_screen >/dev/null; then ui_screen "Sunucu › $server › Paketler › $id › ${COMP_MENU_VARS_TITLE:-Değişkenler}"; fi
        local fzf_card=0 hid_note=""
        if declare -F ui_card_on >/dev/null && ui_card_on; then fzf_card=1; fi
        [ "$fzf_card" = 1 ] || echo ""
        total=$(comp_vars_describe "$COMP_MENU_MF" | grep -c .)
        if [ -z "${COMP_MENU_VARS_ONLY:-}" ] && [ "$total" -gt "${#all_lines[@]}" ] 2>/dev/null; then
            hid_note="ℹ️  $((total - ${#all_lines[@]})) değişken bu sunucunun platformuna uygun olmadığı için gizlendi (yazılı değerleri yine de çalıştırmaya iletilir)."
            [ "$fzf_card" = 1 ] || echo "$hid_note"   # fzf: aşağıda başlık kutusunda
        fi
        if [ "$show_ctl" = 1 ] && [ "$fzf_card" = 0 ]; then
            echo "Gösterilen: ${#lines[@]} / ${#all_lines[@]} değişken · bu sunucuda değiştirilen: $nchg"
            [ "${#lines[@]}" -gt 0 ] || echo "Süzgece uyan değişken yok."
        fi
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local UI_PICK_BACK_DESC="Eylem listesine döner."
        local flash="${COMP_MENU_FLASH:-}"; COMP_MENU_FLASH=""
        [ -z "$flash" ] || [ "$fzf_card" = 1 ] || echo "$flash"   # fzf: ileti başlık kutusunda
        # fzf geniş düzeni: yol çubuğu kenarlık etiketi, sağda açıklama, üstte konum özeti, Ctrl-O ile grup seçimi
        # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
        local UI_PICK_WIDE=1 UI_PICK_TITLE="Sunucu › $server › $id › ${COMP_MENU_VARS_TITLE:-Değişkenler}" UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_HEADER=""
        # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
        UI_PICK_HEADER="${flash:+$flash$'\n'}${hid_note:+$hid_note$'\n'}Grup: $(_comp_menu_group_name "$f_group") · ${#lines[@]}/${#all_lines[@]} değişken · değiştirilen: $nchg${show_ctl:+$([ "${#lines[@]}" -gt 0 ] || echo ' · süzgece uyan değişken yok')}"
        # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
        if [ -n "$glist" ]; then UI_PICK_EXPECT="ctrl-o"; UI_PICK_EXPECT_HINT="Ctrl-O: grup seç"; fi
        ui_fit_wide "$(( ${#labels[@]} + 1 ))" 10   # uzun liste: kullanılabilir yüksekliğin tamamı; kısa liste kompakt
        pick=$(_comp_menu_pick "Geri" "${labels[@]}")
        if [ "$pick" = "key:ctrl-o" ]; then _comp_menu_pick_group; continue; fi
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        if [ "$pick" -le "$nc" ]; then
            kind="${ctl_kinds[$((pick - 1))]}"
            case "$kind" in
                search)
                    IFS= read -r -p "Ara (Enter = temizle): " f_text || return 0
                    f_text="${f_text//[[:cntrl:]]/}" ;;
                changed) if [ "$f_changed" = 1 ]; then f_changed=0; else f_changed=1; fi ;;
                clear) f_text=""; f_group=""; f_changed=0 ;;
                group) _comp_menu_pick_group ;;
            esac
            continue
        fi
        line="${lines[$((pick - nc - 1))]}"
        n=$(_comp_menu_field "$line" 1); t=$(_comp_menu_field "$line" 2)
        echo ""
        if printf '%s\n' "$forced" | grep -qxF -- "$n"; then
            echo "ℹ️  $n çalıştırıcı tarafından belirlenir (hassas çıktı USB'ye yazılmasın diye kullanıcı klasörüne yönlendirilir)."
            read -r -p "Devam için Enter..." _ || return 0
        elif [[ "$t" == list\[* ]] && _comp_menu_interactive; then
            comp_menu_edit_list "$line" "$server" || return 0
        elif [[ "$t" == list\[cidr\] ]]; then
            comp_menu_edit_cidr "$n" "$server" || return 0
        else
            comp_menu_edit_value "$line" || return 0
        fi
    done
}

# ---- Liste editörü: genel motor UI.sh ui_edit_list; burada yalnızca pakete/CIDR'e özgü geri çağrımlar ----
# Durum (dinamik/genel): COMP_MENU_LIST_NAME, COMP_MENU_LIST_LA (CIDR önerisi için yerel adres), _SC/_SD (öneriler), _IP/_PORT/_USER/_KEY
_comp_menu_list_validate() { comp_vars_check "$COMP_MENU_MF" "$COMP_MENU_LIST_NAME" "$1" 2>&1; }
# list[cidr] ekleme: daha geniş ağ daha dar girdilerin yerine geçer; bilgi stdout'a (_comp_menu_cidr_add `list` dizisini okur/yazar)
_comp_menu_list_cidr_add() {
    local -a list=()
    list=(${UI_LIST_ITEMS[@]+"${UI_LIST_ITEMS[@]}"})
    _comp_menu_cidr_add "$1" || true
    UI_LIST_ITEMS=(${list[@]+"${list[@]}"})
}
_comp_menu_list_cidr_extras() {
    local c x i=0
    COMP_MENU_LIST_SC=(); COMP_MENU_LIST_SD=()
    if [ -n "$COMP_MENU_LIST_LA" ]; then
        while IFS=$'\t' read -r c x; do [ -n "$c" ] && { COMP_MENU_LIST_SC+=("$c"); COMP_MENU_LIST_SD+=("$x"); }; done < <(comp_net_suggest "$COMP_MENU_LIST_LA")
    fi
    for c in ${COMP_MENU_LIST_SC[@]+"${COMP_MENU_LIST_SC[@]}"}; do
        UI_LIST_X_LABELS+=("➕ öneri: $c"); UI_LIST_X_ACTS+=("sug:$i"); UI_LIST_X_DESCS+=("${COMP_MENU_LIST_SD[$i]}"); i=$((i + 1))
    done
    UI_LIST_X_LABELS+=("🔎 Sunucunun beni gördüğü adresi sorgula"); UI_LIST_X_ACTS+=(seen)
    UI_LIST_X_DESCS+=("Salt okunur ssh ile sunucunun sizi hangi adresten gördüğünü sorar ve öneriyi buna göre günceller.")
}
_comp_menu_list_cidr_run() {
    local seen
    case "$1" in
        sug:*) _comp_menu_list_cidr_add "${COMP_MENU_LIST_SC[${1#sug:}]}" > "${TMPDIR:-/tmp}/.sshpocket-cidr.$$"; UI_LIST_MSG=$(cat "${TMPDIR:-/tmp}/.sshpocket-cidr.$$"); rm -f -- "${TMPDIR:-/tmp}/.sshpocket-cidr.$$"; UI_LIST_DIRTY=1 ;;
        seen)
            if seen=$(comp_net_seen_by_server "$COMP_MENU_LIST_IP" "$COMP_MENU_LIST_PORT" "$COMP_MENU_LIST_USER" "$COMP_MENU_LIST_KEY"); then
                COMP_MENU_LIST_LA="$seen/32"; UI_LIST_MSG="Sunucu sizi şu adresten görüyor: $seen"
            else UI_LIST_MSG="⚠️  Sunucudan sorgulanamadı (ağ/anahtar/host key)."; fi ;;
    esac
}
# list[cidr] tek ağla kaydedilirken uyarı: H o haliyle kaydeder, e düzenlemeye döner
_comp_menu_list_cidr_save_check() {
    local ans
    [ "${#UI_LIST_ITEMS[@]}" -eq 1 ] || return 0
    echo "⚠️  Listede yalnızca bir ağ var. Başka bir ağdan (ev LAN, ofis, başka VPN) bağlanırsanız SSH kapalı olur."
    IFS= read -r -p "Başka ağ da eklemek ister misiniz? [e/H]: " ans || return 0
    case "$ans" in e|E|evet|Evet) return 1 ;; esac
    return 0
}

# comp_menu_edit_list <TSV-satırı> <sunucu> → etkileşimli (ok tuşlu/fzf) liste editörü: list[string], list[int] ve list[cidr].
# Değişiklik Kaydet'e kadar dosyaya yazılmaz; Vazgeç hiçbir şey yazmaz.
# shellcheck disable=SC2034  # UI_LIST_* değişkenlerini ui_edit_list (dinamik kapsam) okur
comp_menu_edit_list() {
    local line="$1" server="$2" n t req d desc cur v rc prefix
    local -a UI_LIST_ITEMS UI_LIST_X_LABELS UI_LIST_X_DESCS UI_LIST_X_ACTS
    local UI_LIST_TITLE UI_LIST_NAME UI_LIST_META UI_LIST_DESC UI_LIST_REQUIRED UI_LIST_VALIDATE UI_LIST_ADD="" UI_LIST_EXTRAS="" UI_LIST_EXTRA_RUN="" UI_LIST_SAVE_CHECK="" UI_LIST_PROMPT UI_LIST_ADD_HINT="" UI_LIST_DIRTY UI_LIST_MSG
    n=$(_comp_menu_field "$line" 1); t=$(_comp_menu_field "$line" 2); req=$(_comp_menu_field "$line" 3)
    d=$(_comp_menu_field "$line" 4); desc=$(_comp_menu_field "$line" 8)
    cur=$(comp_vars_get "$COMP_MENU_VARS" "$n" 2>/dev/null) || cur=""
    UI_LIST_ITEMS=(); [ -z "$cur" ] || IFS=',' read -r -a UI_LIST_ITEMS <<< "$cur"
    COMP_MENU_LIST_NAME="$n"
    UI_LIST_TITLE="${UI_PICK_TITLE:-Değişkenler}"; UI_LIST_NAME="$n"; UI_LIST_DESC="$desc"; UI_LIST_REQUIRED="$req"
    UI_LIST_META="$t$([ "$req" = yes ] && echo ' · zorunlu') · yazılı: ${cur:-(yok)}${d:+ · varsayılan: $d}"
    UI_LIST_VALIDATE=_comp_menu_list_validate
    if [ "$t" = "list[cidr]" ]; then
        UI_LIST_PROMPT="Yeni CIDR (örn. 192.168.1.0/24): "; UI_LIST_ADD_HINT="CIDR, örn. 192.168.1.0/24. "
        UI_LIST_ADD=_comp_menu_list_cidr_add; UI_LIST_EXTRAS=_comp_menu_list_cidr_extras
        UI_LIST_EXTRA_RUN=_comp_menu_list_cidr_run; UI_LIST_SAVE_CHECK=_comp_menu_list_cidr_save_check
        prefix=$(module_prefix "$server")
        COMP_MENU_LIST_IP=$(srv_get "$COMP_MENU_DIR" "$prefix" IP); COMP_MENU_LIST_PORT=$(srv_get "$COMP_MENU_DIR" "$prefix" PORT); COMP_MENU_LIST_PORT="${COMP_MENU_LIST_PORT:-22}"
        COMP_MENU_LIST_USER=$(srv_get "$COMP_MENU_DIR" "$prefix" USER); COMP_MENU_LIST_KEY=$(srv_get "$COMP_MENU_DIR" "$prefix" KEY)
        [ -n "$COMP_MENU_LIST_KEY" ] || COMP_MENU_LIST_KEY="$COMP_MENU_DIR/id_ed25519_${server}"
        if [ "${COMP_MENU_LIST_KEY:0:1}" = "~" ]; then COMP_MENU_LIST_KEY="$HOME${COMP_MENU_LIST_KEY:1}"; fi
        COMP_MENU_LIST_LA=$(comp_net_local "$COMP_MENU_LIST_IP" 2>/dev/null) || COMP_MENU_LIST_LA=""
    fi
    ui_edit_list && rc=0 || rc=$?
    case "$rc" in
        1) [ "$UI_LIST_DIRTY" = 1 ] && COMP_MENU_FLASH="Vazgeçildi: $n değişmedi." ; return 0 ;;
        2) return 1 ;;
    esac
    if [ "${#UI_LIST_ITEMS[@]}" -eq 0 ]; then
        comp_vars_unset "$COMP_MENU_VARS" "$n"; COMP_MENU_FLASH="Silindi (varsayılana döner): $n"; return 0
    fi
    v=$(IFS=,; printf '%s' "${UI_LIST_ITEMS[*]}")
    if comp_vars_save "$COMP_MENU_MF" "$COMP_MENU_VARS" "$n" "$v" >/dev/null; then COMP_MENU_FLASH="✅ Kaydedildi: $n = $v"
    else COMP_MENU_FLASH="⚠️  Kaydedilemedi: $n"; fi
    return 0
}

# _comp_menu_interactive → 0 ise etkileşimli seçici (ok tuşlu/fzf) kullanılır; değilse numaralı mod (eski akış)
_comp_menu_interactive() { declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; }

# _comp_menu_done <ileti> → etkileşimli modda iletiyi bir sonraki ekranda gösterir (duraksama yok); değilse yazar ve Enter bekler
_comp_menu_done() {
    if _comp_menu_interactive; then COMP_MENU_FLASH="$1"; return 0; fi
    echo "$1"; read -r -p "Devam için Enter..." _ || return 1
}

# _comp_menu_pick_value <ad> <tür> <şimdiki> <varsayılan> <açıklama> <warn_if> <enum-değerleri> → seçilen değer (stdout);
# "" = değiştirme · "-" = sil. bool: true/false · enum: izinli değerler. Açıklama sağ bölmede, riskli değer işaretli.
_comp_menu_pick_value() {
    local n="$1" t="$2" cur="$3" d="$4" desc="$5" warn="$6" vals="$7" c eff mark i start=1 pick
    local -a choices labels
    if [ "$t" = bool ]; then choices=(true false)
    else choices=(); while IFS= read -r c; do [ -n "$c" ] && choices+=("$c"); done < <(printf '%s\n' "$vals" | tr -d '[]"' | tr ',' '\n'); fi
    [ "${#choices[@]}" -gt 0 ] || { printf ''; return 0; }
    eff="${cur:-$d}"; labels=(); i=0
    # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
    local -a UI_PICK_DESCS=()
    for c in "${choices[@]}"; do
        i=$((i + 1)); mark=""
        [ "$c" = "$eff" ] && { mark=" ← şimdiki"; [ -z "$cur" ] && mark=" ← şimdiki (varsayılan)"; }
        labels+=("$c$mark")
        if [ "$warn" = "$c" ] || [ "$warn" = "\"$c\"" ]; then
            UI_PICK_DESCS+=("⚠️  RİSKLİ değer: paketin uyarısı var, kaydederken onay istenir. ${desc:-}")
        else
            UI_PICK_DESCS+=("${desc:-açıklama yok}")
        fi
    done
    # bool: imleç şimdikinin tersinde (bir tuşla değiştirme); enum: şimdikinde
    i=0; for c in "${choices[@]}"; do
        i=$((i + 1))
        if [ "$t" = bool ]; then [ "$c" != "$eff" ] && { start=$i; break; }
        else [ "$c" = "$eff" ] && { start=$i; break; }; fi
    done
    if [ -n "$cur" ]; then
        labels+=("↩ Varsayılana dön (dosyadan sil)")
        UI_PICK_DESCS+=("Bu sunucu için yazılı değeri siler; paket varsayılanı kullanılır${d:+ ($d)}.")
    fi
    # shellcheck disable=SC2034  # ui_pick_arrows / _ui_pick_fzf (dinamik kapsam) okur
    local UI_PICK_WIDE=1 UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_START="$start" UI_PICK_BACK_DESC="Değeri değiştirmeden listeye döner." UI_PICK_HEIGHT=""
    # shellcheck disable=SC2034
    local UI_PICK_TITLE="${UI_PICK_TITLE:+$UI_PICK_TITLE › }$n" UI_PICK_HEADER="$t · yazılı: ${cur:-(yok)}${d:+ · varsayılan: $d}"
    ui_fit_wide "$(( ${#labels[@]} + 1 ))" 3
    pick=$(_comp_menu_pick "Geri (değiştirme)" "${labels[@]}")
    if ! [ "$pick" -ge 1 ] 2>/dev/null; then printf ''
    elif [ "$pick" -le "${#choices[@]}" ]; then printf '%s' "${choices[$((pick - 1))]}"
    else printf -- '-'; fi
}

# comp_menu_edit_value <TSV-satırı> → tek değişkeni düzenletir (doğrula → gerekirse onay → yaz)
# Etkileşimli modda (ok tuşlu/fzf): bool ve enum için seçici, yanlış değerde yeniden sorma, duraksamasız ileti. Numaralı mod: eski akış.
comp_menu_edit_value() {
    local line="$1" n t d lo hi desc cur v v2 out warn vals try=1 maxtry=1 inter=0
    n=$(_comp_menu_field "$line" 1); t=$(_comp_menu_field "$line" 2); d=$(_comp_menu_unq "$(_comp_menu_field "$line" 4)")
    lo=$(_comp_menu_field "$line" 5); hi=$(_comp_menu_field "$line" 6); warn=$(_comp_menu_field "$line" 7); desc=$(_comp_menu_field "$line" 8)
    vals=$(_comp_menu_field "$line" 10)
    cur=$(comp_vars_get "$COMP_MENU_VARS" "$n" 2>/dev/null) || cur=""
    if _comp_menu_interactive; then inter=1; maxtry=3; fi
    while [ "$try" -le "$maxtry" ]; do
        if [ "$inter" = 1 ] && { [ "$t" = bool ] || { [ "$t" = enum ] && [ -n "$vals" ]; }; }; then
            v=$(_comp_menu_pick_value "$n" "$t" "$cur" "$d" "$desc" "$warn" "$vals")
        else
            echo "$n — ${desc:-açıklama yok}"
            echo "Tür: $t${lo:+ · en az $lo}${hi:+ · en çok $hi}${d:+ · varsayılan $d}"
            echo "Şimdiki: ${cur:-(boş)}"
            IFS= read -r -p "Yeni değer (Enter = değiştirme, - = sil): " v || return 1
        fi
        case "$v" in
            "") return 0 ;;
            "-") comp_vars_unset "$COMP_MENU_VARS" "$n"; _comp_menu_done "Silindi (varsayılana döner): $n" || return 1; return 0 ;;
        esac
        if out=$(comp_vars_check "$COMP_MENU_MF" "$n" "$v" 2>&1); then break; fi
        if [ "$try" -lt "$maxtry" ]; then echo "⚠️  $out (deneme $try/$maxtry, tekrar girin)"; try=$((try + 1)); continue; fi
        _comp_menu_done "⚠️  $n: $out" || return 1; return 0
    done
    case "$out" in
        *warn=*)
            echo "⚠️  Bu değer riskli (paketin warn_if uyarısı)."
            IFS= read -r -p "Yine de kaydedilsin mi? [e/H]: " v2 || return 1
            case "$v2" in e|E|evet|Evet) ;; *) if [ "$inter" = 1 ]; then COMP_MENU_FLASH="İptal edildi: $n değişmedi."; else echo "İptal edildi."; fi; return 0 ;; esac ;;
    esac
    if comp_vars_set "$COMP_MENU_VARS" "$n" "$v"; then _comp_menu_done "✅ Kaydedildi: $n = $v" || return 1
    else _comp_menu_done "⚠️  Kaydedilemedi: $n" || return 1; fi
}

# _comp_menu_cidr_add <cidr> → list dizisine (dinamik kapsam) ekler; kapsanan/yinelenen/yerine geçen durumunu yazar
_comp_menu_cidr_add() {
    local c="$1" csv res st rest v
    csv=$(IFS=,; printf '%s' "${list[*]-}")
    res=$(comp_net_add "$csv" "$c") || { echo "⚠️  Geçersiz CIDR."; return 1; }
    st=$(printf '%s\n' "$res" | head -n 1); rest=$(printf '%s\n' "$res" | sed -n 2p)
    case "$st" in
        duplicate) echo "Zaten listede." ;;
        covered:*) echo "Zaten kapsanıyor: ${st#covered:} (daha geniş bir ağ listede)." ;;
        replaced:*) echo "Daha dar girdi(ler) yerine geçti, kapsadığı için kaldırıldı: ${st#replaced:}" ;;
    esac
    list=()
    IFS=',' read -r -a list <<< "$rest"
}

# comp_menu_edit_cidr <değişken> <sunucu> → CIDR listesi: öneri ekle / elle ekle / sil / temizle / kaydet
# Öneri yalnızca ŞU ANKİ bağlantının ağını bilir; başka ağlar (ev LAN, başka VPN) kullanıcıya sorulur. Hiçbir şey otomatik yazılmaz.
comp_menu_edit_cidr() {
    local n="$1" server="$2" cur la seen prefix ip port user key choice c x i k v ans out
    local -a list sc sd keep
    prefix=$(module_prefix "$server")
    ip=$(srv_get "$COMP_MENU_DIR" "$prefix" IP); port=$(srv_get "$COMP_MENU_DIR" "$prefix" PORT); port="${port:-22}"
    user=$(srv_get "$COMP_MENU_DIR" "$prefix" USER); key=$(srv_get "$COMP_MENU_DIR" "$prefix" KEY)
    [ -n "$key" ] || key="$COMP_MENU_DIR/id_ed25519_${server}"
    if [ "${key:0:1}" = "~" ]; then key="$HOME${key:1}"; fi
    cur=$(comp_vars_get "$COMP_MENU_VARS" "$n" 2>/dev/null) || cur=""
    list=()
    if [ -n "$cur" ]; then
        IFS=',' read -r -a list <<< "$cur"
    fi
    la=$(comp_net_local "$ip" 2>/dev/null) || la=""
    while true; do
        sc=(); sd=()
        if [ -n "$la" ]; then
            while IFS=$'\t' read -r c x; do [ -n "$c" ] && { sc+=("$c"); sd+=("$x"); }; done < <(comp_net_suggest "$la")
        fi
        echo "$n — SSH'a izin verilen ağlar (CIDR). Bu liste sunucunun güvenlik duvarına (ufw) yazılır; listede olmayan ağlardan SSH kapanır."
        if [ "${#list[@]}" -gt 0 ]; then echo "Mevcut liste: $(IFS=,; printf '%s' "${list[*]}")"; else echo "Mevcut liste: (boş)"; fi
        if [ -n "$la" ]; then echo "Bu bilgisayarın sunucuya bağlandığı adres: $la"; else echo "Bu bilgisayarın adresi belirlenemedi (öneri yok); elle ekleyin."; fi
        i=1
        for c in ${sc[@]+"${sc[@]}"}; do echo "  $i) ekle: $c  — ${sd[$((i - 1))]}"; i=$((i + 1)); done
        echo "  e) elle ekle   d) listeden sil   t) temizle   v) sunucunun gördüğü adresi sorgula (salt okunur ssh)   k) kaydet   0) iptal"
        IFS= read -r -p "Seçim: " choice || return 1
        case "$choice" in
            0|q|Q) return 0 ;;
            e|E)
                IFS= read -r -p "CIDR (örn. 192.168.1.0/24): " c || return 1
                if out=$(comp_vars_check "$COMP_MENU_MF" "$n" "$c" 2>&1); then _comp_menu_cidr_add "$c"; else echo "⚠️  $out"; fi ;;
            d|D)
                IFS= read -r -p "Silinecek sıra numarası: " x || return 1
                if [[ "$x" =~ ^[0-9]+$ ]] && [ "$x" -ge 1 ] && [ "$x" -le "${#list[@]}" ]; then
                    k=0; keep=()
                    for v in "${list[@]}"; do k=$((k + 1)); [ "$k" -eq "$x" ] || keep+=("$v"); done
                    list=()
                    for v in ${keep[@]+"${keep[@]}"}; do list+=("$v"); done
                else echo "Geçersiz numara."; fi ;;
            t|T) list=() ;;
            v|V)
                if seen=$(comp_net_seen_by_server "$ip" "$port" "$user" "$key"); then la="$seen/32"; echo "Sunucu seni şu adresten görüyor: $seen"
                else echo "⚠️  Sunucudan sorgulanamadı (ağ/anahtar/host key)."; fi ;;
            k|K)
                if [ "${#list[@]}" -eq 0 ]; then echo "⚠️  Liste boş olamaz (zorunlu)."; continue; fi
                if [ "${#list[@]}" -eq 1 ]; then
                    echo "⚠️  Listede yalnızca bir ağ var. Başka bir ağdan (ev LAN, ofis, başka VPN) bağlanırsanız SSH kapalı olur."
                    IFS= read -r -p "Başka ağ da eklemek ister misiniz? [e/H]: " ans || return 1
                    case "$ans" in e|E|evet|Evet) continue ;; esac
                fi
                v=$(IFS=,; printf '%s' "${list[*]}")
                if comp_vars_save "$COMP_MENU_MF" "$COMP_MENU_VARS" "$n" "$v" >/dev/null; then echo "✅ Kaydedildi: $v"; else echo "⚠️  Kaydedilemedi."; fi
                read -r -p "Devam için Enter..." _ || return 1
                return 0 ;;
            *)
                if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le "${#sc[@]}" ]; then
                    _comp_menu_cidr_add "${sc[$((choice - 1))]}"
                else echo "Geçersiz seçim."; fi ;;
        esac
    done
}

# comp_menu_packages → Ana menü › Paketler ve güncelleme (fzf): kilitteki paketler kartlı liste; Enter: kur/onar (onaylı) ya da doğrula.
# Altında sshpocket'in kendi güncellemesi (UpdateAction: imzalı paket, onaylı; sürüm değişirse paketler eşitlenir) ve toplu eşitle, eski sürümleri temizle, yeni yayın var mı.
# Hiçbir şey sormadan kurmaz: kurulum/onarım klasik onay sorusunu (comp_cli_sync) korur. Seçici olmayan modda ComponentsAction eski durum sayfasını gösterir.
comp_menu_packages() {
    local ids id st want have mark repo pick n msg="" need verdict kd nold act
    local -a idarr labels acts
    while true; do
        comp_lock_validate || return 1
        ids=$(comp_ids)
        idarr=(); labels=(); acts=(); need=0
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=()
        while IFS= read -r id; do
            [ -n "$id" ] || continue
            st=$(comp_status "$id"); want=$(comp_get "$id" version); have=$(comp_installed_version "$id"); repo=$(comp_get "$id" repo)
            case "$st" in
                ok) mark="✅ kurulu" ;;
                missing) mark="⬇️  kurulmamış"; need=$((need + 1)) ;;
                outdated) mark="⬆️  güncellenmeli"; need=$((need + 1)) ;;
                ahead) mark="✅ kilitten yeni (indeksle yükseltilmiş)" ;;
                broken) mark="❌ dosyalar değişmiş"; need=$((need + 1)) ;;
                *) mark="$st" ;;
            esac
            idarr+=("$id"); acts+=("pkg:$id")
            labels+=("$(printf '%-18s kilit %-8s kurulu %-8s %s' "$id" "$want" "${have:--}" "$mark")")
            kd="$(comp_dir)/$id"; nold=$(find "$kd" -mindepth 1 -maxdepth 1 -type d ! -name "$want" ! -name '*.bozuk-*' 2>/dev/null | wc -l | tr -d ' ')
            case "$st" in
                ok) verdict="Enter: kurulumu doğrula (dosya özetleri)." ;;
                ahead) verdict="Kurulu sürüm kilitten yeni (imzalı indeksle yükseltilmiş); geri alınmaz." ;;
                broken) verdict="Enter: bozuk kurulum kenara alınır, kilitteki sürüm imzası doğrulanarak yeniden kurulur (onay sorulur)." ;;
                *) verdict="Enter: kilitteki sürüm indirilir, imzası ve sha256'sı doğrulanarak kurulur (onay sorulur)." ;;
            esac
            UI_PICK_DESCS+=("$(ui_card_head "📦 $id"; ui_card_row Kilit "$want"; ui_card_row Kurulu "${have:--}"; ui_card_row Durum "$mark"; ui_card_row Depo "$repo"
                [ "${nold:-0}" -eq 0 ] || ui_card_row Eski "$nold sürüm klasörü diskte (🧹 Eski paket sürümlerini temizle)"
                printf '\n%s\n' "$verdict")")
        done <<< "$ids"
        n=${#idarr[@]}
        # paketlerin ALTINDA: sshpocket güncellemesi
        labels+=("⬆️  sshpocket ${SCRIPT_VERSION:-?} — güncellemeleri kontrol et / güncelle"); acts+=(update)
        UI_PICK_DESCS+=("$(ui_card_head '⬆️  sshpocket'; ui_card_row Sürüm "${SCRIPT_VERSION:-?}"; ui_card_row Paketler "$n kilitte"
            printf '\nYeni sürüm var mı bakar (ağ); imzalı paketi doğrulayıp onayınızla kurar.\nSunucularınıza ve key'"'"'lerinize dokunmaz.\nSürüm değişirse paketler kilitteki sürüme eşitlenir ve eski sürümler temizlenir.\n')")
        if [ "$need" -gt 0 ]; then labels+=("🔄 Kilitteki sürümleri kur / onar ($need paket)"); acts+=(sync); UI_PICK_DESCS+=("$(ui_card_head '🔄 Eşitle'; printf '\nKurulmamış, eski ya da bozuk %s paketi kilitteki sürüme getirir.\nHer biri için onay sorulur; indirme imza ve sha256 ile doğrulanır.\n' "$need")"); fi
        labels+=("🧹 Eski paket sürümlerini temizle"); acts+=(prune); UI_PICK_DESCS+=("$(ui_card_head '🧹 Temizle'; printf '\nGüncellemelerde yan yana biriken eski sürümleri listeler ve silinmesini sorar\n(etkin + önceki + kilitteki sürüm kalır).\n')")
        labels+=("🧾 Paket indeksini kontrol et (ağ)"); acts+=(idxupdate); UI_PICK_DESCS+=("$(ui_card_head '🧾 İndeks'; printf '\nİmzalı paket indeksini alır, doğrular ve hangi paketlerin yükseltilebilir olduğunu gösterir.\nHiçbir şey kurmaz.\n')")
        labels+=("⬆️  Kurulu paketleri indeksten yükselt (onaylı)"); acts+=(idxupgrade); UI_PICK_DESCS+=("$(ui_card_head '⬆️  Yükselt'; printf '\nKurulu paketlerin indeksteki daha yeni sürümlerini listeler; her biri için onay sorar.\nKurulu olmayan paketi bu giriş KURMAZ (kurmak için: upgrade <paket>).\nİndirme: sha256 (indeks) + imza (kilit imzacısı) ile doğrulanır; geri sürüm yok.\n')")
        labels+=("🔎 Kilitten yeni yayın var mı? (ağ)"); acts+=(newrel); UI_PICK_DESCS+=("$(ui_card_head '🔎 Yeni yayın'; printf '\nGitHub yayınlarına bakar (yalnızca bilgi, ağ gerekir).\nYeni sürüm kurulmaz: yeni sshpocket sürümüyle (kilit güncellenince) gelir.\n')")
        clear
        ui_screen "Ana menü › Paketler ve güncelleme"
        # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
        local UI_PICK_WIDE=1 UI_PICK_HEIGHT=16 UI_PICK_BACK_DESC="Ana menüye döner." UI_PICK_HEADER=""
        # shellcheck disable=SC2034
        UI_PICK_HEADER="${msg:+$msg$'\n'}sshpocket ${SCRIPT_VERSION:-?} · kilit: $n paket · $([ "$need" -gt 0 ] && echo "$need eksik/eski/bozuk" || echo 'paketler tamam')"
        msg=""
        ui_fit_wide "$(( ${#labels[@]} + 1 ))" 6
        pick=$(_comp_menu_pick "Geri" "${labels[@]}")
        echo ""
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        act="${acts[$((pick - 1))]}"
        case "$act" in
            pkg:*)
                id="${act#pkg:}"; st=$(comp_status "$id")
                case "$st" in
                    ok) if comp_verify_installed "$id" "$(comp_installed_version "$id")"; then msg="✅ $id doğrulandı: dosyalar kurulumdaki gibi."; else msg="❌ $id doğrulanamadı: dosyalar değişmiş (Enter ile onarın)."; fi ;;
                    ahead) msg="✅ $id: kurulu sürüm kilitten yeni (indeksle yükseltilmiş); dokunulmadı." ;;
                    *) COMP_SYNC_ONLY="$id" comp_cli_sync || true; read -r -p "Devam için Enter..." _ || return 0 ;;
                esac ;;
            update) UpdateAction; read -r -p "Devam için Enter..." _ || return 0 ;;
            sync) comp_cli_sync || true; read -r -p "Devam için Enter..." _ || return 0 ;;
            prune) bash "$SISTEM_PATH/Main.sh" --components prune || true; read -r -p "Devam için Enter..." _ || return 0 ;;
            idxupdate) comp_cli_update || true; read -r -p "Devam için Enter..." _ || return 0 ;;
            idxupgrade) comp_cli_upgrade || true; read -r -p "Devam için Enter..." _ || return 0 ;;
            newrel) comp_cli_check_new || true; read -r -p "Devam için Enter..." _ || return 0 ;;
        esac
    done
}

# comp_menu_actions <paket> <sunucu> → paketin eylem paneli; bildirimde items[] (katalog) varsa önce yazılım seçtirir (S2)
comp_menu_actions() {
    local id="$1" server="$2" info
    info=$(comp_prepare "$id") || { echo "Paket doğrulanamadı: $id (./start.sh --components list)" >&2; return 1; }
    if [ -n "$(comp_info_all "$info" item)" ]; then comp_menu_items "$id" "$server" "$info"; return $?; fi
    _comp_menu_actions_panel "$id" "$server"
}

# comp_menu_items <paket> <sunucu> <bilgi> → yazılım kataloğu: yazılım (ve birden çok modu varsa mod) seçilir; sonra yalnız o yazılımın eylemleri paneli açılır.
# Seçim eylem çalıştırılırken COMP_CTX_RUNSET ile koşuya özel verilir (item_var/mode_var; değişken dosyasına yazılmaz).
comp_menu_items() {
    local id="$1" server="$2" info="$3" row iid label pick mode modes n
    local -a ids labels descs mlist
    while true; do
        ids=(); labels=(); descs=()
        while IFS= read -r row; do
            [ -n "$row" ] || continue
            iid="${row%%|*}"; label="${row#*|}"
            ids+=("$iid"); labels+=("$label")
            modes=$(comp_info_get "$info" "item.$iid.modes")
            descs+=("$(comp_info_get "$info" "item.$iid.description")${modes:+$'\n'Modlar: ${modes//,/, }}")
        done < <(comp_info_all "$info" item)
        clear
        if declare -F ui_screen >/dev/null; then ui_screen "Sunucu › $server › $id › Yazılım kataloğu"; fi
        # shellcheck disable=SC2034  # ui_pick_arrows / _ui_pick_fzf (dinamik kapsam) okur
        local -a UI_PICK_DESCS=("${descs[@]}")
        # shellcheck disable=SC2034
        local UI_PICK_BACK_DESC="Paket listesine döner."
        if declare -F ui_card_on >/dev/null && ui_card_on; then
            # shellcheck disable=SC2034
            local UI_PICK_WIDE=1 UI_PICK_HEIGHT=14
            ui_fit_wide "$(( ${#labels[@]} + 1 ))" 4
        fi
        pick=$(_comp_menu_pick "Geri" "${labels[@]}")
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        iid="${ids[$((pick - 1))]}"
        mode=""
        modes=$(comp_info_get "$info" "item.$iid.modes")
        if [ -n "$modes" ]; then
            IFS=, read -r -a mlist <<< "$modes"
            if [ "${#mlist[@]}" -gt 1 ]; then
                # shellcheck disable=SC2034
                local -a UI_PICK_DESCS=()
                n=$(_comp_menu_pick "Geri" "${mlist[@]}")
                [ "$n" -ge 1 ] 2>/dev/null || continue
                mode="${mlist[$((n - 1))]}"
            else mode="${mlist[0]}"; fi
        fi
        COMP_CTX_RUNSET="$(comp_info_get "$info" item_var)=$iid"
        [ -z "$mode" ] || COMP_CTX_RUNSET="$COMP_CTX_RUNSET"$'\n'"$(comp_info_get "$info" mode_var)=$mode"
        COMP_MENU_ITEM_ACTIONS="$(comp_info_get "$info" "item.$iid.actions")"
        # 📝 Değişkenler satırı yalnız bu yazılımın params alanlarını (genel form) gösterir; params yoksa tüm değişkenler
        COMP_MENU_VARS_ONLY="$(comp_info_get "$info" "item.$iid.params")"
        COMP_MENU_VARS_TITLE="${labels[$((pick - 1))]} › Ayarlar"
        _comp_menu_actions_panel "$id" "$server"
        COMP_CTX_RUNSET=""; COMP_MENU_ITEM_ACTIONS=""; COMP_MENU_VARS_ONLY=""; COMP_MENU_VARS_TITLE=""
    done
}

# comp_menu_server <önek> <sunucu-klasörü> [kategori] → kurulu paket(ler)den (kategoriye göre süzülmüş) birini seçtirir
comp_menu_server() {
    local module_dir="$2" cat="${3:-}" server pick
    local -a ids
    server=$(basename "$module_dir")
    ids=()
    while IFS= read -r pick; do [ -n "$pick" ] && ids+=("$pick"); done < <(comp_menu_installed "$cat")
    [ "${#ids[@]}" -gt 0 ] || { echo "Kurulu paket yok. Ana menü › Paketler ile kurun." >&2; return 0; }
    if [ "${#ids[@]}" -eq 1 ]; then comp_menu_actions "${ids[0]}" "$server"; return $?; fi
    while true; do
        clear
        if declare -F ui_screen >/dev/null; then ui_screen "Sunucu › $server › Paketler"; fi
        if ! { declare -F ui_card_on >/dev/null && ui_card_on; }; then echo ""; fi
        local -a labels=()
        for pick in "${ids[@]}"; do labels+=("$pick $(comp_installed_version "$pick")"); done
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=()
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local UI_PICK_BACK_DESC="Sunucu menüsüne döner."
        if declare -F ui_card_on >/dev/null && ui_card_on; then
            # fzf: paket başına kart (sürüm, bu sunucudaki son preview/apply/audit ve Lynis skoru), geniş düzen
            for pick in "${ids[@]}"; do
                UI_PICK_DESCS+=("$(ui_card_head "📦 $pick $(comp_installed_version "$pick")"
                    if [ -f "$module_dir/components.log" ] && grep -qF "|$pick|" "$module_dir/components.log"; then _server_card_pkg "$module_dir/components.log" "$(_comp_gate_state_dir)" "$pick" "$server" | tail -n +3   # boş satır ve kendi başlığı atlanır (kart başlığı yukarıda)
                    else printf ' Bu sunucuda henüz eylem çalıştırılmadı.\n'; fi)")
            done
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            local UI_PICK_WIDE=1 UI_PICK_HEIGHT=14
            ui_fit_wide "$(( ${#labels[@]} + 1 ))" 4
        fi
        pick=$(_comp_menu_pick "Geri" "${labels[@]}")
        [ "$pick" -ge 1 ] 2>/dev/null || return 0
        comp_menu_actions "${ids[$((pick - 1))]}" "$server"
    done
}
