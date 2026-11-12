/* KeyLab — pencerelerde (.modal-back) odak yönetimi: Tab pencere içinde döner, arkadaki sayfa `inert` olur (Tab/ekran okuyucu içeri giremez),
   pencere kapanınca odak onu açan öğeye döner. Her pencerenin kendi kodunu değiştirmez: `open` sınıfını izler (MutationObserver).
   Bilinçli istisnalar: terminal (.xterm) odaktayken Tab terminale aittir (kabuk tamamlama) ve tuzağa alınmaz; komut paleti Tab'ı kendisi
   yönetir; gizlilik kilidi açıkken (tüm sayfa zaten inert) bu modül dokunmaz, kilit açılınca privacy.js sync()'i çağırır. */
(function () {
  "use strict";
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';
  const locked = () => !!(window.KeyLabPrivacy && window.KeyLabPrivacy.locked);
  const visible = el => !el.closest("[hidden]") && !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
  const focusables = m => $$(FOCUSABLE, m).filter(visible);
  const openers = new Map(); // pencere -> onu açmadan önce odakta olan öğe
  const was = new Map();     // pencere -> önceki "açık" durumu
  let stack = [], lastOutside = null;

  document.addEventListener("focusin", e => { if (!e.target.closest(".modal-back.open")) lastOutside = e.target; }, true);
  // odak olayı gelmeyen durumlar için (pencere odakta değil, programla odaklama): tıklanan/etkinleştirilen öğeyi de "açan öğe" say
  document.addEventListener("click", e => { const t = e.target.closest && e.target.closest(FOCUSABLE); if (t && !t.closest(".modal-back.open")) lastOutside = t; }, true);

  function sync() {
    if (locked()) return;
    const open = $$(".modal-back.open");
    for (const el of Array.from(document.body.children)) {
      if (el.tagName === "SCRIPT" || el.id === "toast" || el.id === "privacy-lock") continue;
      if (open.length && !open.includes(el)) el.setAttribute("inert", ""); else el.removeAttribute("inert");
    }
  }

  function onChange(m) {
    const now = m.classList.contains("open");
    if (now === !!was.get(m)) return;
    was.set(m, now);
    if (now) {
      openers.set(m, lastOutside && lastOutside.isConnected ? lastOutside : document.activeElement);
      stack = stack.filter(x => x !== m); stack.push(m);
      sync();
      setTimeout(() => { // pencere kendi odağını vermediyse ilk etkileşimli öğeye ver
        if (!m.classList.contains("open") || m.contains(document.activeElement)) return;
        const f = focusables(m)[0]; if (f) f.focus({ preventScroll: true });
      }, 80);
    } else {
      stack = stack.filter(x => x !== m);
      sync();
      const o = openers.get(m); openers.delete(m);
      const a = document.activeElement;
      if (o && o.isConnected && !locked() && (!a || a === document.body || m.contains(a))) { try { o.focus({ preventScroll: true }); } catch (e) { /* odaklanamazsa kalsın */ } }
    }
  }

  document.addEventListener("keydown", e => {
    const cur = document.activeElement; // klavyeyle açılan pencerelerde (⌘K, kısayol) açan öğeyi pencere odağı almadan ÖNCE yakala
    if (cur && cur !== document.body && !cur.closest(".modal-back.open")) lastOutside = cur;
    if (e.key !== "Tab" || locked()) return;
    const top = stack[stack.length - 1];
    if (!top || !top.classList.contains("open") || top.id === "palette") return;
    const a = document.activeElement;
    if (a && a.closest && a.closest(".xterm")) return; // terminalde Tab kabuğa aittir
    const f = focusables(top);
    if (!f.length) { e.preventDefault(); return; }
    const first = f[0], last = f[f.length - 1];
    if (!top.contains(a)) { e.preventDefault(); first.focus(); }
    else if (e.shiftKey && a === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && a === last) { e.preventDefault(); first.focus(); }
  }, true);

  function init() {
    const obs = new MutationObserver(recs => { for (const r of recs) onChange(r.target); });
    for (const m of $$(".modal-back")) { was.set(m, m.classList.contains("open")); obs.observe(m, { attributes: true, attributeFilter: ["class"] }); }
  }

  window.KeyLabModalTrap = { init, sync };
  init();
})();
