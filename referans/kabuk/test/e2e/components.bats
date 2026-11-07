#!/usr/bin/env bats
# Gerçek bir Ubuntu 26.04 (systemd'li Docker LAB'ı) sunucusuna karşı paket akışı: sshpocket → ssh → ansible → ufw/sshd/systemd.
#   ./test/e2e/lab/up.sh && bats test/e2e/components.bats && ./test/e2e/lab/down.sh
# LAB yoksa/çalışmıyorsa testler ATLANIR. Varsayılan `bats test` bu klasörü ÇALIŞTIRMAZ.
# Testler LAB'ın DURUMUNA göre atlar (temiz/uygulanmış), bu yüzden sıra bağımsızdır; ama tam akış için up.sh ile SIFIRDAN başlayın.
# Gerekenler: docker (Colima/Docker Desktop), expect, ~/.cache/sshpocket altında kurulu linux-hardening venv'i
#   (yoksa: E2E_INSTALL_VENV=e ile ilk çalıştırma onaylı kurar; PyPI'dan ~10 MB indirir).
load ../test_helper

setup() {
    LABDIR="$ROOT/test/e2e/lab"; STATE="$LABDIR/.state"
    command -v docker >/dev/null || skip "docker yok"
    command -v expect >/dev/null || skip "expect yok"
    [ -f "$STATE/kh" ] || skip "önce ./test/e2e/lab/up.sh"
    [ "$(docker inspect -f '{{.State.Running}}' sshpocket-lab 2>/dev/null)" = "true" ] || skip "sshpocket-lab konteyneri çalışmıyor"
    PORT="${LAB_PORT:-2223}"
    LAB_SSH=(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o ConnectTimeout=8 -o UserKnownHostsFile="$STATE/kh" -p "$PORT" -i "$STATE/id_lab" tester@127.0.0.1)
}

# lab_run <eylem> → çıktı (CR temizlenmiş) ve çıkış kodu; ANSI ve Ansible gürültüsü kalır, aramalar buna göre yapılır
lab_run() { run "$LABDIR/run.sh" "$@"; output="$(printf '%s' "$output" | tr -d '\r')"; }
in_lab() { docker exec sshpocket-lab sh -c "$1"; }
applied() { in_lab 'test -e /etc/ssh/sshd_config.d/00-hardening.conf' 2>/dev/null; }
fresh_or_skip() { ! applied || skip "LAB zaten uygulanmış; sıfırdan başlamak için up.sh"; }
applied_or_skip() { applied || skip "LAB'a henüz apply yapılmamış"; }

@test "lab: sshd'ye anahtarla girilir, sudo parolasızdır ve işletim sistemi Ubuntu 26.04" {
    run "${LAB_SSH[@]}" 'sudo -n true && . /etc/os-release && echo "$PRETTY_NAME"'
    [ "$status" -eq 0 ]
    [[ "$output" == *"Ubuntu 26.04"* ]] || false
}

@test "rollback_status (temiz sunucu): installed=no ve sonuç satırı ayrıştırılır" {
    fresh_or_skip
    lab_run rollback_status
    [ "$status" -eq 0 ]
    [[ "$output" == *"Profil: ubuntu-26.04"* ]] || false
    [[ "$output" == *"installed=no"* ]] || false
    [[ "$output" == *"failed=0"* ]] || false
}

@test "apply, taze bir preview olmadan REDDEDİLİR ve sunucu değişmez" {
    fresh_or_skip
    rm -rf "$STATE/userdata"
    lab_run apply
    [ "$status" -eq 190 ]
    [[ "$output" == *"önce 'preview'"* ]] || false
    run in_lab 'test -e /etc/ssh/sshd_config.d/00-hardening.conf'
    [ "$status" -ne 0 ]
}

@test "preview: başarılı olur ve sunucuda HİÇBİR ŞEY değişmez" {
    fresh_or_skip
    lab_run preview
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    run in_lab 'dpkg -s dma >/dev/null 2>&1 || test -e /etc/ssh/sshd_config.d/00-hardening.conf || test -e /etc/modprobe.d/60-hardening-usb.conf'
    [ "$status" -ne 0 ]
    grep -q '|preview|ok$' "$STATE/userdata/linux-hardening/lab/results"
}

