"use strict";
/* Gömülü terminal: her sunucunun terminalleri KENDİ pencere sekmelerinde (bağımsız pencereler); alt çubuk tüm oturumları gösterir;
   aynı sunucuda birden çok terminal; "+" ile aynı sunucuda yeni terminal; kapatınca başka sunucuya atlamaz. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const mk = () => { let n = 0; return { "/api/home": mixedHome, "/api/term/open": () => ({ id: "t" + (++n) }), "/api/term/close": { ok: true }, "/api/term/resize": { ok: true }, "/api/term/input": { ok: true } }; };
const isOpen = h => h.$("#termmodal").classList.contains("open");
const tabs = h => h.$$("#term-tabs .term-chip").map(c => c.textContent.replace(/\s+/g, " ").replace("✕", "").trim());
const dock = h => h.$$("#term-dock .term-chip").map(c => c.textContent.replace(/\s+/g, " ").replace("✕", "").trim());
const posts = (h, p) => h.calls.filter(c => c.method === "POST" && c.path === p).map(c => c.body);
const open = async (h, name) => { h.win.KeyLabTerminal.open(name); await h.settle(); };
const dockChip = (h, text) => h.$$("#term-dock .term-chip").find(c => c.textContent.includes(text));

test("sunucu penceresi: pencere yalnız etkin sunucunun terminallerini sekme olarak gösterir; alt çubuk hepsini", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); assert.deepEqual(tabs(h), [">_ web1"]);
  await open(h, "db1"); assert.equal(h.$("#term-title").textContent, ">_ db1"); assert.deepEqual(tabs(h), [">_ db1"], "web1'in terminali db1 penceresinde görünmez");
  assert.deepEqual(dock(h), [">_ web1", ">_ db1"], "alt çubuk tüm oturumları gösterir"); assert.deepEqual(h.errors, []); h.close();
});

test("sunucu penceresi: alt çubukta başka sunucunun çipi o sunucunun penceresini açar; diğer oturum kapanmaz", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); await open(h, "db1");
  await h.click(dockChip(h, "web1")); assert.ok(isOpen(h)); assert.equal(h.$("#term-title").textContent, ">_ web1"); assert.deepEqual(tabs(h), [">_ web1"]);
  assert.deepEqual(posts(h, "/api/term/close"), [], "pencere değiştirmek oturumu kapatmaz");
  await h.click(dockChip(h, "db1")); assert.equal(h.$("#term-title").textContent, ">_ db1"); h.close();
});

test("sunucu penceresi: aynı sunucuda iki terminal kendi sekmelerinde (#1, #2); diğer sunucu etkilenmez", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); await open(h, "web1"); await open(h, "db1");
  await h.click(dockChip(h, "web1 #2")); assert.deepEqual(tabs(h), [">_ web1 #1", ">_ web1 #2"]); assert.equal(h.$("#term-title").textContent, ">_ web1 #2");
  await h.click(h.$$("#term-tabs .term-chip")[0]); assert.equal(h.$("#term-title").textContent, ">_ web1 #1");
  await h.click(dockChip(h, "db1")); assert.deepEqual(tabs(h), [">_ db1"]); assert.deepEqual(dock(h), [">_ web1 #1", ">_ web1 #2", ">_ db1"]); h.close();
});

test("sunucu penceresi: '+' bu sunucuda yeni terminal açar (başka sunucuda değil); sınırda kapalı", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "db1"); assert.equal(h.$("#term-tabs .term-chip-add").textContent, "+"); assert.match(h.$("#term-tabs .term-chip-add").getAttribute("aria-label"), /db1/);
  await h.click("#term-tabs .term-chip-add"); assert.deepEqual(posts(h, "/api/term/open").map(b => b.server), ["db1", "db1"]); assert.deepEqual(tabs(h), [">_ db1 #1", ">_ db1 #2"]);
  await open(h, "web1"); await h.click("#term-tabs .term-chip-add"); assert.deepEqual(posts(h, "/api/term/open").map(b => b.server), ["db1", "db1", "web1", "web1"]);
  assert.equal(h.$("#term-tabs .term-chip-add").disabled, true, "4 oturumda '+' kapalı"); assert.equal(dock(h).length, 4); h.close();
});

test("sunucu penceresi: sunucunun son terminali kapanınca pencere kapanır (başka sunucuya atlamaz); ilk kapanınca aynı sunucunun diğerine geçer", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); await open(h, "web1"); await open(h, "db1");
  await h.click(".modal-foot [data-term-close]"); assert.deepEqual(posts(h, "/api/term/close"), [{ id: "t3" }]); assert.ok(!isOpen(h), "db1'in son terminali kapanınca pencere kapanır"); assert.deepEqual(dock(h), [">_ web1 #1", ">_ web1 #2"]);
  await h.click(dockChip(h, "web1 #2")); await h.click(".modal-foot [data-term-close]"); assert.deepEqual(posts(h, "/api/term/close").map(b => b.id), ["t3", "t2"]);
  assert.ok(isOpen(h)); assert.equal(h.$("#term-title").textContent, ">_ web1"); assert.deepEqual(tabs(h), [">_ web1"]); h.close();
});

test("sunucu penceresi: en çok 4 oturum; beşincisi açılmaz, uyarı gösterilir ve mevcut oturum açık kalır", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  for (const n of ["web1", "web1", "db1", "db1"]) await open(h, n);
  const before = posts(h, "/api/term/open").length; await open(h, "vm1");
  assert.equal(posts(h, "/api/term/open").length, before, "beşinci istek gitmez"); assert.match(h.$("#term-note").textContent, /En fazla 4/); assert.equal(dock(h).length, 4); h.close();
});

test("sunucu penceresi: sunucu sayfasındaki 'Gömülü terminal' her tıklamada o sunucuda yeni terminal açar", async () => {
  const h = await boot({ hash: "#s=web1", overrides: mk() });
  await h.click("#srv-iterm"); await h.click("#srv-iterm");
  assert.deepEqual(posts(h, "/api/term/open").map(b => b.server), ["web1", "web1"]); assert.deepEqual(tabs(h), [">_ web1 #1", ">_ web1 #2"]); h.close();
});

test("kırmızı düğme (pencereyi kapat): bu sunucunun TÜM terminallerini sonlandırır, başka sunucunun oturumlarına dokunmaz", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); await open(h, "web1"); await open(h, "db1"); await h.click(dockChip(h, "web1 #1"));
  await h.click(".pty-dots .d-r");
  assert.deepEqual(posts(h, "/api/term/close").map(b => b.id).sort(), ["t1", "t2"], "web1'in iki terminali kapanır"); assert.ok(!isOpen(h)); assert.deepEqual(dock(h), [">_ db1"], "db1 oturumu yaşar");
  assert.equal(h.$("#term-state").textContent, ""); await h.click(dockChip(h, "db1")); assert.deepEqual(tabs(h), [">_ db1"]); assert.deepEqual(h.errors, []); h.close();
});

test("terminal: alt çubuktaki 'Oturumu kapat' ve sekme çipinin ✕ düğmesi yalnız tek oturumu kapatır", async () => {
  const h = await boot({ hash: "", overrides: mk() });
  await open(h, "web1"); await open(h, "web1");
  await h.click(".modal-foot [data-term-close]"); assert.deepEqual(posts(h, "/api/term/close").map(b => b.id), ["t2"]); assert.deepEqual(tabs(h), [">_ web1"]);
  await open(h, "web1"); await h.click("#term-tabs .term-chip:not(.active) .term-chip-x"); assert.equal(posts(h, "/api/term/close").length, 2); assert.equal(tabs(h).length, 1); h.close();
});

test("terminal akışı: her oturum için önce /api/term/ticket ile bilet alınır, EventSource bu biletle açılır (kimlik + bilet adreste)", async () => {
  let n = 0; const h = await boot({ hash: "", overrides: { ...mk(), "/api/term/ticket": (u, { body }) => ({ ticket: "tk-" + body.id + "-" + (++n) }) } });
  await open(h, "web1"); await open(h, "db1");
  assert.deepEqual(posts(h, "/api/term/ticket"), [{ id: "t1" }, { id: "t2" }]);
  assert.deepEqual(h.win.__es, ["/api/term/stream?id=t1&ticket=tk-t1-1", "/api/term/stream?id=t2&ticket=tk-t2-2"]); assert.deepEqual(h.errors, []); h.close();
});

test("terminal akışı: bilet alınamazsa akış açılmaz; not gösterilir, durum 'bağlanamadı'", async () => {
  const h = await boot({ hash: "", overrides: { ...mk(), "/api/term/ticket": { status: 401, body: "oturum" } } });
  await open(h, "web1");
  assert.equal((h.win.__es || []).length, 0); assert.match(h.$("#term-note").textContent, /Akış bileti alınamadı/); assert.equal(h.$("#term-state").textContent, "bağlanamadı"); h.close();
});

test("terminal akışı: bilet beklenirken oturum kapatılırsa akış hiç açılmaz", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "", overrides: { ...mk(), "/api/term/ticket": async () => { await gate; return { ticket: "tk" }; } } });
  h.win.KeyLabTerminal.open("web1"); await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r));
  h.$(".pty-dots .d-r").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r));
  release(); await h.settle(); assert.equal((h.win.__es || []).length, 0); assert.deepEqual(h.errors, []); h.close();
});
