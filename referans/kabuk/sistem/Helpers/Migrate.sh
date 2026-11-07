#!/usr/bin/env bash
# Migrate.sh - Eski düzeni (sistem/Modules/<ad>/.env + üretilmiş modül) yeni düzene (servers/<ad>/server.env) taşır
#
# İlkeler:
#  * KOPYALA → DOĞRULA → ancak sonra eskiyi kenara al. Hiçbir adımda veri SİLİNMEZ.
#  * Her sunucu önce servers/.migrating-<ad>/ içinde hazırlanır, doğrulanınca tek hamlede (rename) yerine konur.
#  * Hata olursa o sunucunun eski klasörü olduğu gibi kalır, yarım servers/<ad> bırakılmaz.
#  * İdempotent: taşınacak bir şey yoksa hiçbir şey yapmaz. Yeni biçimde aynı ad varsa atlar.
#  * Eski klasörler servers/.migrated-<tarih>/<ad>/ altına alınır (key'lerin kopyası orada kalır; elle silinir).
#  * Keychain'e dokunulmaz: hesap adı klasör adından türediği için passphrase'ler aynen geçerlidir.
# Gerektirir: Validate.sh, ModuleFiles.sh (module_prefix), Servers.sh (servers_dir, srv_get), SSHModule.sh, SISTEM_PATH