# ---------- GERİ ALMA GÜVENLİK AĞI (apply'ın en kritik güvencesi): sunucu kendi kendini kurtarır ----------
preview_ok_or_skip() { grep -q '|preview|ok$' "$STATE/userdata/linux-hardening/lab/results" 2>/dev/null || skip "önce preview testi"; }
server_unchanged() {   # sunucu hardening öncesi hâlinde: drop-in yok, ufw pasif, zamanlayıcı kapalı, armed=no
    run in_lab 'test -e /etc/ssh/sshd_config.d/00-hardening.conf'; [ "$status" -ne 0 ]
    run in_lab 'ufw status | head -1';                                  [[ "$output" == *"inactive"* ]] || false
    run in_lab 'systemctl is-active hardening-rollback.timer';          [ "$output" = "inactive" ]
    run in_lab '/usr/local/sbin/hardening-rollback status';             [[ "$output" == *"armed=no"* ]] || false
    run in_lab 'sshd -T | grep -i "^passwordauthentication"';            [[ "$output" == *"passwordauthentication yes"* ]] || false
}

@test "GERİ ALMA (A): yeni bağlantı doğrulaması BAŞARISIZ olursa apply önceki duruma döner ve 'fail' kaydedilir" {
    fresh_or_skip; preview_ok_or_skip
    LAB_BREAK_VERIFY=1 lab_run apply
    [ "$status" -ne 0 ]
    [[ "$output" == *"ROLLBACK - restore previous state"* ]] || false
    [[ "$output" == *"rescued=1"* ]] || false
    server_unchanged
    grep -q '|apply|fail$' "$STATE/userdata/linux-hardening/lab/results"
}

@test "GERİ ALMA (B): DENETLEYİCİ KAYBOLURSA (ansible SSH reload'unda öldürülür) sunucu zamanlayıcıyla kendini geri alır" {
    fresh_or_skip; preview_ok_or_skip
    LAB_BREAK_VERIFY=1 E2E_KILL_ON_RELOAD=1 lab_run apply
    [[ "$output" == *"ANSIBLE ÖLDÜRÜLDÜ"* ]] || false
    # öldürülme anında sunucu DEĞİŞMİŞ ve güvenlik ağı KURULU (armed)
    run in_lab 'test -e /etc/ssh/sshd_config.d/00-hardening.conf'
    [ "$status" -eq 0 ]
    run in_lab '/usr/local/sbin/hardening-rollback status'
    [[ "$output" == *"armed=yes"* ]] || false
    # hardening_rollback_timeout=30 sn sonra systemd zamanlayıcısı geri alır (en çok 60 sn bekle)
    local i
    for i in $(seq 1 30); do
        sleep 2
        in_lab 'test ! -e /etc/ssh/sshd_config.d/00-hardening.conf' && break
    done
    server_unchanged
    run in_lab 'journalctl -t hardening-rollback --no-pager -n 20'
    [[ "$output" == *"geri alma tamamlandi"* ]] || false
}

@test "apply (preview sonrası): başarılı olur; SSH, ufw ve servisler beklenen duruma gelir; güvenlik ağı disarm edilir" {
    fresh_or_skip
    grep -q '|preview|ok$' "$STATE/userdata/linux-hardening/lab/results" 2>/dev/null || skip "önce preview testi"
    lab_run apply
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    run in_lab 'sshd -T | grep -E "^(permitrootlogin no|passwordauthentication no|allowtcpforwarding no|maxauthtries 3)$" | wc -l'
    [ "$output" = "4" ]
    run in_lab 'ufw status verbose'
    [[ "$output" == *"Default: deny (incoming)"* ]] || false
    [[ "$output" == *"22/tcp"*"ALLOW IN"*"172.17.0.0/16"* ]] || false
    run in_lab 'systemctl is-active fail2ban ufw'
    [ "${lines[0]}" = "active" ]
    run in_lab 'systemctl is-active hardening-rollback.timer'
    [ "$output" = "inactive" ]
    run in_lab 'dpkg -s postfix >/dev/null 2>&1'
    [ "$status" -ne 0 ]
    # erişim hâlâ var (yeni bağlantı doğrulaması + disarm çalıştı)
    run "${LAB_SSH[@]}" echo ERISIM-VAR
    [ "$status" -eq 0 ]
}

