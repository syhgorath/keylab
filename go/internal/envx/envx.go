// Package envx: kullanıcıya açık ortam değişkenleri. Yeni ad KEYLAB_<AD>, eski ad SSHPOCKET_<AD> geriye uyumlu yedek olarak okunur
// (yeni ad doluysa o kazanır). Paket sözleşmesi değişkenleri (SSHPOCKET_SERVER/ACTION/PROFILE/VAR_*, PROGRESS/RESULT/STATUS) buraya GİRMEZ.
package envx

import "os"

// Get: KEYLAB_<ad>, boşsa SSHPOCKET_<ad>.
func Get(ad string) string {
	if v := os.Getenv("KEYLAB_" + ad); v != "" {
		return v
	}
	return os.Getenv("SSHPOCKET_" + ad)
}

// PickDir: durum/günlük klasörü adı için seçim: yeni klasör varsa o, yoksa eskisi varsa eskisi (veri taşınmaz), ikisi de yoksa yeni.
func PickDir(yeni, eski string) string {
	if st, err := os.Stat(yeni); err == nil && st.IsDir() {
		return yeni
	}
	if st, err := os.Stat(eski); err == nil && st.IsDir() {
		return eski
	}
	return yeni
}
