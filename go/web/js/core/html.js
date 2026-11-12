/* KeyLab — `html` etiketli şablon: değerler VARSAYILAN olarak kaçışlanır (XSS'e karşı güvenli varsayılan).
   YALNIZ YENİ KOD içindir; eski şablon dizeleri (esc() ile elle kaçışlı) olduğu gibi kalır.

     el.innerHTML = html`<li title="${ad}">${ad} ${raw(guvenilirIsaret)}</li>`;
     el.innerHTML = html`<ul>${liste.map(x => html`<li>${x}</li>`)}</ul>`;   // dizi: her öğe kaçışlanır, html`` sonuçları olduğu gibi

   Kurallar:
   - Şablonun SABİT parçaları yazarın kodudur, güvenilir sayılır. ${…} değerleri kaçışlanır (& < > " ').
   - html`` sonucu (SafeHtml) iç içe kullanılırsa yeniden kaçışlanmaz. Dizi her öğesiyle aynı kuralı izler.
   - raw(metin) kaçışı bilerek kapatır: YALNIZ kendi ürettiğin sabit işaretleme için; kullanıcı/sunucu/paket verisiyle ASLA.
   - null, undefined ve false boş metin olur (koşullu parça: ${x && html`…`}).
   - Öznitelik değerlerini HER ZAMAN çift tırnakla yaz: ${…} tırnaksız özniteliğe konursa kaçış yetmez.
   - setHtml(el, güvenli) yalnız SafeHtml kabul eder; düz dize verilirse hata atar (yanlışlıkla kaçışsız yazmayı engeller). */
(function (root) {
  "use strict";
  const MAP = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => MAP[c]);
  class SafeHtml {
    constructor(s) { this.s = s; Object.freeze(this); }
    toString() { return this.s; }
  }
  const isSafe = v => v instanceof SafeHtml;
  const raw = s => new SafeHtml(String(s ?? ""));
  const part = v => {
    if (v == null || v === false) return "";
    if (isSafe(v)) return v.s;
    if (Array.isArray(v)) return v.map(part).join("");
    return esc(v);
  };
  function html(strings, ...vals) {
    let out = strings[0];
    for (let i = 0; i < vals.length; i++) out += part(vals[i]) + strings[i + 1];
    return new SafeHtml(out);
  }
  function setHtml(el, safe) {
    if (!isSafe(safe)) throw new TypeError("setHtml: html`…` ya da raw() sonucu gerekir");
    el.innerHTML = safe.s;
  }
  const api = { html, raw, esc, isSafe, setHtml };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.KeyLabHtml = api;
})(typeof window !== "undefined" ? window : globalThis);
