package main

import (
	"errors"
	"io"
	"keylab/internal/platform"
	"keylab/internal/vault"
)

// vaultSecrets: platform.Secrets arayüzünü şifreli kasayla gerçekler. Misafir (USB) modda Keychain yoktur; knock dizisi gibi sırlar (KnockAccount)
// kasada tutulur ve mevcut knock kodu (cli.Servers, servers.PreConnect) değişmeden kasayı "gizli değer deposu" olarak kullanır. Yalnız KASA AÇIKKEN çalışır:
// kilitliyken Find/Has bulamaz, Set/Delete hata verir. Yalnız uygulama sürecinde kullanılır (alt süreçler kasaya erişemez; knock zaten süreç içinde gönderilir).
type vaultSecrets struct{ m *vaultMgr }

var errVaultNoInteractive = errors.New("kasa etkileşimli giriş desteklemez")

func (s vaultSecrets) Find(account string) (string, bool) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if s.m.v == nil {
		return "", false
	}
	return s.m.v.Get(account)
}

func (s vaultSecrets) Has(account string) bool { _, ok := s.Find(account); return ok }

func (s vaultSecrets) Delete(account string) bool {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if s.m.v == nil {
		return false
	}
	ok, err := s.m.v.Delete(account)
	return ok && err == nil
}

func (s vaultSecrets) Set(account, value string) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if s.m.v == nil {
		return vault.ErrLocked
	}
	s.m.lastTouch = s.m.now()
	return s.m.v.Set(account, value)
}

func (s vaultSecrets) SetInteractive(string, io.Reader, io.Writer, io.Writer) error {
	return errVaultNoInteractive
}

func (s vaultSecrets) Available() bool {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	return s.m.v != nil
}

var _ platform.Secrets = vaultSecrets{}

// knockSecrets: knock dizisinin saklandığı depo: misafir modda şifreli kasa, değilse Keychain.
func (a *App) knockSecrets() platform.Secrets {
	if platform.GuestMode() {
		return vaultSecrets{a.vaultM()}
	}
	return platform.Default()
}

// knockErr: misafir modda "Keychain'de yok" yerine gerçek nedeni söyler (kasa yok / dizi kasada yok).
func (a *App) knockErr(err error) error {
	if err == nil || !platform.GuestMode() {
		return err
	}
	if !vault.Exists(a.vaultM().path) {
		return errors.New("misafir (USB) modda knock dizisi şifreli kasada tutulur: önce Ayarlar > Şifreli passphrase kasası'ndan kasayı oluşturun")
	}
	return errors.New("knock dizisi şifreli kasada yok; sunucu sayfasındaki Port knocking ayarından kaydedin")
}
