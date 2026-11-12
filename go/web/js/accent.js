/* KeyLab — vurgu rengi seçimi (Ayarlar > Vurgu rengi). Yalnız vurgu jetonları değişir (bkz. tasarim.css sonundaki
   :root[data-accent="…"] kuralları); yüzey ve metin renkleri, açık/koyu tema ve WCAG AA kontrastı korunur. Seçim tarayıcıda
   saklanır ("keylab.accent"); depolama yoksa yalnız o oturum için geçerlidir. Varsayılan (indigo) için öznitelik konmaz. */
(function () {
  "use strict";
  const KEY = "keylab.accent";
  // [kimlik, ad, önizleme rengi]; renkler tasarim.css'tekilerle aynı (accent_gen ile üretilip kontrast doğrulandı)
  const ACCENTS = [
    ["default", "İndigo", "#4f46e5"],
    ["blue", "Mavi", "#1d4ed8"],
    ["teal", "Turkuaz", "#0f766e"],
    ["emerald", "Zümrüt", "#047857"],
    ["amber", "Kehribar", "#b45309"],
    ["rose", "Gül", "#be123c"],
    ["violet", "Mor", "#7c3aed"],
    ["graphite", "Grafit", "#334155"],
  ];
  const ids = ACCENTS.map(a => a[0]);
  let cur = "default";
  try { const v = localStorage.getItem(KEY); if (ids.includes(v)) cur = v; } catch (e) { /* depolama yok */ }

  function apply() {
    if (cur === "default") document.documentElement.removeAttribute("data-accent"); else document.documentElement.setAttribute("data-accent", cur);
    document.querySelectorAll("#set-accent .accent-sw").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.a === cur)));
  }

  function init() {
    const box = document.querySelector("#set-accent"); if (!box) return;
    for (const [id, name, color] of ACCENTS) {
      const b = document.createElement("button");
      b.type = "button"; b.className = "accent-sw"; b.dataset.a = id; b.title = name; b.setAttribute("aria-label", name);
      b.style.background = color; // önizleme rengi (veri sabitinden; kullanıcı girdisi değil)
      b.addEventListener("click", () => { cur = id; try { localStorage.setItem(KEY, cur); } catch (e) { /* yalnız bu oturum */ } apply(); });
      box.appendChild(b);
    }
    apply();
  }

  window.KeyLabAccent = { init, ACCENTS };
  init(); // betik gövdenin sonunda yüklenir; DOM hazır
})();
