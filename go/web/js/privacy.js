/* KeyLab — gizlilik kilidi: hareketsizlikte (ya da L tuşu / düğmeyle) ekranı opak bir perdeyle örter.
   DÜRÜST ETİKET: bu bir PAROLA kilidi DEĞİL; yalnızca yan masadakinin ekranı görmesini engelleyen bir gizlilik perdesidir.
   Kilit açmak için parola sorulmaz. Gerçek erişim koruması oturum jetonu + 127.0.0.1 sınırıdır (bkz. Ayarlar > Güvenlik).
   pocketui'deki js/core/security.js'ten uyarlandı. Farklar: açık pencereler KAPATILMAZ (iş kaybı olmasın) — perde üstlerini örter ve
   arkadaki her şey `inert` yapılır (Tab/ekran okuyucu içeri giremez); kilitliyken tuşlar yutulur; varsayılan KAPALI; boşta sayacı
   yalnızca gerçek kullanıcı girdisiyle (fare/klavye/dokunma) sıfırlanır, arka plan işleri ya da terminal çıktısı saymaz. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const KEY = "keylab.idle", OPTS = ["0", "5", "10", "30"];
  let mins = "0", timer = 0, locked = false, lastReset = 0, from = null;

  try { const v = localStorage.getItem(KEY); if (OPTS.includes(v)) mins = v; } catch (e) { /* depolama yoksa varsayılan */ }

  const setInert = on => {
    for (const el of Array.from(document.body.children)) {
      if (el.id === "privacy-lock" || el.tagName === "SCRIPT") continue;
      if (on) el.setAttribute("inert", ""); else el.removeAttribute("inert");
    }
  };

  function lock(reason) {
    const el = $("#privacy-lock"); if (!el || locked) return;
    locked = true; clearTimeout(timer);
    from = document.activeElement;
    const n = parseInt(mins, 10);
    $("#privacy-lock-sub").textContent = reason === "idle"
      ? `${n} dk işlem yapılmadığı için ekran gizlendi. Açık pencereler ve çalışan işler olduğu gibi duruyor.`
      : "Ekran gizlendi. Açık pencereler ve çalışan işler olduğu gibi duruyor.";
    el.hidden = false; setInert(true);
    $("#privacy-unlock").focus();
  }

  function unlock() {
    const el = $("#privacy-lock"); if (!el || !locked) return;
    locked = false; el.hidden = true; setInert(false);
    if (from && from.isConnected && from.focus) from.focus();
    from = null; arm();
  }

  function arm() {
    clearTimeout(timer);
    const n = parseInt(mins, 10);
    if (locked || !n) return;
    timer = setTimeout(() => lock("idle"), n * 60000);
  }

  function paint() { $$("#set-idle button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.v === mins))); }

  function init() {
    $$("#set-idle button").forEach(b => b.addEventListener("click", () => {
      mins = b.dataset.v; try { localStorage.setItem(KEY, mins); } catch (e) { /* yalnız bu oturum */ }
      paint(); arm();
    }));
    $("#lock-btn").addEventListener("click", () => lock("manual"));
    $("#set-lock-now").addEventListener("click", () => lock("manual"));
    $("#privacy-unlock").addEventListener("click", unlock);

    // yalnız gerçek kullanıcı girdisi sayacı sıfırlar; mousemove sık geldiği için en çok saniyede bir
    ["mousemove", "mousedown", "keydown", "touchstart", "wheel"].forEach(ev => window.addEventListener(ev, () => {
      if (locked) return; const now = Date.now(); if (now - lastReset < 1000) return; lastReset = now; arm();
    }, { passive: true, capture: true }));

    window.addEventListener("keydown", e => {
      if (locked) { // kilitliyken yalnız Enter/Boşluk (kilidi açan düğme) geçer; kısayollar (?, /, Esc…) yutulur
        if (e.key === "Enter" || e.key === " ") return;
        e.preventDefault(); e.stopImmediatePropagation(); return;
      }
      if ((e.key === "l" || e.key === "L") && !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
        const el = document.activeElement, tag = (el && el.tagName) || "";
        if (/input|textarea|select/i.test(tag) || (el && el.isContentEditable)) return;
        if (el && el.closest && el.closest(".xterm")) return; // terminale yazılan "l" kilit açmasın
        if (document.querySelector(".modal-back.open")) return; // pencere açıkken yanlışlıkla kilitlenmesin
        e.preventDefault(); lock("manual");
      }
    }, true);

    paint(); arm();
  }

  window.KeyLabPrivacy = { init, lock, unlock, get locked() { return locked; } };
})();
