#!/usr/bin/env bash
# ComponentsReport.sh - Paket sürümü değişince "ne değişti, hangi sunucum etkileniyor" raporu (yalnızca okur; hiçbir şey yazmaz/değiştirmez)
# Eski sürüm: kurulu (etkin) sürümden hemen önceki, diskte hâlâ duran sürüm (güncellemelerde yan yana birikir; prune silmeden önce).
# Gerektirir: Components.sh, ComponentsInstall.sh, ComponentsManifest.sh, ComponentsRun.sh, ComponentsVars.sh, ComponentsGates.sh, Servers.sh; isteğe bağlı: ComponentsOnboard.sh

COMP_REPORT_MAX="${COMP_REPORT_MAX:-12}"      # bölüm başına en çok satır

# comp_report_prev_version <paket> → etkin sürümden küçük en büyük, component.json'u duran sürüm (yoksa boş)
comp_report_prev_version() {
    local id="$1" cur d v best=""
    cur=$(comp_installed_version "$id")
    [ -n "$cur" ] || return 0
    for d in "$(comp_dir)/$id"/*/; do
        [ -f "${d}component.json" ] || continue
        v=$(basename "$d")
        comp_version_valid "$v" || continue
        [ "$(comp_version_cmp "$v" "$cur")" = "-1" ] || continue
        if [ -z "$best" ] || [ "$(comp_version_cmp "$v" "$best")" = "1" ]; then best="$v"; fi
    done
    printf '%s' "$best"
}

# comp_report_vars_diff <eski component.json> <yeni component.json> → TSV: "+<ad><tür><varsayılan>" eklendi · "-<ad>" kalktı · "t<ad><eski tür><yeni tür>" · "d<ad><eski><yeni>" varsayılan değişti
comp_report_vars_diff() {
    {
        comp_vars_describe "$1" | awk -F'\t' '{ print "o\t" $1 "\t" $2 "\t" $4 }'
        comp_vars_describe "$2" | awk -F'\t' '{ print "n\t" $1 "\t" $2 "\t" $4 }'
    } | awk -F'\t' '
        $1 == "o" { ot[$2]=$3; od[$2]=$4; ord[++no]=$2 }
        $1 == "n" { nt[$2]=$3; nd[$2]=$4; nrd[++nn]=$2 }
        END {
            for (i = 1; i <= nn; i++) { k=nrd[i]
                if (!(k in ot)) print "+\t" k "\t" nt[k] "\t" nd[k]
                else if (ot[k] != nt[k]) print "t\t" k "\t" ot[k] "\t" nt[k]
                else if (od[k] != nd[k]) print "d\t" k "\t" od[k] "\t" nd[k] }
            for (i = 1; i <= no; i++) { k=ord[i]; if (!(k in nt)) print "-\t" k }
        }'
}

# _comp_report_cap <başlık> → stdin satırlarını girintili yazar; en çok COMP_REPORT_MAX, fazlası "… ve N tane daha"
_comp_report_cap() {
    local line n=0 extra=0
    while IFS= read -r line; do
        [ -n "$line" ] || continue
        if [ "$n" -le "$COMP_REPORT_MAX" ]; then printf '    %s\n' "$line"; else extra=$((extra + 1)); fi
    done
    [ "$extra" -eq 0 ] || printf '    … ve %s tane daha\n' "$extra"
    [ "$n" -gt 0 ]
}

# _comp_report_short <metin> → en çok 40 karakter
_comp_report_short() { local t="$1"; if [ "${#t}" -gt 40 ]; then printf '%s…' "${t:0:39}"; else printf '%s' "$t"; fi; }

# _comp_report_vars_lines <eski> <yeni> → okunur değişken farkı satırları (case, $(…) içinde değil: bash 3.2 ayrıştırıcısı)
_comp_report_vars_lines() {
    local k n t d
    while IFS=$'\t' read -r k n t d; do
        case "$k" in
            "+") printf 'yeni değişken: %s (%s%s)\n' "$n" "$t" "${d:+, varsayılan $(_comp_report_short "$d")}" ;;
            "-") printf 'kalkan değişken: %s\n' "$n" ;;
            t) printf 'tür değişti: %s: %s → %s\n' "$n" "$t" "$d" ;;
            d) printf 'varsayılan değişti: %s: %s → %s\n' "$n" "$(_comp_report_short "$t")" "$(_comp_report_short "$d")" ;;
        esac
    done < <(comp_report_vars_diff "$1" "$2")
}

# comp_report_servers <paket> <yeni component.json> <yeni bilgi> → bu makinedeki sunucular için etki satırları (boşsa hiçbir şey yazmaz)
comp_report_servers() {
    local id="$1" mf="$2" info="$3" srv dir vf act rv v acts names miss out=""
    while IFS= read -r srv; do
        [ -n "$srv" ] || continue
        dir="$(servers_dir)/$srv"
        srv_is_current "$dir" 2>/dev/null || continue
        vf=$(comp_vars_file "$dir" "$id")
        [ -f "$vf" ] || continue                      # bu sunucuda paket hiç ayarlanmamış
        acts=""; names=" "
        for act in $(comp_info_all "$info" action); do
            rv=$(comp_gate_unmet_vars "$info" "$act" "$vf" | tr '\n' ' ')
            [ -n "${rv// /}" ] || continue
            acts="$acts${acts:+, }$act"
            for v in $rv; do case "$names" in *" $v "*) ;; *) names="$names$v " ;; esac; done
        done
        names=$(printf '%s' "$names" | sed -e 's/^ *//' -e 's/ *$//')
        [ -z "$acts" ] || out="$out${out:+$'\n'}$srv: $acts ⛔ — değeri boş: $names (Paketler › Değişkenler)"
        miss=$(comp_vars_missing "$mf" "$vf" | tr '\n' ' ')
        [ -z "${miss// /}" ] || out="$out${out:+$'\n'}$srv: zorunlu değişken eksik: ${miss% }"
        if declare -F comp_onboard_needed >/dev/null && comp_onboard_needed "$id" "$vf"; then
            out="$out${out:+$'\n'}$srv: sunucu türü seçilmemiş (paket menüsü ilk girişte soracak)"
        fi
    done < <(srv_names)
    [ -z "$out" ] || printf '%s\n' "$out"
    return 0
}

