// Package shellgold: kabuk referansıyla karşılaştırmalı testler için kayıt/oynatma ("altın dosya").
//
// Eskiden bu testler her koşuda referans/kabuk motorunu çalıştırırdı. Artık kabuğun çıktısı bir kez KAYDEDİLİR (testdata/*.json)
// ve testler Go sonucunu kayıtla karşılaştırır; referans/kabuk depodan silindi.
//
//	Oynatma (varsayılan, CI):  kayıttan okur; kayıt yoksa test başarısız olur (anahtar iletide).
//	Kayıt: KEYLAB_KAYIT_KABUK=1 go test ./internal/<paket> — eski motor gerekir (depodan silindi: `git checkout kabuk-referansi -- referans/kabuk`);
//	yürütmeyi çalıştırır ve dosyayı yeniden yazar.
//
// Anahtar, çağıranın ürettiği DETERMİNİSTİK metindir (girdi içeriği, argümanlar; geçici klasör yolu, rastgele değer YOK).
package shellgold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// Store: tek bir altın dosya.
type Store struct {
	path   string
	mu     sync.Mutex
	data   map[string]json.RawMessage
	record bool
	dirty  bool
	seq    map[string]int
}

// Open: testdata/<ad>.json dosyasını açar. Kayıt modunda test bitince değişiklikleri yazar.
func Open(t testing.TB, ad string) *Store {
	t.Helper()
	s := &Store{path: filepath.Join("testdata", ad+".json"), data: map[string]json.RawMessage{}, seq: map[string]int{}, record: os.Getenv("KEYLAB_KAYIT_KABUK") == "1"}
	b, err := os.ReadFile(s.path)
	if s.record {
		// kayıt modunda mevcut dosya yüklenir: aynı dosyayı paylaşan testler birbirini ezmez (eski kayıtları silmek için dosyayı silin)
		if err == nil {
			_ = json.Unmarshal(b, &s.data)
		}
	} else {
		if err != nil {
			t.Fatalf("altın dosya okunamadı (%v); kaydetmek için KEYLAB_KAYIT_KABUK=1 ile çalıştırın (eski motor gerekir: git checkout kabuk-referansi -- referans/kabuk)", err)
		}
		if err := json.Unmarshal(b, &s.data); err != nil {
			t.Fatalf("altın dosya bozuk: %v", err)
		}
	}
	t.Cleanup(func() { s.flush(t) })
	return s
}

// Do: kayıt modunda run()'ı çalıştırıp sonucu saklar; oynatmada kayıtlı sonucu döndürür.
func Do[T any](t testing.TB, s *Store, key string, run func() T) T {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record {
		v := run()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("kayıt serileştirilemedi: %v", err)
		}
		s.data[key] = b
		s.dirty = true
		s.flush(t) // kayıt modunda hemen yaz: alt testlerin Cleanup sırası ve paylaşılan paket düzeyi mağazalar için güvenli
		return v
	}
	raw, ok := s.data[key]
	if !ok {
		t.Fatalf("kayıtlı kabuk sonucu yok (anahtar %q); KEYLAB_KAYIT_KABUK=1 ile yeniden kaydedin", key)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("kayıt çözülemedi: %v", err)
	}
	return v
}

func (s *Store) flush(t testing.TB) {
	if !s.record || !s.dirty {
		return
	}
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make([]byte, 0, 1024)
	ordered = append(ordered, "{\n"...)
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		ordered = append(ordered, ' ', ' ')
		ordered = append(ordered, kb...)
		ordered = append(ordered, ": "...)
		ordered = append(ordered, s.data[k]...)
		if i < len(keys)-1 {
			ordered = append(ordered, ',')
		}
		ordered = append(ordered, '\n')
	}
	ordered = append(ordered, "}\n"...)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		t.Errorf("testdata klasörü: %v", err)
		return
	}
	if err := os.WriteFile(s.path, ordered, 0o644); err != nil {
		t.Errorf("altın dosya yazılamadı: %v", err)
	}
}

// Next: aynı testte sırayla çağrılan kabuk çalışmaları için deterministik anahtar (test adı + sıra numarası).
// Testin çağrı sırası değişirse kayıt yeniden alınmalıdır.
func (s *Store) Next(t testing.TB) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := t.Name()
	if _, ok := s.seq[name]; !ok {
		// test bitince sayaç sıfırlanır (-count=N ya da paket düzeyi paylaşılan mağaza için)
		t.Cleanup(func() { s.mu.Lock(); delete(s.seq, name); s.mu.Unlock() })
	}
	s.seq[name]++
	return fmt.Sprintf("%s#%d", name, s.seq[name])
}
