package main

// Veri klasörlerinin çözümü: taşınabilir (USB/klasör) ya da kullanıcıya özel (~/.keylab).
//
// Sunucular klasörü (kullanıcı verisi: server.env + anahtarlar). Sıra:
//  1. -servers bayrağı
//  2. $KEYLAB_SERVERS (ya da eski $SSHPOCKET_SERVERS)
//  3. TAŞINABİLİR: ikilinin yanındaki servers/ (ikili X.app/Contents/MacOS içindeyse X.app'in yanındaki servers/); uygulama HARİCİ SÜRÜCÜDEYSE
//     (USB, /Volumes/<ad>) servers/ yoksa yanında kendisi oluşturulur (yazılamıyorsa, ör. salt okunur disk imajı, 4./6. adımlara düşer)
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

// resolveServersDir: yukarıdaki sıra (3. adıma, uygulama harici sürücüdeyse "servers/ yoksa yanında oluştur" eklenir: onExternal).
// create=true: klasör yok, çağıran oluşturmalı.
func resolveServersDir(flagVal, envVal, exe, cwd string, onExternal bool) (dir string, create bool) {
	if flagVal != "" {
		return flagVal, false
	}
	if envVal != "" && isDir(envVal) {
		return envVal, false
	}
	if p := filepath.Join(appBase(exe), "servers"); isDir(p) {
		return p, false
	}
	if onExternal { // uygulama harici bir sürücüde (USB): veri de yanında; servers/ yoksa kendisi açılır, kullanıcı klasör açmaz
		return filepath.Join(appBase(exe), "servers"), true
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

// needsFirstRunSetup: components/ klasörü ve gömülü paket kilidi bu açılışta hazırlanmalı mı? Kurulu mod (sunucular klasörü ~/.keylab/servers,
// klasör daha önce var olsa bile) ve taşınabilir mod (servers/ ikilinin/.app'in yanında) için evet; -servers ile verilen özel klasörler için hayır.
func needsFirstRunSetup(servers, exe string, created bool) bool {
	if created {
		return true
	}
	s := filepath.Clean(servers)
	if s == filepath.Join(appBase(exe), "servers") {
		return true
	}
	h := homeDataDir()
	return h != "" && s == filepath.Join(h, "servers")
}

// ensureHomeDirs: kurulu modda ilk açılışta klasörleri oluşturur (servers 0700: anahtarlar burada durur).
func ensureHomeDirs(servers, comps string) error {
	if err := os.MkdirAll(servers, 0o700); err != nil {
		return err
	}
	return os.MkdirAll(comps, 0o755)
}

func envServers() string { return envx.Get("SERVERS") }

// dataDirs: açılışta kullanılacak veri klasörleri.
type dataDirs struct {
	servers, components string
	created             bool   // servers/ (ve components/) bu açılışta oluşturuldu
	note                string // kullanıcıya gösterilecek açıklama (geri dönüş vb.); boş olabilir
}

// prepareDataDirs: sunucular/components klasörlerini çözer ve gerekiyorsa oluşturur. Harici sürücüde oluşturma başarısız olursa
// (salt okunur birim) sessizce eski davranışa (~/.keylab) döner ve nedenini note'ta söyler. servers=="" ise ev klasörü bilinmiyor demektir.
func prepareDataDirs(flagServers, flagComps, envVal, exe, cwd string, onExternal bool) (dataDirs, error) {
	pick := func(ext bool) (dataDirs, bool) {
		sd, create := resolveServersDir(flagServers, envVal, exe, cwd, ext)
		d := dataDirs{servers: sd, created: create}
		if sd == "" {
			return d, true
		}
		d.components = flagComps
		if d.components == "" {
			d.components = filepath.Join(filepath.Dir(filepath.Clean(sd)), "components")
		}
		return d, false
	}
	d, empty := pick(onExternal)
	if empty {
		return d, nil
	}
	if d.created {
		if err := ensureHomeDirs(d.servers, d.components); err != nil {
			if !onExternal {
				return d, err
			}
			note := "harici sürücüye yazılamadı (" + err.Error() + "); veriler bu bilgisayarda (~/.keylab) tutulacak"
			d, empty = pick(false)
			if empty {
				return d, nil
			}
			d.note = note
			if d.created {
				if err2 := ensureHomeDirs(d.servers, d.components); err2 != nil {
					return d, err2
				}
			}
		}
	}
	return d, nil
}
