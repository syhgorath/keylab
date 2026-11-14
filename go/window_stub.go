//go:build !webview

package main

// Varsayılan derleme (CGO'suz, her işletim sistemine çapraz derlenir): gömülü pencere kabuğu yok; -browser window bu derlemede reddedilir.

func windowSupported() bool { return false }

func runWindow(url, title string) {}
