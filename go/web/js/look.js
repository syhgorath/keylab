/* KeyLab — yüzey görünümü seçimi (Ayarlar > Görünüm > Yüzeyler): "Düz" (varsayılan) ya da "Cam + Aurora" (yarı saydam yüzeyler,
   bulanık arka plan, zeminde yumuşak renk geçişi). Yalnız <html data-look="glass"> özniteliği değişir; tüm stil tasarim.css sonundaki
   :root[data-look="glass"] bloğundadır (opak yedekler: backdrop-filter desteği yoksa, prefers-reduced-transparency ve forced-colors).
   Seçim tarayıcıda saklanır ("keylab.look"); depolama yoksa yalnız o oturum için geçerlidir. Betik <head>'de yüklenir ki ilk çizimde
   titreme olmasın; düğmeler DOM hazır olunca bağlanır. */
(function () {
  "use strict";
  const KEY = "keylab.look", LOOKS = ["flat", "glass"];
  let cur = "flat";
  try { const v = localStorage.getItem(KEY); if (LOOKS.includes(v)) cur = v; } catch (e) { /* depolama yok */ }

  function apply() {
    if (cur === "glass") document.documentElement.setAttribute("data-look", "glass"); else document.documentElement.removeAttribute("data-look");
    document.querySelectorAll("#set-look button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.v === cur)));
  }

  function init() {
    document.querySelectorAll("#set-look button").forEach(b => b.addEventListener("click", () => {
      cur = LOOKS.includes(b.dataset.v) ? b.dataset.v : "flat";
      try { localStorage.setItem(KEY, cur); } catch (e) { /* yalnız bu oturum */ }
      apply();
    }));
    apply();
  }

  window.KeyLabLook = { init, get: () => cur };
  apply(); // <head>'de: öznitelik ilk çizimden önce konur
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init); else init();
})();
