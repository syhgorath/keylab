package servers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Knock.sh (istemci tarafı port knocking) ve SSHModule.sh ssh_port_open / ssh_module_status karşılığı.
// Knock dizisi düz metin dosyada TUTULMAZ: Keychain hesabı USBMonitor_<ÖNEK>_Knock. server.env: KNOCK=keychain, KNOCK_PROTO, KNOCK_DELAY_MS.

const KeychainService = "USBMonitor"

// KnockAccount, knock_account.
func KnockAccount(prefix string) string { return "USBMonitor_" + prefix + "_Knock" }

// PassphraseAccount, ssh_keychain_account.
func PassphraseAccount(prefix string) string { return "USBMonitor_" + prefix + "_Passphrase" }

// KnockPortsValid, knock_ports_valid: "7000,8000,9000" — 1-10 port, her biri 1-65535.
func KnockPortsValid(seq string) bool {
	parts := strings.Split(seq, ",")
	if len(parts) < 1 || len(parts) > 10 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

// KnockEnabled, knock_enabled.
func KnockEnabled(dir string) bool {
	v, _ := Get(dir, "KNOCK")
	return v == "keychain"
}

// KnockSettings: server.env'den protokol ve gecikme (varsayılan tcp, 300 ms); geçersizse hata (kabuk knock_pre_connect iletileri).
func KnockSettings(dir string) (proto string, delayMS int, err error) {
	proto, _ = Get(dir, "KNOCK_PROTO")
	if proto == "" {
		proto = "tcp"
	}
	ds, _ := Get(dir, "KNOCK_DELAY_MS")
	if ds == "" {
		ds = "300"
	}
	if proto != "tcp" && proto != "udp" {
		return "", 0, fmt.Errorf("KNOCK_PROTO geçersiz (%s): tcp veya udp olmalı", proto)
	}
	n, aerr := strconv.Atoi(ds)
	if aerr != nil || strings.Trim(ds, "0123456789") != "" || n > 5000 {
		return "", 0, fmt.Errorf("KNOCK_DELAY_MS geçersiz (%s): 0-5000 olmalı", ds)
	}
	return proto, n, nil
}

// Keychain: macOS `security` aracı (testte PATH'teki sahte araçla değiştirilir).
type Keychain struct{}

// Find, hesabın değerini verir (security find-generic-password -w).
func (Keychain) Find(account string) (string, bool) {
	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", KeychainService, "-w").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSuffix(string(out), "\n"), true
}

// Has: kayıt var mı (değeri okumadan).
func (Keychain) Has(account string) bool {
	return exec.Command("security", "find-generic-password", "-a", account, "-s", KeychainService).Run() == nil
}

// Delete: kaydı siler; yoksa false.
func (Keychain) Delete(account string) bool {
	return exec.Command("security", "delete-generic-password", "-a", account, "-s", KeychainService).Run() == nil
}

// Dialer: knock vuruşu ve port yoklaması için bağlantı kurucu (testte yerel dinleyiciye yönlendirilir).
var dialTimeout = func(network, addr string, d time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, addr, d)
}

// KnockSend, knock_send: her port için tek vuruş (en çok 1 sn), aralarında delayMS bekler. Hatalar yok sayılır (kapalı porta SYN gitmiştir).
func KnockSend(ctx context.Context, ip, seq, proto string, delayMS int) {
	for _, p := range strings.Split(seq, ",") {
		n, _ := strconv.Atoi(p)
		addr := net.JoinHostPort(ip, strconv.Itoa(n))
		c, err := dialTimeout(proto, addr, time.Second)
		if err == nil {
			if proto == "udp" {
				c.SetWriteDeadline(time.Now().Add(time.Second))
				c.Write([]byte("x"))
			}
			c.Close()
		}
		if delayMS > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(delayMS) * time.Millisecond):
			}
		}
	}
}

// PreConnect, knock_pre_connect: etkin değilse hiçbir şey yapmaz. Keychain/ayar bozuksa hata (bağlantı denenmemeli).
func PreConnect(ctx context.Context, kc Keychain, dir, prefix, ip string) error {
	if !KnockEnabled(dir) {
		return nil
	}
	proto, delay, err := KnockSettings(dir)
	if err != nil {
		return err
	}
	seq, ok := kc.Find(KnockAccount(prefix))
	if !ok {
		return errors.New("Knock dizisi Keychain'de yok; 'Knock ayarları'nı yapın")
	}
	if !KnockPortsValid(seq) {
		return errors.New("Keychain'deki knock dizisi geçersiz")
	}
	KnockSend(ctx, ip, seq, proto, delay)
	return nil
}

// PortOpen, ssh_port_open: TCP bağlantısı en çok secs saniyede kurulabiliyor mu.
func PortOpen(ip, port string, secs float64) bool {
	if secs <= 0 {
		secs = 1
	}
	c, err := dialTimeout("tcp", net.JoinHostPort(ip, port), time.Duration(secs*float64(time.Second)))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// Status, ssh_module_status: "green" (açık), "red" (kapalı), "lock" (kapalı ama knock gerekir), "unset" (ayar yok/geçersiz).
// Durum denetimi knock GÖNDERMEZ.
func Status(serversDir, name string, timeoutSec float64) string {
	d := joinDir(serversDir, name)
	if !fileExists(d + "/server.env") {
		return "unset"
	}
	ip, _ := Get(d, "IP")
	port, _ := Get(d, "PORT")
	if port == "" {
		port = "22"
	}
	if ip == "" || ValidateTarget(ip, "x", port) != nil {
		return "unset"
	}
	if PortOpen(ip, port, timeoutSec) {
		return "green"
	}
	if KnockEnabled(d) {
		return "lock"
	}
	return "red"
}
