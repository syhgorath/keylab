/* KeyLab — arayüzün PAYLAŞILAN durumu. Birden çok bölümün okuduğu/yazdığı dokuz alan tek nesnede toplanır; modüller bunu doğrudan
   `import { appState }` ile okur (eskiden her modüle `getHome` gibi geri çağrılar verilirdi). Bir ES modülünden dışarı aktarılan
   `let` değişkenine içeriden atama yapılamaz; bu yüzden değişebilen durum bir nesnenin ÖZELLİĞİ olarak tutulur.
   Yalnız birden çok bölümü ilgilendiren durum buradadır; tek bir bölümün kendi durumu (ör. Paketler sayfasının süzgeçleri) o bölümde kalır. */
export const appState = {
  home: { servers: [], packages: [], indexSerial: "" }, // /api/home yanıtı
  selected: "",       // Lynis panelinde gösterilen sunucu
  query: "",          // üst çubuk arama kutusu
  view: "dash",       // etkin sayfa
  routing: false,     // geçmişten (geri/ileri/elle hash) ya da açılışta uygulanıyor: yeni geçmiş kaydı açma
  mode: "tiles",      // sunucu görünümü: tiles | list
  srv: null,          // /api/server yanıtı (açık sunucu sayfası)
  srvName: "",        // açık sunucunun adı
  srvTab: "",         // açık sunucu sayfasındaki sekme
};