# comp_report_pkg <paket> [eski-sürüm] → eski ↔ yeni (etkin) sürüm farkı ve sunucu etkisi. Eski verilmezse diskteki bir önceki sürüm.
comp_report_pkg() {
    local id="$1" old="${2:-}" new omf nmf oi ni lines a b any=0
    new=$(comp_installed_version "$id")
    [ -n "$new" ] || { echo "$id: kurulu değil." >&2; return 1; }
    [ -n "$old" ] || old=$(comp_report_prev_version "$id")
    if [ -z "$old" ] || [ "$old" = "$new" ]; then echo "$id $new: karşılaştırılacak eski sürüm yok."; return 0; fi
    omf="$(comp_dir)/$id/$old/component.json"; nmf="$(comp_dir)/$id/$new/component.json"
    oi=$(comp_manifest_info "$omf" "$id" "$old" 2>/dev/null) || { echo "$id $old: eski bildirim okunamadı; rapor yok." >&2; return 1; }
    ni=$(comp_manifest_info "$nmf" "$id" "$new" 2>/dev/null) || { echo "$id $new: bildirim okunamadı; rapor yok." >&2; return 1; }
    echo "📋 $id $old → $new: ne değişti?"

    lines=$(_comp_report_vars_lines "$omf" "$nmf")
    if [ -n "$lines" ]; then echo "  Değişkenler:"; printf '%s\n' "$lines" | _comp_report_cap && any=1; fi

    lines=""
    for a in $(comp_info_all "$ni" action); do
        b=$(comp_action_get "$ni" "$a" requires_vars)
        [ -n "$b" ] && [ "$b" != "$(comp_action_get "$oi" "$a" requires_vars)" ] && lines="$lines${lines:+$'\n'}$a artık şu değişken(ler) dolu olmadan çalışmaz: $b"
    done
    if [ -n "$lines" ]; then echo "  Yeni kurallar:"; printf '%s\n' "$lines" | _comp_report_cap && any=1; fi

    a=$(comp_info_all "$ni" platform_choice | tr '\n' ' '); b=$(comp_info_all "$oi" platform_choice | tr '\n' ' ')
    if [ -n "$a" ] && [ "$a" != "$b" ]; then
        echo "  Sunucu türü seçimi: ${a% }${b:+ (eskiden: ${b% })}"; any=1
    fi
    [ "$any" = 1 ] || echo "  Değişken ve kural farkı yok."

    lines=$(comp_report_servers "$id" "$nmf" "$ni")
    if [ -n "$lines" ]; then echo "  Sunucularınız:"; printf '%s\n' "$lines" | _comp_report_cap; fi
    echo "  (Bu rapor yalnızca okur; hiçbir şey değiştirilmedi.)"
}

# comp_report_all → kurulu her paket için comp_report_pkg
comp_report_all() {
    local id
    while IFS= read -r id; do
        [ -n "$id" ] || continue
        [ -n "$(comp_installed_version "$id")" ] || continue
        comp_report_pkg "$id" || true
    done < <(comp_ids 2>/dev/null)
}