@test "rollback_status (apply sonrası): installed=yes armed=no ve sonuç saklanır" {
    applied_or_skip
    lab_run rollback_status
    [ "$status" -eq 0 ]
    [[ "$output" == *"installed=yes"* ]] || false
    [[ "$output" == *"armed=no"* ]] || false
    [ -f "$STATE/userdata/linux-hardening/lab/result.rollback_status" ]
}

@test "ERİŞİM AĞI TUZAĞI: izin verilmeyen bir ağdan SSH zaman aşımına uğrar; ufw kapatılınca aynı istemci girer" {
    applied_or_skip
    docker network create sshpocket-other >/dev/null 2>&1 || true
    docker network connect sshpocket-other sshpocket-lab >/dev/null 2>&1 || true
    local ip
    ip=$(docker inspect -f '{{(index .NetworkSettings.Networks "sshpocket-other").IPAddress}}' sshpocket-lab)
    [ -n "$ip" ]
    client() { cat "$STATE/id_lab" | docker run --rm -i --network sshpocket-other --entrypoint /bin/sh sshpocket-lab:ubuntu26 -c \
        "cat > /tmp/k && chmod 600 /tmp/k && ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=6 -i /tmp/k tester@$ip echo ERISIM-VAR"; }
    run client
    [ "$status" -ne 0 ]
    [[ "$output" == *"timed out"* ]] || false
    in_lab 'ufw disable' >/dev/null
    run client
    in_lab 'ufw --force enable' >/dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"ERISIM-VAR"* ]] || false
}

# ---------- audit ve elle geri alma eylemleri (apply sonrası, sunucu sertleştirilmiş) ----------
RB='/usr/local/sbin/hardening-rollback'
# Güvenlik ağını elle kurar ve zamanlayıcıyı durdurur: "armed" kalır ama süre dolunca kendiliğinden geri almaz,
# böylece rollback_now/rollback_disarm sshpocket üzerinden, yarışsız denenir.
arm_frozen() { in_lab "$RB arm >/dev/null && systemctl stop hardening-rollback.timer"; }
hardened() { in_lab 'test -e /etc/ssh/sshd_config.d/00-hardening.conf && ufw status | head -1 | grep -q "Status: active"'; }

@test "audit: lynis kurulur, rapor kullanıcının klasörüne 0600/0700 ile iner ve summary.json üretilir" {
    applied_or_skip
    rm -rf "$STATE/userdata/linux-hardening/lab/out"
    lab_run audit
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    run in_lab 'dpkg -s lynis'
    [ "$status" -eq 0 ]
    local sum dir
    sum="$(find "$STATE/userdata/linux-hardening/lab/out" -name summary.json)"
    [ -n "$sum" ]
    [ "$(printf '%s\n' "$sum" | wc -l | tr -d ' ')" = "1" ]
    dir="$(dirname "$sum")"
    [ "$(file_mode "$sum")" = "600" ]
    [ "$(file_mode "$dir/lynis-report.dat")" = "600" ]
    [ "$(file_mode "$dir/lynis.log")" = "600" ]
    [ "$(file_mode "$dir")" = "700" ]
    [ "$(file_mode "$(dirname "$dir")")" = "700" ]
    run python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert isinstance(d["hardening_index"], int) and d["host"]=="lab" and d["tests_performed"]>0' "$sum"
    [ "$status" -eq 0 ]
    grep -q '|audit|ok$' "$STATE/userdata/linux-hardening/lab/results"
}

@test "rollback_now (armed DEĞİLKEN): hiçbir şey yapmaz, sunucu sertleştirilmiş kalır" {
    applied_or_skip
    run in_lab "$RB status"
    [[ "$output" == *"armed=no"* ]] || false
    lab_run rollback_now
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    hardened
    run in_lab 'journalctl -t hardening-rollback --no-pager -n 5'
    [[ "$output" == *"armed degil"* ]] || false
    grep -q '|rollback_now|ok$' "$STATE/userdata/linux-hardening/lab/results"
}

