#!/usr/bin/env bash
# SSHConfigImport.sh - ~/.ssh/config içindeki Host kayıtlarını okur

# Joker karakterli (*, ?, !) olmayan Host kayıtlarını yazdırır.
# Çıktı satırı: alias|hostname|user|port|identityfile
# Not: Include/Match blokları desteklenmez (Match sonrası ayarlar atlanır).
# Parametre: $1 - config dosyası
ssh_config_hosts() {
    awk '
    function flush(   j) {
        for (j = 1; j <= n; j++) {
            print alias[j] "|" (host == "" ? alias[j] : host) "|" user "|" port "|" idf
        }
        n = 0
    }
    {
        line = $0
        sub(/^[ \t]+/, "", line)
        if (line == "" || line ~ /^#/) next
        kw = line; sub(/[ \t=].*/, "", kw)
        val = line; sub(/^[^ \t=]+[ \t]*=?[ \t]*/, "", val)
        gsub(/^"|"$/, "", val)
        k = tolower(kw)
        if (k == "host") {
            flush(); host = ""; user = ""; port = ""; idf = ""
            m = split(val, parts, /[ \t]+/)
            for (j = 1; j <= m; j++) {
                if (parts[j] != "" && parts[j] !~ /[*?!]/) { n++; alias[n] = parts[j] }
            }
            inblock = 1
            next
        }
        if (k == "match") { flush(); inblock = 0; next }
        if (!inblock) next
        if (k == "hostname" && host == "") host = val
        else if (k == "user" && user == "") user = val
        else if (k == "port" && port == "") port = val
        else if (k == "identityfile" && idf == "") idf = val
    }
    END { flush() }
    ' "$1"
}

# Host takma adından geçerli bir modül adı üretir
# Parametre: $1 - alias
ssh_config_module_name() {
    local name
    name=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9\n' '_')
    case "$name" in [a-z]*) ;; *) name="h_${name}" ;; esac
    printf '%s' "$name"
}
