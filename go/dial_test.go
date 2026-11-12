package main

import (
	"net"
	"testing"
	"time"
)

func TestDialState(t *testing.T) {
	// up: dinleyen bir port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("yerel dinleme kurulamadı:", err)
	}
	addr := ln.Addr().String()
	if got := dialState(addr, time.Second); got != "up" {
		t.Errorf("dinleyen port: %q, beklenen up", got)
	}
	// refused: az önce kapatılan port, makine canlı ama dinleyen yok (RST)
	ln.Close()
	if got := dialState(addr, time.Second); got != "refused" {
		t.Errorf("kapalı yerel port: %q, beklenen refused", got)
	}
	// down: yönlendirilmeyen belge adresi (zaman aşımı ya da ulaşılamıyor); asla "up" ya da "refused" olmamalı
	got := dialState("192.0.2.1:9", 200*time.Millisecond)
	if got != "down" {
		t.Errorf("ulaşılamayan adres: %q, beklenen down", got)
	}
}