@test "rollback_now (armed): armdan SONRAKİ değişiklikler geri alınır (sshd drop-in ve ufw); erişim sürer" {
    applied_or_skip
    arm_frozen
    in_lab 'echo "# test-isareti" >> /etc/ssh/sshd_config.d/00-hardening.conf && ufw allow 8080/tcp >/dev/null'
    run in_lab 'grep -c test-isareti /etc/ssh/sshd_config.d/00-hardening.conf; ufw status | grep -c 8080'
    [ "${lines[0]}" = "1" ]
    [ "${lines[1]}" != "0" ]
    lab_run rollback_now
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    run in_lab 'grep -c test-isareti /etc/ssh/sshd_config.d/00-hardening.conf'
    [ "$output" = "0" ]
    run in_lab 'ufw status | grep -c 8080'
    [ "$output" = "0" ]
    hardened
    run in_lab "$RB status"
    [[ "$output" == *"armed=no"* ]] || false
    run in_lab 'systemctl is-active hardening-rollback.timer'
    [ "$output" = "inactive" ]
    run "${LAB_SSH[@]}" echo ERISIM-VAR
    [ "$status" -eq 0 ]
}

@test "rollback_disarm: geri almayı İPTAL eder (armed ve yedek silinir), sunucu sertleştirilmiş kalır" {
    applied_or_skip
    arm_frozen
    run in_lab "$RB status"
    [[ "$output" == *"armed=yes"* ]] || false
    lab_run rollback_disarm
    [ "$status" -eq 0 ]
    [[ "$output" == *"failed=0"* ]] || false
    run in_lab "$RB status"
    [[ "$output" == *"armed=no"* ]] || false
    run in_lab 'test -e /var/lib/hardening-rollback/armed || test -e /var/lib/hardening-rollback/00-hardening.conf'
    [ "$status" -ne 0 ]
    run in_lab 'systemctl is-enabled hardening-rollback.timer'
    [ "$output" != "enabled" ]
    hardened
    grep -q '|rollback_disarm|ok$' "$STATE/userdata/linux-hardening/lab/results"
}

@test "pve_firewall_preview, Proxmox OLMAYAN platformda sshpocket tarafından reddedilir (ssh'a gitmeden); sunucuda hiçbir şey değişmez" {
    local before after
    before=$(in_lab 'ufw status | head -1; ls /etc/pve 2>&1 | head -1; ls /etc/ssh/sshd_config.d | wc -l')
    lab_run pve_firewall_preview
    [ "$status" -ne 0 ]
    [[ "$output" == *"bu sunucunun platformunda (generic) kullanılamaz; yalnızca: proxmox-host"* ]] || false
    after=$(in_lab 'ufw status | head -1; ls /etc/pve 2>&1 | head -1; ls /etc/ssh/sshd_config.d | wc -l')
    [ "$before" = "$after" ]
    run in_lab 'test -e /etc/pve'
    [ "$status" -ne 0 ]
    run grep -q '|pve_firewall_preview|' "$STATE/userdata/linux-hardening/lab/results"
    [ "$status" -ne 0 ]
}

# Proxmox benzeri LAB: /etc/pve + pveversion işaretleri ve sunucu değişkenlerinde hardening_pve_apply=true (ana `apply` için gerekir).
# 0.7.0'da bu durumda pve_firewall_preview cluster.fw'yi GERÇEKTEN yazıyordu; 0.8.0'da önizleme yazma kapısını daima kapalı tutar.
pve_lab_cleanup() {
    in_lab 'rm -rf /etc/pve /usr/bin/pveversion' 2>/dev/null || true
    [ -f "$STATE/servers/lab/linux-hardening.vars" ] && sed -i.bak -e '/^hardening_pve_apply=/d' -e '/^hardening_platform=/d' "$STATE/servers/lab/linux-hardening.vars" && rm -f "$STATE/servers/lab/linux-hardening.vars.bak"
    return 0
}
teardown() { [ -f "$STATE/kh" ] && pve_lab_cleanup; return 0; }

