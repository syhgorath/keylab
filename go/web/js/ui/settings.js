/* KeyLab — Tema ve Ayarlar sayfası: tema düğmesi, görünüm (simge/liste), parola girişi tercihi, eşzamanlı iş sınırı, yollar/yapılandırma.
   app.js'ten çıkarıldı (refaktör 5e), kod BİREBİR. Testler: go/webtest/ayarlar.test.js */
import { $, $$, esc, json } from "../lib/util.js";
import { appState } from "../state.js";

// hooks: { render() — sunucu listesini yeniden çiz }  →  { load(), paintAsk() }
export function initSettings(hooks) {
  const { render } = hooks;
  const THEMES = ["auto", "light", "dark"], THEME_TXT = { auto: "otomatik", light: "açık", dark: "koyu" }, THEME_ICO = { auto: "◐", light: "☀", dark: "☾" };
  let theme = "dark"; // varsayılan koyu (Cam + Aurora en iyi koyuda görünür); kullanıcı "Otomatik"/"Açık"ı seçtiyse o saklanır
  try { theme = THEMES.includes(localStorage.getItem("sshpocket.theme")) ? localStorage.getItem("sshpocket.theme") : "dark"; } catch (e) { /* depolama kapalı olabilir */ }
  function applyTheme() {
    if (theme === "auto") document.documentElement.removeAttribute("data-theme"); else document.documentElement.setAttribute("data-theme", theme);
    const b = $("#theme-btn"); b.setAttribute("aria-label", "Tema: " + THEME_TXT[theme]); b.title = "Tema: " + THEME_TXT[theme] + " (tıklayınca değişir)";
    $("#theme-ico").textContent = THEME_ICO[theme];
    $$("#set-theme button").forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === theme)));
  }
  function setTheme(v) { theme = v; try { localStorage.setItem("sshpocket.theme", v); } catch (e) { /* önemli değil */ } applyTheme(); }
  $("#theme-btn").addEventListener("click", () => setTheme(THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length]));
  $$("#set-theme button").forEach(b => b.addEventListener("click", () => setTheme(b.dataset.v)));
  $$("#set-mode button").forEach(b => b.addEventListener("click", () => { appState.mode = b.dataset.v; try { localStorage.setItem("sshpocket.mode", appState.mode); } catch (e) { /* önemli değil */ } render(); paintMode(); }));
  const paintMode = () => $$("#set-mode button").forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === appState.mode)));
  applyTheme();
  const paintAsk = () => $$("#set-ask button").forEach(x => { x.setAttribute("aria-pressed", String(x.dataset.v === appState.askMode)); if (x.dataset.v === "system") { x.disabled = !appState.askAvail; x.title = appState.askAvail ? "" : "Bu platformda sistem penceresi yok"; } });
  /* Eylemler > aynı anda en çok (sunucu tarafı ayar: /api/ui-settings) */
  const maxEl = $("#set-maxjobs"), maxSt = $("#set-maxjobs-st");
  let maxShown = 4;
  async function loadMaxJobs() { try { const r = await json("/api/ui-settings"); maxShown = r.maxJobs; maxEl.value = r.maxJobs; maxEl.min = r.min; maxEl.max = r.max; } catch (e) { maxSt.textContent = "okunamadı: " + e.message; } }
  maxEl.addEventListener("change", async () => {
    const n = parseInt(maxEl.value, 10), lo = parseInt(maxEl.min, 10), hi = parseInt(maxEl.max, 10);
    if (!(n >= lo && n <= hi)) { maxEl.value = maxShown; maxSt.textContent = `${lo} ile ${hi} arasında olmalı`; return; }
    try { const r = await json("/api/ui-settings", { maxJobs: n }); maxShown = r.maxJobs; maxEl.value = r.maxJobs; maxSt.textContent = "kaydedildi"; }
    catch (e) { maxEl.value = maxShown; maxSt.textContent = "kaydedilemedi: " + e.message; }
  });
  loadMaxJobs();
  $$("#set-ask button").forEach(b => b.addEventListener("click", () => { appState.askMode = b.dataset.v; try { localStorage.setItem("sshpocket.askpass", appState.askMode); } catch (e) { /* önemli değil */ } paintAsk(); }));

  async function loadSettings() {
    paintMode(); applyTheme(); paintAsk();
    let a;
    try { a = await json("/api/about"); appState.askAvail = !!a.askpass; paintAsk(); } catch (e) { $("#set-paths").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    const row = (k, v) => `<div class="urow sj-8657c7"><div class="muted">${esc(k)}</div><div class="mono small sj-752397">${esc(v) || "—"}</div></div>`;
    $("#set-paths").innerHTML = [["KeyLab sürümü", a.version], ["Kök klasör", a.root], ["Sunucular", a.serversDir], ["Paketler", a.componentsDir], ["Durum ve günlükler", a.stateDir], ["Platform", a.platform], ["Go", a.go]].map(([k, v]) => row(k, v)).join("");
    const conf = Object.entries(a.conf || {});
    $("#set-conf").innerHTML = conf.length ? conf.map(([k, v]) => row(k, v)).join("") : '<div class="small muted sj-fdf20b">sshpocket.conf bulunamadı ya da boş.</div>';
  }

  return { load: loadSettings, paintAsk };
}
