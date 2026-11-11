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
    },
    en: {
      nav_general: "General", nav_security: "Security", nav_system: "System",
      nav_dash: "Dashboard", nav_servers: "Servers", nav_packages: "Packages",
      nav_lynis: "Lynis audit", nav_logs: "Logs", nav_settings: "Settings",
      search_ph: "Search servers, packages…",
      tree_title: "Servers", tree_add: "+ Add", tree_search: "Search server or IP…", tree_none: "No matching server.",
      lang_title: "Dil / Language",
    },
  };

  const getLang = () => currentLang;

  function t(key, fallback) {
    const d = DICTS[currentLang];
    if (d && d[key] != null) return d[key];
    if (DICTS.tr[key] != null) return DICTS.tr[key];
    return fallback != null ? fallback : key;
  }

  function applyI18n(root) {
    const base = root || document;
    const each = (attr, fn) => base.querySelectorAll("[" + attr + "]").forEach(el => fn(el, t(el.getAttribute(attr), null)));
    each("data-i18n", (el, v) => { if (v != null) el.textContent = v; });
    each("data-i18n-title", (el, v) => { if (v != null) el.title = v; });
    each("data-i18n-placeholder", (el, v) => { if (v != null) el.placeholder = v; });
    each("data-i18n-aria", (el, v) => { if (v != null) el.setAttribute("aria-label", v); });
    const badge = document.getElementById("lang-cur");
    if (badge) badge.textContent = currentLang.toUpperCase();
  }

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

  window.KeyLabI18n = { getLang, setLang, t, applyI18n, SUPPORTED_LANGS, DICTS };
  init();
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => applyI18n(document));
})();
