package main

// Veri klasörlerinin çözümü: taşınabilir (USB/klasör) ya da kullanıcıya özel (~/.keylab).
//
// Sunucular klasörü (kullanıcı verisi: server.env + anahtarlar). Sıra:
//  1. -servers bayrağı
//  2. $KEYLAB_SERVERS (ya da eski $SSHPOCKET_SERVERS)
//  3. TAŞINABİLİR: ikilinin yanındaki servers/ (ikili X.app/Contents/MacOS içindeyse X.app'in yanındaki servers/)
//  4. ~/.keylab/servers varsa o
//  5. eski geliştirici davranışı: çalışma klasörünün üstündeki ../servers varsa o
//  6. hiçbiri yoksa ~/.keylab/servers (ilk açılışta oluşturulur, 0700)
// Eskiden burada geliştiricinin kendi USB'sine ait sabit bir yol (/Volumes/test/servers) vardı; kaldırıldı: USB'den çalışan bir ikili
// zaten 3. adımla kendi yanındaki servers/ klasörünü bulur.
//
// components/ klasörü varsayılan olarak sunucular klasörünün yanındadır (kök klasör = ikisinin üstü): taşınabilirde USB kökü,
// kurulu kullanımda ~/.keylab. Kök klasörde components.lock, index_signers, release_signers ve VERSION aranır (bkz. findRoot).
// Not: macOS indirilen imzasız .app'i "App Translocation" ile rastgele salt okunur bir yolda çalıştırabilir; orada servers/ bulunamaz
// ve ~/.keylab kullanılır (veri kaybolmaz, kurulu mod gibi davranır).

import (
	"keylab/internal/envx"
	"os"
	"path/filepath"
	"strings"
)

const homeDataName = ".keylab"

func isDir(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

// appBase: ikilinin "taşınabilir kök"ü: X.app/Contents/MacOS/<ikili> ise X.app'in bulunduğu klasör, değilse ikilinin klasörü.
func appBase(exe string) string {
	d := filepath.Dir(exe)
	if filepath.Base(d) == "MacOS" && filepath.Base(filepath.Dir(d)) == "Contents" {
		if app := filepath.Dir(filepath.Dir(d)); strings.HasSuffix(app, ".app") {
			return filepath.Dir(app)
		}
	}
	return d
}

// homeDataDir: kurulu kullanımın veri kökü (~/.keylab); ev klasörü bilinmiyorsa "".
func homeDataDir() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return filepath.Join(h, homeDataName)
}

// resolveServersDir: yukarıdaki sıra. create=true: klasör yok, çağıran oluşturmalı (yalnız 6. adım).
func resolveServersDir(flagVal, envVal, exe, cwd string) (dir string, create bool) {
	if flagVal != "" {
		return flagVal, false
	}
	if envVal != "" && isDir(envVal) {
		return envVal, false
	}
	if p := filepath.Join(appBase(exe), "servers"); isDir(p) {
		return p, false
	}
	home := homeDataDir()
	if home != "" {
		if p := filepath.Join(home, "servers"); isDir(p) {
			return p, false
		}
	}
	if cwd != "" {
		if p := filepath.Join(filepath.Dir(cwd), "servers"); isDir(p) {
			return p, false
		}
	}
	if home == "" {
		return "", false
	}
	return filepath.Join(home, "servers"), true
}

// stateDirFor: durum/günlük klasörünün varsayılanı (-state bayrağı verilmediyse). TAŞINABİLİR modda (sunucular klasörü ikilinin/.app'in
// yanındaki servers/ ise) sunucular klasörünün yanındaki state/ kullanılır: günlükler, denetim kayıtları, sertleştirme sonuçları USB ile
// birlikte gider ve bilgisayarda iz kalmaz. $KEYLAB_STATE_DIR her zaman önceliklidir. Diğer modlarda ~/.local/share/keylab (defaultStateDir).
// Ayrı Chromium profili BURAYA konmaz (bkz. main.go): Chromium profilindeki sembolik bağ (SingletonLock) FAT/exFAT USB'de oluşturulamaz.
// Python önbelleği (~/.cache/keylab) ve Keychain da bilgisayarda kalır.
func stateDirFor(servers, exe string) string {
	if envx.Get("STATE_DIR") == "" && filepath.Clean(servers) == filepath.Join(appBase(exe), "servers") {
		return filepath.Join(filepath.Dir(filepath.Clean(servers)), "state")
	}
	return defaultStateDir()
}

// ensureHomeDirs: kurulu modda ilk açılışta klasörleri oluşturur (servers 0700: anahtarlar burada durur).
func ensureHomeDirs(servers, comps string) error {
	if err := os.MkdirAll(servers, 0o700); err != nil {
		return err
	}
	return os.MkdirAll(comps, 0o755)
}

func envServers() string { return envx.Get("SERVERS") }
