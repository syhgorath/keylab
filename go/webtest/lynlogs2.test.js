"use strict";
/* Lynis/Günlükler ek testler (refaktör 5): mutasyon turunda kaçan davranışları sabitler — ön süzgeç tek kullanımlık, arama kırpma,
   en yeni ölçüm, silinen sunucu, grafik sınırları, hata satırı deseni, bilinmeyen OS simgesi. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const L = (server, pkg, file) => ({ server, package: pkg, file, size: 2048, mtime: "2026-11-10T10:00:00Z" });
const LOGS = [L("web1", "utility", "last-run.log"), L("web1", "linux-hardening", "20261101-audit.log"), L("db1", "utility", "20261102-durum.log")];
const logApi = text => ({ "/api/home": mixedHome, "/api/logs": LOGS, "/api/log": text });
const rows = h => h.$$("#loglist button[data-i]");

const FIND = [
  { id: "SSH-7408", kind: "uyari", text: "Zayıf SSH ayarı", value: "x=1" },
  { id: "KRNL-6000", kind: "oneri", text: "Kernel parametresi", value: "net.x=1" },
  { id: "AUTH-9286", kind: "oneri", text: "Parola yaşı", value: "" },
];
const LY = { date: "2026-11-10", dates: ["2026-11-01", "2026-11-05", "2026-11-10"], history: [61, 68, 74], score: 74, suggestions: 12, warnings: 1 };
const homeLyn = (extra = {}) => () => { const h = mixedHome(); h.servers.find(s => s.name === "web1").lynis = LY; h.servers.find(s => s.name === "db1").lynis = { ...LY, date: "2026-10-01" }; Object.assign(h, extra); return h; };
const lynApi = (o = {}) => ({ "/api/home": homeLyn(), "/api/lynis": u => ({ server: u.searchParams.get("server"), report: "", lynis: LY, findings: FIND }), ...o });
const srv = u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.lynis.warnings = 2; return d; };
const sel = h => h.$$("#lynis-pick [aria-selected='true']").map(b => b.dataset.n);
const pressed = (h, k) => h.$(`#lynis-detail [data-k="${k}"]`).getAttribute("aria-pressed");

test("lynis: ön süzgeç tek kullanımlıktır; arama baştaki/sondaki boşluğu keser", async () => {
  const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: lynApi({ "/api/server": srv }) });
  await h.click("#srv-body [data-go-lynis-p]"); assert.equal(pressed(h, "uyari"), "true");
  await h.click('#lynis-pick [data-n="db1"]'); assert.equal(pressed(h, "all"), "true", "ön süzgeç bir kez uygulanır");
  await h.click('#lynis-pick [data-n="web1"]'); assert.equal(pressed(h, "all"), "true");
  await h.type("#lyn-q", "kernel  "); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu");
  assert.deepEqual(h.errors, []); h.close();
});

test("lynis: en yeni ölçümlü sunucu seçili açılır; seçili sunucu listeden kalkarsa seçim düşer", async () => {
  // web1 (11-10) db1'den (10-01) yeni; sunucu sırasında db1 önce gelse de web1 seçilmeli
  let h = await boot({ hash: "#v=lynis", overrides: lynApi() });
  assert.deepEqual(sel(h), ["web1"]); h.close();
  let gone = false;
  h = await boot({ hash: "#v=lynis", overrides: lynApi({ "/api/home": () => { const x = homeLyn()(); if (gone) x.servers = x.servers.filter(s => s.name !== "vm1"); return x; } }) });
  await h.click('#lynis-pick [data-n="vm1"]'); assert.deepEqual(sel(h), ["vm1"]);
  gone = true; await h.click("#refresh-all"); await h.click('[data-nav="dash"]'); await h.click('[data-nav="lynis"]');
  assert.equal(h.$("#lynis-pick [data-n='vm1']"), null); assert.deepEqual(sel(h), ["web1"]); assert.deepEqual(h.errors, []); h.close();
});

const mk = (hist, dates) => lynApi({ "/api/lynis": { server: "web1", report: "", findings: [], lynis: { ...LY, dates, history: hist, score: hist[hist.length - 1] } } });

test("lynis grafik: sıfır değişim '▲ +0'; tek nokta ortalanır; değerler 0–100'e kısılır; genişlik 1000 ile sınırlı", async () => {
  let h = await boot({ hash: "#v=lynis", overrides: mk([50, 50], ["2026-11-01", "2026-11-02"]) }); await h.click('#lynis-pick [data-n="web1"]');
  assert.equal(h.$(".lynis-delta").textContent, "▲ +0"); h.close();
  h = await boot({ hash: "#v=lynis", overrides: mk([40], ["2026-11-01"]) }); await h.click('#lynis-pick [data-n="web1"]');
  assert.equal(h.$(".chartbox circle.pt").getAttribute("cx"), "325"); h.close(); // (30 + 630 − 10) / 2
  h = await boot({ hash: "#v=lynis", overrides: mk([150, -20], ["2026-11-01", "2026-11-02"]) }); await h.click('#lynis-pick [data-n="web1"]');
  assert.deepEqual(h.$$(".chartbox circle.pt").map(c => c.getAttribute("cy")), ["10", "168"]); h.close(); // T=10, H−B=168
  h = await boot({ hash: "#v=lynis", overrides: mk([10, 20], ["2026-11-01", "2026-11-02"]) });
  Object.defineProperty(h.win.HTMLElement.prototype, "clientWidth", { get() { return this.classList.contains("main") ? 5000 : 0; }, configurable: true });
  await h.click('#lynis-pick [data-n="web1"]'); assert.equal(h.$(".chartbox svg").getAttribute("viewBox"), "0 0 1000 190"); h.close();
});

test("günlükler: 'süzgeçleri temizle' türü de sıfırlar; hata satırı deseni", async () => {
  let h = await boot({ hash: "#v=logs", overrides: logApi("x") });
  await h.click('#logfilters [data-kind="arch"]'); assert.equal(rows(h).length, 2); await h.click("#logclear");
  assert.equal(h.$('#logfilters [data-kind="all"]').getAttribute("aria-pressed"), "true"); assert.equal(rows(h).length, 3); h.close();
  h = await boot({ hash: "#v=logs", overrides: logApi("ok: x\nfatal: y\nFAILED! z\nfailed=2\nfailed=0\nERROR q\nHATA w\nnormal") });
  await h.click(rows(h)[0]);
  assert.deepEqual(h.$$("#logview div").map(d => d.classList.contains("err")), [false, true, true, true, false, true, true, false]); h.close();
});

test("simgeler: bilinmeyen işletim sistemi 'linux' simgesine düşer", async () => {
  const h = await boot({ hash: "#v=servers", overrides: { "/api/home": () => { const x = mixedHome(); x.servers[0].os = "alpine"; return x; } } });
  const html = h.$("#view-servers").innerHTML;
  assert.doesNotMatch(html, /undefined/); assert.match(html, /var\(--surface-3\)/, "linux simgesi"); h.close();
});
