/* KeyLab çeviri motoru (TR/EN). pocketui'deki js/core/i18n.js'ten esinlenildi; farklar: metinler textContent ile yazılır
   (HTML enjeksiyonu olmaz) ve saklama anahtarı "keylab.lang". Eksik anahtar Türkçeye, o da yoksa t() çağrısındaki varsayılana düşer;
   böylece çeviri parça parça eklenebilir. Yeni arayüz metinleri: HTML'de data-i18n="anahtar" (metin), data-i18n-placeholder,
   data-i18n-title, data-i18n-aria; JS'te t("anahtar", "Türkçe varsayılan"). */
(function () {
  "use strict";

  const STORAGE_KEY = "keylab.lang";
  const SUPPORTED_LANGS = ["tr", "en"];
  let currentLang = "tr";

  const DICTS = {
    tr: {
      nav_general: "Genel", nav_security: "Güvenlik", nav_system: "Sistem",
      nav_dash: "Pano", nav_servers: "Sunucular", nav_packages: "Paketler",
      nav_lynis: "Lynis denetimi", nav_logs: "Günlükler", nav_settings: "Ayarlar",
      search_ph: "Sunucu, paket ara…",
      tree_title: "Sunucular", tree_add: "+ Ekle", tree_search: "Sunucu veya IP ara…", tree_none: "Eşleşen sunucu yok.",
      lang_title: "Dil / Language",
      st_invalid: "yapılandırma geçersiz", st_checking: "denetleniyor", st_up: "çevrimiçi", st_knock: "🔒 kapalı — knock gerekir",
      st_closed: "port kapalı (makine yanıt veriyor)", st_down: "bağlantı yok",
      theme_light: "Açık",
      imp_selected: "seçili", imp_loading: "Taranıyor…", imp_fail: "Hata:", imp_done: "sunucu eklendi", imp_skipped: "zaten vardı (atlandı)", imp_errors: "hata",
      term_sub: "Gömülü terminal: kayıtlı sunucuya ssh ile bağlanır. Parola ve passphrase burada, terminalde sorulur.",
      term_foot: "Kapatınca oturum sonlanır. Terminal açma/kapama durum klasöründe terminal-audit.log dosyasına yazılır.",
      term_end: "Oturumu kapat", term_exited: "oturum bitti", term_lib_missing: "xterm.js yüklenemedi.",
      term_input_fail: "Girdi iletilemedi: oturum kapanmış olabilir.", term_conn_lost: "Akış koptu; oturum kapanmış olabilir.",
    },
    en: {
      nav_general: "General", nav_security: "Security", nav_system: "System",
      nav_dash: "Dashboard", nav_servers: "Servers", nav_packages: "Packages",
      nav_lynis: "Lynis audit", nav_logs: "Logs", nav_settings: "Settings",
      search_ph: "Search servers, packages…",
      tree_title: "Servers", tree_add: "+ Add", tree_search: "Search server or IP…", tree_none: "No matching server.",
      lang_title: "Dil / Language",
      st_invalid: "invalid configuration", st_checking: "checking", st_up: "online", st_knock: "🔒 closed — knock required",
      st_closed: "port closed (host is responding)", st_down: "no connection",
      theme_light: "Light",
      imp_selected: "selected", imp_loading: "Scanning…", imp_fail: "Error:", imp_done: "server(s) added", imp_skipped: "already existed (skipped)", imp_errors: "error(s)",
      term_sub: "Embedded terminal: connects to the registered server over ssh. Passwords and passphrases are asked here, in the terminal.",
      term_foot: "Closing ends the session. Opening/closing a terminal is logged to terminal-audit.log in the state folder.",
      term_end: "End session", term_exited: "session ended", term_lib_missing: "xterm.js could not be loaded.",
      term_input_fail: "Input could not be sent: the session may have closed.", term_conn_lost: "Stream lost; the session may have closed.",
    },
  };

  const getLang = () => currentLang;

  function t(key, fallback) {
    const d = DICTS[currentLang];
    if (d && d[key] != null) return d[key];
    if (DICTS.tr[key] != null) return DICTS.tr[key];
    return fallback != null ? fallback : key;
  }

  let applyI18n = function (root) {
    const base = root || document;
    const each = (attr, fn) => base.querySelectorAll("[" + attr + "]").forEach(el => fn(el, t(el.getAttribute(attr), null)));
    each("data-i18n", (el, v) => { if (v != null) el.textContent = v; });
    each("data-i18n-title", (el, v) => { if (v != null) el.title = v; });
    each("data-i18n-placeholder", (el, v) => { if (v != null) el.placeholder = v; });
    each("data-i18n-aria", (el, v) => { if (v != null) el.setAttribute("aria-label", v); });
    const badge = document.getElementById("lang-cur");
    if (badge) badge.textContent = currentLang.toUpperCase();
  };

  function setLang(lang) {
    if (!SUPPORTED_LANGS.includes(lang)) lang = "tr";
    currentLang = lang;
    try { localStorage.setItem(STORAGE_KEY, lang); } catch (_) { /* depolama kapalı olabilir */ }
    document.documentElement.lang = lang;
    applyI18n(document);
    window.dispatchEvent(new CustomEvent("keylab:langchange", { detail: { lang } }));
  }

  function init() {
    let saved = null;
    try { saved = localStorage.getItem(STORAGE_KEY); } catch (_) { /* yoksay */ }
    currentLang = SUPPORTED_LANGS.includes(saved) ? saved : "tr";
    document.documentElement.lang = currentLang;
    applyI18n(document);
  }

  /* ---- Ekranda çizilen metin için ifade çevirisi (geçiş katmanı) ----
     Çizim kodları Türkçe metni doğrudan üretir. İngilizce seçiliyken burada, tam eşleşen metin düğümleri (ve title / placeholder /
     aria-label öznitelikleri) js/core/i18n-phrases-en.js tablosuna göre değiştirilir; Türkçeye dönünce özgün metin geri konur.
     Tam eşleşme aranır: kullanıcı verisi (sunucu adı, günlük satırı) bir ifadeyle tıpatıp aynı değilse asla değişmez.
     Eşleşmeyen metin Türkçe kalır (çeviri parça parça eklenir). Kalıcı çözüm: metinleri data-i18n / t() anahtarlarına taşımak. */
  const PHRASES = window.KeyLabPhrasesEN || { exact: {}, patterns: [] };
  const tracked = new Map(); // düğüm -> özgün değer (metin düğümü için nodeValue; öznitelik için {attr: değer})
  const ATTRS = ["title", "placeholder", "aria-label"];
  const SKIP = new Set(["SCRIPT", "STYLE", "TEXTAREA", "INPUT", "PRE", "CODE"]);
  function trPhrase(raw) {
    const k = raw.replace(/\s+/g, " ").trim();
    if (!k) return null;
    if (PHRASES.exact[k] != null) return PHRASES.exact[k];
    for (const [re, rep] of PHRASES.patterns) if (re.test(k)) return k.replace(re, rep);
    return null;
  }
  let busy = false, scheduled = false;
  function translateTree() {
    if (busy) return;
    busy = true;
    try {
      if (currentLang !== "en") {
        tracked.forEach((orig, node) => {
          if (!node.isConnected) { tracked.delete(node); return; }
          if (node.nodeType === 3) node.nodeValue = orig;
          else Object.keys(orig).forEach(a => node.setAttribute(a, orig[a]));
        });
        tracked.clear();
        return;
      }
      const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
        acceptNode: n => (n.parentNode && SKIP.has(n.parentNode.nodeName)) ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT,
      });
      let n;
      while ((n = w.nextNode())) {
        if (tracked.has(n)) continue;
        const r = trPhrase(n.nodeValue);
        if (r == null) continue;
        const lead = n.nodeValue.match(/^\s*/)[0], trail = n.nodeValue.match(/\s*$/)[0];
        tracked.set(n, n.nodeValue);
        n.nodeValue = lead + r + trail;
      }
      document.body.querySelectorAll("[title],[placeholder],[aria-label]").forEach(el => {
        const orig = tracked.get(el) || {};
        let changed = false;
        ATTRS.forEach(a => {
          const v = el.getAttribute(a);
          if (v == null) return;
          const r = trPhrase(orig[a] != null ? orig[a] : v);
          if (r != null && r !== v) { orig[a] = orig[a] != null ? orig[a] : v; el.setAttribute(a, r); changed = true; }
        });
        if (changed) tracked.set(el, orig);
      });
    } finally { busy = false; }
  }
  function scheduleTranslate() {
    if (scheduled) return;
    scheduled = true;
    // setTimeout: arka plandaki sekmede requestAnimationFrame durur; kısa bir gecikme art arda çizimleri de tek turda toplar
    setTimeout(() => { scheduled = false; translateTree(); }, 20);
  }
  const origApply = applyI18n;
  applyI18n = function (root) { origApply(root); translateTree(); };

  window.KeyLabI18n = { getLang, setLang, t, applyI18n, translateTree, SUPPORTED_LANGS, DICTS };
  init();
  new MutationObserver(() => { if (!busy) scheduleTranslate(); }).observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => applyI18n(document));
})();
