"use strict";
/* Gömülü terminal penceresi (çerçeve) testleri: trafik ışığı düğmeleri (kapat / küçült / tam ekran), durum göstergesi, oturum çipleri.
   xterm kendisi sahte (harness.fakeXterm); sunucu tarafı PTY ve EventSource sahte. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const api = (o = {}) => ({ "/api/home": mixedHome, "/api/term/open": { id: "t1" }, "/api/term/close": { ok: true }, "/api/term/resize": { ok: true }, "/api/term/input": { ok: true }, ...o });
const isOpen = h => h.$("#termmodal").classList.contains("open");
const state = h => ({ text: h.$("#term-state").textContent, cls: h.$("#term-state").className });
const posts = (h, p) => h.calls.filter(c => c.method === "POST" && c.path === p).map(c => c.body);

async function opened(o) { const h = await boot({ hash: "#s=web1", overrides: api(o) }); await h.click("#srv-iterm"); return h; }

test("terminal penceresi: pencere düğmeleri (kapat/küçült/tam ekran) erişilebilir adlarla; başlık '>_ SSH'", async () => {
  const h = await boot({ hash: "", overrides: api() });
  assert.deepEqual([...h.$$("#term-window .pty-dots button")].map(b => b.className), ["d-r", "d-y", "d-g"]);
  assert.match(h.$(".pty-dots .d-r").getAttribute("aria-label"), /Pencereyi kapat/); assert.match(h.$(".pty-dots .d-y").getAttribute("aria-label"), /küçült/); assert.match(h.$(".pty-dots .d-g").getAttribute("aria-label"), /Tam ekran/);
  assert.equal(h.$(".pty-dots .d-g").getAttribute("aria-pressed"), "false"); assert.equal(h.$("#term-title").textContent.trim(), ">_ SSH"); assert.equal(h.$("#term-state").textContent, ""); h.close();
});

test("terminal penceresi: açılınca başlık '>_ sunucu', durum 'bağlı' (yeşil), oturum çipi hem pencerede hem alt çubukta", async () => {
  const h = await opened();
  assert.ok(isOpen(h)); assert.equal(h.$("#term-title").textContent, ">_ web1"); assert.deepEqual(state(h), { text: "bağlı", cls: "term-window-state ok" });
  assert.equal(h.$$("#term-tabs .term-chip").length, 1); assert.ok(h.$("#term-tabs .term-chip.active")); assert.equal(h.$("#term-dock").hidden, false); assert.equal(h.$$("#term-dock .term-chip").length, 1);
  assert.deepEqual(posts(h, "/api/term/open"), [{ server: "web1", rows: 24, cols: 80 }]); assert.deepEqual(h.errors, []); h.close();
});

test("terminal penceresi: bağlanırken 'bağlanıyor…' (sarı), sonra 'bağlı'", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#s=web1", overrides: api({ "/api/term/open": async () => { await gate; return { id: "t1" }; } }) });
  h.$("#srv-iterm").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r));
  assert.deepEqual(state(h), { text: "bağlanıyor…", cls: "term-window-state warn" });
  release(); await h.settle(); assert.deepEqual(state(h), { text: "bağlı", cls: "term-window-state ok" }); h.close();
});

test("terminal penceresi: açılamazsa not gösterilir ve durum 'bağlanamadı' (kırmızı)", async () => {
  const h = await opened({ "/api/term/open": { status: 500, body: "ssh yok" } });
  assert.ok(isOpen(h)); assert.match(h.$("#term-note").textContent, /^✖ /); assert.equal(h.$("#term-note").hidden, false); assert.deepEqual(state(h), { text: "bağlanamadı", cls: "term-window-state bad" }); h.close();
});

test("terminal penceresi: sarı düğme küçültür (oturum açık kalır, alt çubukta çip, kapatma isteği yok)", async () => {
  const h = await opened(); await h.click(".pty-dots .d-y");
  assert.ok(!isOpen(h)); assert.equal(h.$("#term-dock").hidden, false); assert.equal(h.$$("#term-dock .term-chip").length, 1); assert.deepEqual(posts(h, "/api/term/close"), []);
  await h.click("#term-dock .term-chip"); assert.ok(isOpen(h), "çipe tıklayınca pencere geri açılır"); assert.equal(h.$("#term-title").textContent, ">_ web1"); h.close();
});

test("terminal penceresi: kırmızı düğme oturumu kapatır (/api/term/close), pencere ve çipler kalkar, durum boşalır", async () => {
  const h = await opened(); await h.click(".pty-dots .d-r");
  assert.deepEqual(posts(h, "/api/term/close"), [{ id: "t1" }]); assert.ok(!isOpen(h)); assert.equal(h.$("#term-dock").hidden, true); assert.equal(h.$$("#term-tabs .term-chip").length, 0); assert.equal(h.$("#term-state").textContent, ""); h.close();
});

test("terminal penceresi: yeşil düğme tam ekran ↔ eski boyut (sınıf, aria-pressed, başlık) ve terminali yeniden sığdırır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: api() }); let fits = 0; h.win.FitAddon.FitAddon.prototype.fit = () => { fits++; };
  await h.click("#srv-iterm"); const w = h.$("#term-window"), g = h.$(".pty-dots .d-g"); const f0 = fits;
  await h.click(g); assert.ok(w.classList.contains("maximized")); assert.equal(g.getAttribute("aria-pressed"), "true"); assert.equal(g.title, "Eski boyuta dön"); assert.ok(fits > f0, "tam ekranda yeniden sığdırılır");
  await h.click(g); assert.ok(!w.classList.contains("maximized")); assert.equal(g.getAttribute("aria-pressed"), "false"); assert.equal(g.title, "Tam ekran / eski boyut"); assert.deepEqual(h.errors, []); h.close();
});

test("terminal penceresi: iki oturum — durum etkin oturumu izler; ilk oturumu kapatınca ikincisi açılır", async () => {
  let n = 0; const h = await boot({ hash: "#s=web1", overrides: api({ "/api/term/open": () => ({ id: "t" + (++n) }) }) });
  await h.click("#srv-iterm"); await h.click("#srv-iterm");
  assert.equal(h.$$("#term-tabs .term-chip").length, 2); assert.equal(h.$("#term-title").textContent, ">_ web1 #2"); assert.equal(state(h).text, "bağlı");
  await h.click(".modal-foot [data-term-close]"); assert.deepEqual(posts(h, "/api/term/close"), [{ id: "t2" }]); assert.ok(isOpen(h)); assert.equal(h.$("#term-title").textContent, ">_ web1"); assert.equal(h.$$("#term-tabs .term-chip").length, 1); h.close();
});