# Taşınacak eski sunucu adları (.env'i olanlar; gönderilen, yapılandırılmamış eski 'example' şablonunda .env yoktur)
migrate_legacy_names() {
    local mdir="$SISTEM_PATH/Modules" p n
    [ -d "$mdir" ] || return 0
    for p in "$mdir"/*/; do
        [ -d "$p" ] || continue
        n=$(basename "$p")
        module_name_valid "$n" || continue
        [ -f "${p}.env" ] || continue
        printf '%s\n' "$n"
    done
}

# Eski .env içeriğini yeni biçime çevirir: <ÖNEK>_ADI=değer → ADI=değer; diğer satırlar aynen.
# Parametreler: $1 - eski .env, $2 - önek
migrate_translate_env() {
    local line trimmed
    while IFS= read -r line || [ -n "$line" ]; do
        trimmed="${line#"${line%%[![:space:]]*}"}"
        case "$trimmed" in
            "$2"_*=*) printf '%s\n' "${trimmed#"$2"_}" ;;
            *)        printf '%s\n' "$line" ;;
        esac
    done < "$1"
}

# Plan satırı (değerleri gösterir; key içeriği/parola asla). Parametre: $1 - ad
migrate_plan_line() {
    local n="$1" src ip user port keys=0 f
    src="$SISTEM_PATH/Modules/$n"
    ip=$(srv_get "$src" "$(module_prefix "$n")" IP)
    user=$(srv_get "$src" "$(module_prefix "$n")" USER)
    port=$(srv_get "$src" "$(module_prefix "$n")" PORT)
    for f in "$src"/id_*; do [ -f "$f" ] && keys=$((keys + 1)); done
    printf '  • %-18s %s@%s:%s   (%s key dosyası)\n' "$n" "${user:-?}" "${ip:-?}" "${port:-22}" "$keys"
}

# Tek sunucuyu taşır. Dönüş: 0 tamam, 2 atlandı (zaten var), 1 hata (eski klasör olduğu gibi kalır)
# Parametre: $1 - ad
migrate_one() {
    local n="$1" prefix src dst stg f k a b
    prefix=$(module_prefix "$n")
    src="$SISTEM_PATH/Modules/$n"
    dst="$(servers_dir)/$n"
    stg="$(servers_dir)/.migrating-$n"

    if [ -e "$dst" ]; then
        echo "⏭️  $n: servers/ içinde zaten var, atlandı (eski klasör olduğu gibi duruyor)"
        return 2
    fi
    if [ -d "$stg" ]; then rm -r -- "$stg"; fi

    mkdir -p "$(servers_dir)" && chmod 700 "$(servers_dir)" 2>/dev/null
    if ! mkdir "$stg"; then echo "⚠️  $n: hazırlık klasörü oluşturulamadı"; return 1; fi
    chmod 700 "$stg"

    # 1) ayar dosyası
    if ! migrate_translate_env "$src/.env" "$prefix" > "$stg/server.env"; then
        echo "⚠️  $n: ayar dosyası dönüştürülemedi"; rm -r -- "$stg"; return 1
    fi
    chmod 600 "$stg/server.env"

    # 2) key dosyaları (kopyala; eskisine dokunma)
    for f in "$src"/id_*; do
        [ -f "$f" ] || continue
        if ! cp -p "$f" "$stg/$(basename "$f")"; then
            echo "⚠️  $n: $(basename "$f") kopyalanamadı"; rm -r -- "$stg"; return 1
        fi
        case "$f" in *.pub) ;; *) chmod 600 "$stg/$(basename "$f")" ;; esac
    done

    # 3) doğrulama: değerler ve key dosyaları eskisiyle aynı mı
    for k in IP USER PORT KEY OS; do
        a=$(srv_get "$src" "$prefix" "$k")
        b=$(srv_get "$stg" "$prefix" "$k")
        if [ "$a" != "$b" ]; then
            echo "⚠️  $n: doğrulama başarısız ($k eşleşmiyor); hiçbir şey değiştirilmedi"
            rm -r -- "$stg"; return 1
        fi
    done
    for f in "$src"/id_*; do
        [ -f "$f" ] || continue
        if ! cmp -s "$f" "$stg/$(basename "$f")"; then
            echo "⚠️  $n: doğrulama başarısız ($(basename "$f") farklı); hiçbir şey değiştirilmedi"
            rm -r -- "$stg"; return 1
        fi
    done

    # 4) tek hamlede yerine koy
    if ! mv "$stg" "$dst"; then
        echo "⚠️  $n: yerine konamadı"; rm -r -- "$stg" 2>/dev/null; return 1
    fi
    echo "✅ $n taşındı → servers/$n"
    return 0
}

# Ana akış. Seçenekler: --dry-run (yalnızca plan), --yes (onay sorma)
run_migration() {
    local dry=0 yes=0 a names n rc ok=() failed=0 stamp bak
    for a in "$@"; do
        case "$a" in
            --dry-run) dry=1 ;;
            --yes)     yes=1 ;;
        esac
    done

    names=$(migrate_legacy_names)
    if [ -z "$names" ]; then
        echo "Taşınacak eski biçimli sunucu yok."
        return 0
    fi

    echo ""
    echo "Eski biçimli sunucular bulundu (sistem/Modules/):"
    while IFS= read -r n; do migrate_plan_line "$n"; done <<< "$names"
    echo ""
    echo "Her biri servers/<ad>/ altına taşınacak (ayarlar genel anahtarlara çevrilir, key'ler kopyalanır ve doğrulanır)."
    echo "Eski klasörler SİLİNMEZ: servers/.migrated-<tarih>/ altına alınır. Keychain'deki passphrase'lere dokunulmaz."
    if [ "$dry" = "1" ]; then
        echo "(deneme modu: hiçbir şey değiştirilmedi)"
        return 0
    fi
    if [ "$yes" != "1" ]; then
        local ans
        read -r -p "Taşınsın mı? (y/n) " ans || return 1
        case "$ans" in y|Y) ;; *) echo "İptal edildi. Hiçbir şey değiştirilmedi."; return 1 ;; esac
    fi

    while IFS= read -r n; do
        migrate_one "$n"
        rc=$?
        case "$rc" in
            0) ok+=("$n") ;;
            2) ;;
            *) failed=1 ;;
        esac
    done <<< "$names"

    # Yalnızca başarıyla taşınanların eski klasörünü kenara al
    if [ "${#ok[@]}" -gt 0 ]; then
        stamp=$(date +%Y%m%d-%H%M%S)
        bak="$(servers_dir)/.migrated-$stamp"
        mkdir -p "$bak" && chmod 700 "$bak"
        for n in "${ok[@]}"; do
            if mv "$SISTEM_PATH/Modules/$n" "$bak/$n"; then
                :
            else
                echo "⚠️  $n: eski klasör kenara alınamadı (yeni kopya geçerli; eski klasör yerinde)"
            fi
        done
        # Taşınacak bir şey kalmadıysa boş eski klasörü de kaldır (yalnızca boşsa; rmdir)
        rmdir "$SISTEM_PATH/Modules" 2>/dev/null || true
        echo ""
        echo "Eski klasörler: $bak"
        echo "ℹ️  Orada key dosyalarının bir kopyası kalır. Yeni düzenin çalıştığını doğruladıktan sonra o klasörü kendiniz silin."
    fi
    return "$failed"
}

# Eski sürümlerin gönderdiği, hiç yapılandırılmamış 'example' şablonunu (yalnızca ExampleModule.sh/.env.example/.DS_Store) kaldırır.
# İçinde .env, key (id_*) ya da başka bir kullanıcı dosyası varsa DOKUNMAZ. Açılışta çağrılır; boş bir "EXAMPLE" menü satırını önler.
migrate_cleanup_stale_example() {
    local d="$SISTEM_PATH/Modules/example" f extra=0
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
        rmdir "$SISTEM_PATH/Modules" 2>/dev/null || true
        Log "INFO" "Eski, yapılandırılmamış 'example' şablonu temizlendi"
    fi
    return 0
}

