package platform

import "io"

// Volumes: çıkarılabilir birimler (USB bellek). Çıkarma, uygulama o birimin kendisinden çalışıyorsa uygulama kapandıktan SONRA yapılır
// (disk sürecin altından çekilmez; bkz. internal/eject/defer.go).
type Volumes interface {
	// MountRoot: yol çıkarılabilir bir birimin altındaysa birimin kökü (ör. macOS'ta "/Volumes/<ad>"), değilse "".
	MountRoot(path string) string
	// Eject: birimdeki anahtarları ajandan siler ve birimi çıkarır; iletiler w'ye yazılır. 0: çıkarıldı/ertelendi/yapılacak yok, 1: çıkarılamadı.
	Eject(w io.Writer, root string) int
	// WillDefer: çalışan ikili bu birimden mi çalışıyor (Eject çıkarmayı süreç bittikten sonraya erteler)?
	WillDefer(root string) bool
}

// DefaultVolumes: bu işletim sisteminin birim yöneticisi. macOS dışında şimdilik aynı `diskutil` yolu (araç yoksa bir şey yapmaz); Linux/Windows sonra.
func DefaultVolumes() Volumes { return DiskutilVolumes{} }
