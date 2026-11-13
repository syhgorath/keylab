"use strict";
/* Ayarlar / yeniden adlandırma / Sunucu ekle ek testleri (refaktör 5e): mutasyon turunda kaçanlar. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { fakeJobs } = require("./lib/fakejobs");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const base = (o = {}) => ({ "/api/home": mixedHome, ...o });
const tick = async () => { await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r)); };
const draftOk = { ok: true, errors: {}, envPath: "servers/yeni1/server.env", envText: "IP=1", keyPath: "k", commands: ["c"] };

test("ayarlar: sayfaya girişte /api/about yeniden okunur ve sistem parola penceresi durumu güncellenir", async () => {
  let avail = true;
  const h = await boot({ hash: "", overrides: base({ "/api/about": () => ({ ...fixture("about"), askpass: avail }) }) });
  avail = false; await h.click('[data-nav="settings"]');
  const sys = h.$('#set-ask button[data-v="system"]'); assert.equal(sys.disabled, true); assert.equal(sys.title, "Bu platformda sistem penceresi yok"); h.close();
});

test("yeniden adlandır: istek sürerken Tamam kilitli; başarıdan sonra liste yeniden okunur", async () => {
  let release; const gate = new Promise(r => { release = r; }); let renamed = false;
  const h = await boot({ hash: "", overrides: base({ "/api/server-rename": async () => { await gate; renamed = true; return { ok: true }; }, "/api/server": u => { const d = fixture("server-db1"); d.server.name = u.searchParams.get("name"); return d; } }) });
  await h.click('[data-tsrv="db1"]'); await h.click("#srv-more"); await h.click('#ctx [data-ctx="rename"]'); h.$("#rn-new").value = "db2";
  const homes = h.calls.filter(c => c.path === "/api/home").length;
  h.$("#dlg-ok").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await tick(); assert.equal(h.$("#dlg-ok").disabled, true, "istek sürerken kilitli");
  release(); await h.settle(); assert.ok(renamed); assert.equal(h.calls.filter(c => c.path === "/api/home").length, homes + 1, "liste yeniden okunur"); h.close();
});

test("hazırlık görevi: bu sunucuda iş sürerken yeni görev penceresi açılmaz", async () => {
  const f = fakeJobs();
  const h = await boot({ hash: "#s=web1", overrides: base({ ...f.overrides, "/api/server": () => { const d = fixture("server-web1"); d.server.up = true; d.server.hasKey = true; return d; } }) });
  await h.click("[data-test]"); await h.click("#run-go"); const title = h.$("#run-title").textContent; assert.match(title, /durum/);
  await h.click('[data-task="sshadd"]'); assert.equal(h.$("#run-title").textContent, title, "çalışan iş penceresi bozulmaz"); assert.equal(h.$("#run-go").textContent !== "Başlat", true); h.close();
});

test("sunucu ekle: profil listesi null gelirse hata göstermez; yalnız input olayı alanı 'dokunulmuş' yapar", async () => {
  let h = await boot({ hash: "#v=add", overrides: base({ "/api/profiles": () => null, "/api/server-draft": () => draftOk }) });
  assert.doesNotMatch(h.$("#e-os").textContent, /Profiller alınamadı/); h.close();
  h = await boot({ hash: "#v=add", overrides: base({ "/api/server-draft": () => ({ ok: false, errors: { name: "Ad geçersiz" } }) }) });
  h.timers.run(200); await h.settle(); assert.doesNotMatch(h.$("#e-name").textContent, /Ad geçersiz/);
  h.$("#add-name").value = "X"; h.$("#add-name").dispatchEvent(new h.win.Event("input", { bubbles: true })); h.timers.run(200); await h.settle();
  assert.match(h.$("#e-name").textContent, /Ad geçersiz/, "input olayı da alanı dokunulmuş sayar"); h.close();
});

test("sunucu ekle: oluşturma sürerken düğme kilitli ve eski hata temizlenir", async () => {
  let release, mode = "fail"; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#v=add", overrides: base({ "/api/server-draft": () => draftOk, "/api/server-create": async () => { if (mode === "fail") return { status: 400, body: "var" }; await gate; return { ok: true }; } }) });
  for (const [k, v] of Object.entries({ name: "yeni1", host: "10.9.9.9", user: "root", port: "22" })) { h.$("#add-" + k).value = v; h.$("#add-" + k).dispatchEvent(new h.win.Event("input", { bubbles: true })); }
  h.timers.run(200); await h.settle(); await h.click("#add-create"); assert.match(h.$("#add-err").textContent, /^✖ /);
  mode = "ok"; h.$("#add-create").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await tick();
  assert.equal(h.$("#add-create").disabled, true); assert.equal(h.$("#add-err").textContent, "", "yeni denemede eski hata silinir");
  release(); await h.settle(); h.close();
});