@test "pve_firewall_preview (Proxmox benzeri LAB + hardening_pve_apply=true): hiçbir şey yazılmaz, restart çalışmaz, güvenlik ağı arm edilmez" {
    local ver cmd
    ver=$(cat "$ROOT/components/linux-hardening/current")
    cmd=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['actions']['pve_firewall_preview']['cmd'])" "$ROOT/components/linux-hardening/$ver/component.json")
    [[ "$cmd" == *"--check"* ]] || false
    [[ "$cmd" != *" -e "* && "$cmd" != *" -i "* && "$cmd" != *"--limit"* ]] || false
    in_lab 'mkdir -p /etc/pve/firewall && printf "#!/bin/sh\necho pve-manager/9.0.0\n" > /usr/bin/pveversion && chmod +x /usr/bin/pveversion'
    printf 'hardening_pve_apply=true\nhardening_platform=proxmox-host\n' >> "$STATE/servers/lab/linux-hardening.vars"
    local armed_before
    armed_before=$(in_lab 'systemctl is-active hardening-rollback.timer 2>&1; ls /etc/pve/firewall | wc -l')
    E2E_RISKY=e lab_run pve_firewall_preview
    [ "$status" -eq 0 ]
    [[ "$output" == *"ÖNİZLEME (yazılmadı)"* ]] || false
    [[ "$output" != *"Apply now"*"changed:"* ]] || false
    [[ "$output" != *"Write the file ENABLED"*"changed:"* ]] || false
    run in_lab 'test -e /etc/pve/firewall/cluster.fw'
    [ "$status" -ne 0 ]
    [ "$armed_before" = "$(in_lab 'systemctl is-active hardening-rollback.timer 2>&1; ls /etc/pve/firewall | wc -l')" ]
    grep -q '|pve_firewall_preview|ok$' "$STATE/userdata/linux-hardening/lab/results"
}

# ---------- hata yolları: sunucuya dokunulmaz, temiz ret ----------
@test "sunucu erişilemezse (konteyner duraklatıldı): 'algılanamadı' ile durur, Ansible başlamaz" {
    docker pause sshpocket-lab >/dev/null
    lab_run rollback_status
    docker unpause sshpocket-lab >/dev/null
    [ "$status" -eq 190 ]
    [[ "$output" == *"işletim sistemi algılanamadı"* ]] || false
    [[ "$output" == *"Zaman aşımı"* ]] || false              # gerçek OpenSSH mesajından sınıflandırılan ipucu
    [[ "$output" != *"PLAY RECAP"* ]] || false
}

@test "host key tanınmıyorsa (known_hosts boş) çalıştırılmaz" {
    cp "$STATE/kh" "$STATE/kh.bak"; : > "$STATE/kh"
    lab_run rollback_status
    cp "$STATE/kh.bak" "$STATE/kh"; rm -f "$STATE/kh.bak"
    [ "$status" -eq 190 ]
    [[ "$output" == *"host key'i tanınmıyor"* ]] || false
    [[ "$output" != *"PLAY RECAP"* ]] || false
}

@test "host key DEĞİŞMİŞSE (sahte anahtar) bağlantı reddedilir; host key doğrulaması kapatılmaz" {
    cp "$STATE/kh" "$STATE/kh.bak"
    local fk="$BATS_TEST_TMPDIR/fakehost"
    ssh-keygen -q -t ed25519 -N '' -f "$fk"
    { printf '[127.0.0.1]:%s ' "$PORT"; cut -d' ' -f1,2 "$fk.pub"; } > "$STATE/kh"
    lab_run rollback_status
    cp "$STATE/kh.bak" "$STATE/kh"; rm -f "$STATE/kh.bak"
    [ "$status" -eq 190 ]
    [[ "$output" == *"algılanamadı"* ]] || false
    [[ "$output" == *"host key'i DEĞİŞMİŞ"* ]] || false      # gerçek OpenSSH "REMOTE HOST IDENTIFICATION HAS CHANGED" uyarısından
    [[ "$output" != *"PLAY RECAP"* ]] || false
}
