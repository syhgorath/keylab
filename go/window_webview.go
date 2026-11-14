//go:build webview

package main

// DENEME: uygulamanın kendi penceresi. Go sürecinin içinde gömülü webview (macOS'ta WKWebView) yerel adresi (tek kullanımlık açılış jetonuyla)
// gösterir; pencere kapanınca runWindow döner ve süreç biter. Tarayıcı, eklenti ve ayrı profil gerekmez. Derleme: CGO + `-tags webview`
// (go/scripts/keylabtest.sh --pencere). Üçüncü taraf bağımlılık: github.com/webview/webview_go (MIT); varsayılan derlemeye girmez.

import (
	webview "github.com/webview/webview_go"
)

func windowSupported() bool { return true }

func runWindow(url, title string) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle(title)
	w.SetSize(1280, 820, webview.HintNone)
	installNativeMenu(title) // macOS: Kopyala/Yapıştır/Tümünü seç ve Çık kısayolları için ana menü (webview kendisi menü kurmaz)
	w.Navigate(url)
	w.Run()
}
