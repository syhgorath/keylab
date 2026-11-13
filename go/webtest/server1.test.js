"use strict";
/* Sunucu sayfası DAVRANIŞ testleri 1/3 (refaktör 5d): açılış, başlık düğmeleri, adres düzenleme, sekmeler, Özet sekmesi.
   Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
// /api/server yanıtını değiştirir: f(d, ad)
const srv = f => ({ "/api/home": mixedHome, "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.up = d.server.name !== "vm1"; d.server.conn = d.server.up ? "up" : "down"; f && f(d, u.searchParams.get("name")); return d; } });
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const tabs = h => h.$$("#srv-tabs button").map(b => b.dataset.p + (b.getAttribute("aria-selected") === "true" ? "*" : ""));
const calls = (h, p) => h.calls.filter(c => c.path === p || c.path.startsWith(p + "?"));

/* ---------------- açılış ve başlık ---------------- */
test("sunucu sayfası: başlık, durum, adres, sekmeler (Özet, Hardening, Utility, Kurulum), adres çubuğu", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() });
  assert.equal(h.$(".view:not([hidden])").id, "view-server");
  assert.equal(txt(h, "#srv-head h1"), "web1"); assert.match(txt(h, "#srv-head .sv-state"), /çevrimiçi/); assert.ok(h.$("#srv-head .sv-state").classList.contains("ok"));
  assert.equal(txt(h, "#srv-addr"), "root@10.255.0.10:22"); assert.ok(h.$("#srv-head svg.osico"));
  assert.deepEqual(tabs(h), ["ozet*", "linux-hardening", "utility", "kurulum"]); assert.deepEqual(h.$$("#srv-tabs button").map(b => b.textContent), ["Özet", "Hardening", "Utility", "Kurulum"]);
  assert.match(h.win.location.hash, /^#s=web1&p=ozet$/);
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu sayfası: durum rozeti tonları (geçersiz, denetleniyor, çevrimdışı, knock)", async () => {
  const cases = [
    [s => { s.valid = false; }, "warn", /geçersiz/], [s => { s.checked = false; s.up = false; }, "plain", /denetleniyor/],
    [s => { s.up = false; s.conn = "down"; }, "danger", /bağlantı yok/], [s => { s.up = false; s.knock = true; }, "warn", /knock/],
    [s => { s.up = false; s.conn = "refused"; }, "warn", /./],
  ];
  for (const [f, tone, re] of cases) {
    const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/home": () => { const x = mixedHome(); f(x.servers.find(s => s.name === "web1")); return x; }, "/api/server": u => { const d = fixture("server-web1"); d.server.up = true; d.server.conn = "up"; f(d.server); return d; } } });
    assert.ok(h.$("#srv-head .sv-state").classList.contains(tone), tone + " " + h.$("#srv-head .sv-state").className); assert.match(txt(h, "#srv-head .sv-state"), re); h.close();
  }
});

test("sunucu sayfası: geçersiz sunucuda adres rozeti yok, düğmeler kapalı", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.valid = false; }) });
  assert.equal(h.$("#srv-addr"), null); assert.match(txt(h, "#srv-head"), /server\.env geçersiz/);
  for (const id of ["#srv-term", "#srv-iterm", "#srv-copy"]) assert.equal(h.$(id).disabled, true, id);
  assert.ok(h.$("#srv-edit") && h.$("#srv-more")); h.close();
});

test("sunucu sayfası: platform etiketi; sekme ve adres çubuğu (p=…) açılışta uygulanır", async () => {
  const h = await boot({ hash: "#s=web1&p=utility", overrides: srv(d => { d.server.platform = "ubuntu 26.04"; }) });
  assert.equal(txt(h, "#srv-head .tag"), "ubuntu 26.04"); assert.deepEqual(tabs(h), ["ozet", "linux-hardening", "utility*", "kurulum"]); h.close();
});

test("sunucu sayfası: yüklenirken 'yükleniyor…', hata → bildirim, eski yanıt yenisini ezmez", async () => {
  let release; const gate = new Promise(r => { release = r; });
  let h = await boot({ hash: "", overrides: { ...srv(), "/api/server": async u => { if (u.searchParams.get("name") === "web1") await gate; return fixture("server-" + u.searchParams.get("name")); } } });
  h.$('[data-tsrv="web1"]').dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r));
  assert.match(txt(h, "#srv-body"), /yükleniyor…/); assert.equal(txt(h, "#srv-head h1"), "web1"); assert.equal(h.$("#srv-tabs").innerHTML, ""); assert.match(h.win.location.hash, /^#s=web1&p=ozet$/);
  await h.click('[data-tsrv="db1"]'); await h.settle(); assert.equal(txt(h, "#srv-head h1"), "db1");
  release(); await h.settle(); assert.equal(txt(h, "#srv-head h1"), "db1", "geç gelen web1 yanıtı db1'i ezmez"); h.close();
  h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/server": { status: 500, body: "patladi" } } });
  assert.match(h.$("#srv-body .notice.danger").textContent, /Sunucu açılamadı/); h.close();
});

test("sunucu sayfası: geri bağlantısı Sunucular sayfasına döner", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() }); await h.click("#srv-back"); assert.equal(h.$(".view:not([hidden])").id, "view-servers"); h.close();
});

/* ---------------- başlık düğmeleri ---------------- */
test("sunucu sayfası: 'Terminal'de bağlan' → /api/action terminal; knock varsa ayrı bildirim; hata bildirimi", async () => {
  let reply = { ok: true };
  const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/action": (u, { body }) => reply } });
  await h.click("#srv-term"); assert.deepEqual(h.calls.find(c => c.path === "/api/action").body, { name: "web1", action: "terminal" }); assert.equal(txt(h, "#toast"), "Terminal'de açıldı");
  reply = { knock: true }; await h.click("#srv-term"); assert.match(txt(h, "#toast"), /knock Terminal'de gönderiliyor/);
  reply = { status: 500, body: "olmadi" }; await h.click("#srv-term"); assert.match(txt(h, "#toast"), /^Hata: /);
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu sayfası: 'Gömülü terminal' KeyLabTerminal.open(sunucu) çağırır", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() }); const seen = []; h.win.KeyLabTerminal.open = n => seen.push(n);
  await h.click("#srv-iterm"); assert.deepEqual(seen, ["web1"]); h.close();
});

test("sunucu sayfası: 'SSH komutu' ve adres rozeti panoya kopyalar; pano yoksa istem penceresi", async () => {
  const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/action": { cmd: "ssh root@10.255.0.10" } } });
  await h.click("#srv-copy"); assert.deepEqual(h.win.__clip, ["ssh root@10.255.0.10"]); assert.equal(txt(h, "#toast"), "Komut kopyalandı");
  assert.deepEqual(calls(h, "/api/action").map(c => c.body), [{ name: "web1", action: "cmd" }]);
  await h.click("#srv-addr"); assert.deepEqual(h.win.__clip, ["ssh root@10.255.0.10", "root@10.255.0.10:22"]); assert.equal(txt(h, "#toast"), "Adres kopyalandı");
  Object.defineProperty(h.win.navigator, "clipboard", { value: { writeText: async () => { throw new Error("izin yok"); } }, configurable: true });
  const asked = []; h.win.prompt = (m, v) => { asked.push([m, v]); }; globalThis.prompt = h.win.prompt;
  await h.click("#srv-addr"); await h.click("#srv-copy"); assert.deepEqual(asked, [["Kopyalayın:", "root@10.255.0.10:22"], ["Kopyalayın:", "ssh root@10.255.0.10"]]);
  assert.deepEqual(h.errors, []); h.close();
});

test("sunucu sayfası: '⋯' menüsü bağlam menüsünü açar", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() });
  await h.click("#srv-more"); const m = h.$("#ctxmenu, .ctxmenu, [role=menu]"); assert.ok(m, "bağlam menüsü"); assert.ok(m.classList.contains("open") || !m.hidden);
  assert.match(m.textContent, /Yeniden adlandır/); h.close();
});

/* ---------------- adres düzenleme ---------------- */
test("adres düzenleme: alanlar dolu, değişiklik yoksa uyarı, kaydet → /api/server-edit + yenileme", async () => {
  let ok = true; const edits = [];
  const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/server-edit": (u, { body }) => { edits.push(body); return ok ? { ok: true } : { status: 400, body: { error: "port geçersiz" }.error }; } } });
  await h.click("#srv-edit");
  assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-title").textContent, "Bağlantı bilgilerini düzenle"); assert.equal(h.$("#dlg-sub").textContent, "web1");
  assert.deepEqual([h.$("#ea-host").value, h.$("#ea-user").value, h.$("#ea-port").value], ["10.255.0.10", "root", "22"]); assert.equal(h.$("#dlg-ok").textContent, "Kaydet");
  await h.click("#dlg-ok"); assert.equal(h.$("#dlg-err").textContent, "Değişiklik yok."); assert.equal(edits.length, 0);
  h.$("#ea-port").value = ""; h.$("#ea-host").value = " 10.0.0.9 "; ok = false; await h.click("#dlg-ok");
  assert.deepEqual(edits[0], { name: "web1", host: "10.0.0.9", user: "root", port: "22" }, "boş port 22 olur, alanlar kırpılır"); assert.match(h.$("#dlg-err").textContent, /^✖ /); assert.equal(h.$("#dlg-ok").disabled, false); assert.ok(h.$("#dlg").classList.contains("open"));
  ok = true; const homes = calls(h, "/api/home").length, srvs = calls(h, "/api/server").length;
  h.$("#ea-user").dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true })); await h.settle();
  assert.equal(edits.length, 2, "Enter kaydeder"); assert.ok(!h.$("#dlg").classList.contains("open")); assert.equal(txt(h, "#toast"), "Bağlantı bilgileri güncellendi");
  assert.equal(calls(h, "/api/home").length, homes + 1); assert.equal(calls(h, "/api/server").length, srvs + 1, "sunucu sayfası yeniden okunur");
  assert.deepEqual(h.errors, []); h.close();
});

test("adres düzenleme: sunucu okunamazsa bildirim; odak adres alanına gider", async () => {
  let fail = false;
  const h = await boot({ hash: "#s=web1", overrides: { ...srv(), "/api/server": u => (fail ? { status: 500, body: "yok" } : fixture("server-web1")) } });
  fail = true; await h.click("#srv-edit"); assert.match(txt(h, "#toast"), /^Okunamadı: /); assert.ok(!h.$("#dlg").classList.contains("open"));
  fail = false; await h.click("#srv-edit"); h.timers.run(60); assert.equal(h.doc.activeElement, h.$("#ea-host")); h.close();
});

/* ---------------- sekmeler ---------------- */
test("sekmeler: tıklayınca adres çubuğu (replaceState) ve içerik değişir; bilinmeyen sekme Özet'e düşer", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() });
  const hist = h.win.history.length;
  await h.click('#srv-tabs [data-p="utility"]'); assert.match(h.win.location.hash, /^#s=web1&p=utility$/); assert.deepEqual(tabs(h), ["ozet", "linux-hardening", "utility*", "kurulum"]); assert.equal(h.win.history.length, hist, "geçmişe kayıt eklenmez");
  assert.ok(h.$("#srv-body .pk-info"));
  await h.click('#srv-tabs [data-p="ozet"]'); assert.ok(h.$("#srv-body .ozet4")); h.close();
  const g = await boot({ hash: "#s=web1&p=yok-boyle-paket", overrides: srv() }); assert.deepEqual(tabs(g), ["ozet*", "linux-hardening", "utility", "kurulum"]); g.close();
});

test("sekmeler: sunucuda paket yoksa yalnız Özet; paket sekmesi 'Bu sunucu için paket bulunamadı'", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv(d => { d.packages = []; }) });
  assert.deepEqual(tabs(h), ["ozet*"]); assert.match(txt(h, "#srv-body"), /Bu sunucu için paket yok\./); assert.equal(h.$("[data-test]"), null, "utility yoksa 'Bağlantıyı dene' yok"); h.close();
});

test("sekmeler: sıra sabit (hardening, utility, kurulum), bilinmeyen paketler sonda ve adıyla", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv(d => { const k = d.packages.find(p => p.id === "kurulum"); d.packages = [{ ...k, id: "zzz", name: "Zzz paket", items: null }, ...d.packages.filter(p => p.id !== "kurulum"), k].reverse(); }) });
  assert.deepEqual(h.$$("#srv-tabs button").map(b => b.dataset.p), ["ozet", "linux-hardening", "utility", "kurulum", "zzz"]); assert.equal(h.$$("#srv-tabs button").pop().textContent, "Zzz paket"); h.close();
});

/* ---------------- Özet sekmesi ---------------- */
test("özet: Durum & ağ kartı (TCP rozeti, sistem, anahtar, knock) ve Sertleştirme kartı", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.os = "ubuntu"; d.server.platform = "26.04"; d.server.hasKey = true; }) });
  const net = h.$$(".oz-card")[0]; assert.match(txt(h, ".oz-card .badge"), /TCP erişilebilir/); assert.match(net.textContent, /Ubuntu · 26\.04/); assert.match(net.textContent, /SSH anahtarı\s*var/); assert.match(net.textContent, /Port knocking\s*kapalı/);
  const lyn = h.$$(".oz-card")[1]; assert.ok(lyn.querySelector(".gauge.ok") || lyn.querySelector(".gauge")); assert.match(lyn.textContent, /1 uyarı/); assert.match(lyn.textContent, /12 öneri/); assert.match(lyn.textContent, /Ölçüm: 2026-11-10/); assert.ok(lyn.querySelector("[data-go-lynis]"));
  h.close();
});

test("özet: TCP rozeti varyantları; Lynis yoksa yönlendirme; skor kutusu rengi; güncel değil bayrağı", async () => {
  const mk = f => boot({ hash: "#s=web1", overrides: srv(f) });
  let h = await mk(d => { d.server.valid = false; }); assert.match(txt(h, ".oz-card .badge"), /yapılandırma geçersiz/); h.close();
  h = await mk(d => { d.server.checked = false; }); assert.match(txt(h, ".oz-card .badge"), /denetleniyor/); h.close();
  h = await mk(d => { d.server.up = false; d.server.knock = true; }); assert.match(txt(h, ".oz-card .badge"), /kapalı · knock gerekir/); h.close();
  h = await mk(d => { d.server.up = false; }); assert.match(txt(h, ".oz-card .badge"), /erişilemiyor/); h.close();
  h = await mk(d => { d.server.lynis = null; }); const l = h.$$(".oz-card")[1]; assert.match(l.textContent, /Bu sunucu için ölçüm yok/); assert.ok(l.querySelector('[data-goto="linux-hardening"]')); assert.equal(l.querySelector("[data-go-lynis]"), null); h.close();
  const sc = async n => { const x = await mk(d => { d.server.lynis.score = n; }); const c = x.$$(".oz-card")[1].querySelector(".gauge").className; x.close(); return c; };
  assert.match(await sc(85), /\bok\b/); assert.match(await sc(80), /\bok\b/); assert.match(await sc(79), /\bwarn\b/); assert.match(await sc(60), /\bwarn\b/); assert.match(await sc(59), /\bdanger\b/);
  h = await mk(d => { d.server.recent = [{ package: "linux-hardening", version: "0.13.0", action: "apply", result: "ok", time: "2026-11-11T12:00:00Z" }, ...d.server.recent]; });
  assert.match(txt(h, ".oz-card:nth-child(2)"), /⚠ güncel değil/); h.close();
  h = await mk(d => { d.server.recent = [{ package: "linux-hardening", version: "0.13.0", action: "apply", result: "ok", time: "2026-11-10T00:00:30Z" }]; });
  assert.doesNotMatch(txt(h, ".oz-card:nth-child(2)"), /güncel değil/, "ölçümden 60 sn'den az sonraki apply sayılmaz"); h.close();
});

test("özet: paketler ve son işlemler kartları, günlükler bağlantısı", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv() });
  assert.equal(h.$$(".mini-row").length, 3); assert.match(h.$$(".mini-row")[0].textContent, /Linux sertleştirme.*v0\.13\.0/); assert.deepEqual(h.$$(".mini-row [data-goto]").map(b => b.dataset.goto), ["linux-hardening", "utility", "kurulum"]);
  assert.equal(h.$$(".feed-row").length, 2); assert.ok(h.$$(".feed-row")[0].classList.contains("ok")); assert.match(h.$$(".feed-row")[0].textContent, /linux-hardening 0\.13\.0.*✓ audit/);
  await h.click('.mini-row [data-goto="utility"]'); assert.match(h.win.location.hash, /p=utility/); assert.ok(h.$("#srv-body .pk-info")); h.close();
  const g = await boot({ hash: "#s=db1", overrides: srv(d => { d.server.recent = [{ package: "utility", version: "0.1.0", action: "durum", result: "fail", time: "2026-11-10T10:00:00Z" }]; }) });
  assert.ok(g.$(".feed-row.fail")); assert.match(g.$(".feed-row").textContent, /✖ durum/);
  const f = await boot({ hash: "#s=vm1", overrides: srv(d => { d.server.recent = null; d.server.lynis = null; }) }); assert.match(f.$(".oz-card:nth-child(4)").textContent, /Henüz işlem yok/); g.close(); f.close();
});

test("özet: en çok 4 son işlem gösterilir", async () => {
  const h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.recent = Array.from({ length: 6 }, (_, i) => ({ package: "utility", version: "0.1.0", action: "a" + i, result: "ok", time: "2026-11-10T10:00:00Z" })); }) });
  assert.equal(h.$$(".feed-row").length, 4); h.close();
});

test("özet: bağlantı hazırlığı adımları ve sayacı (anahtar, ajan, Keychain, knock)", async () => {
  const mk = f => boot({ hash: "#s=web1", overrides: srv(f) });
  const rows = h => h.$$(".prep-row").map(r => [r.querySelector(".prep-dot").className.replace("prep-dot ", ""), r.querySelector(".prep-title").textContent.replace(/\s+/g, " ").trim()]);
  let h = await mk(); assert.equal(txt(h, ".prep-count"), "0/3 adım hazır"); assert.deepEqual(rows(h).map(r => r[0]), ["todo", "idle", "idle", "idle", "idle"]); assert.ok(h.$('[data-task="keygen"]')); assert.equal(h.$('[data-task="keycopy"]'), null); assert.equal(h.$('[data-task="sshadd"]'), null); assert.ok(h.$('[data-task="knock"]')); assert.equal(txt(h, '[data-task="knock"]'), "Ayarla…");
  assert.equal(h.$("[data-test]").disabled, true, "anahtar yokken Dene kapalı"); h.close();
  h = await mk(d => { d.server.hasKey = true; d.prep = { agent: true, passphrase: true, key: true }; });
  assert.equal(txt(h, ".prep-count"), "3/3 adım hazır"); assert.deepEqual(rows(h).map(r => r[0]), ["ok", "idle", "ok", "ok", "idle"]); assert.equal(h.$('[data-task="keygen"]'), null);
  assert.deepEqual(h.$$("[data-task]").map(b => b.textContent), ["Gönder…", "Yeniden ekle…", "Ayarla…", "Ayarla…"]); assert.match(txt(h, ".prep-row:nth-child(3)"), /ajanda/); assert.match(txt(h, ".prep-row:nth-child(4)"), /bu Mac'te kayıtlı/); assert.equal(h.$("[data-test]").disabled, false); h.close();
  h = await mk(d => { d.server.hasKey = true; d.prep = { agent: false, passphrase: false }; d.knock = { enabled: true, hasSeq: true, proto: "udp", delay: "500", hint: "3 port" }; });
  assert.equal(txt(h, ".prep-count"), "2/4 adım hazır"); assert.ok(h.$(".prep-count").title.includes("ve knock")); assert.equal(h.$$(".prep-row")[4].querySelector(".prep-dot").className.includes("ok"), true);
  assert.match(txt(h, ".prep-row:nth-child(5)"), /3 port/); assert.match(txt(h, ".prep-row:nth-child(5)"), /Açık · udp, 500 ms/); assert.equal(txt(h, '[data-task="knock"]'), "Değiştir…"); assert.match(txt(h, ".oz-card"), /Port knocking\s*açık · udp/); h.close();
  h = await mk(d => { d.knock = { enabled: true, hasSeq: false, proto: "tcp", delay: "300" }; });
  assert.ok(h.$$(".prep-row")[4].querySelector(".prep-dot.warn")); assert.match(txt(h, ".prep-row:nth-child(5)"), /bu Mac'te dizi yok/); assert.match(txt(h, ".oz-card"), /açık · dizi yok/); h.close();
});

test("özet: geçersiz sunucuda hazırlık düğmeleri kapalı; adım düğmeleri görev penceresini açar; Dene → utility durum onayı", async () => {
  let h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.valid = false; d.server.hasKey = true; }) });
  assert.ok(h.$$("[data-task]").every(b => b.disabled)); h.close();
  h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.hasKey = true; }) });
  await h.click('[data-task="sshadd"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /web1/); assert.equal(h.$("#run-sub").textContent, "Bağlantı hazırlığı"); assert.equal(h.$("#run-go").textContent, "Başlat");
  await h.click("#run-cancel-confirm, #run-close"); h.close();
  h = await boot({ hash: "#s=web1", overrides: srv(d => { d.server.hasKey = true; }) });
  await h.click("[data-test]"); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /durum/); h.close();
  h = await boot({ hash: "#s=web1", overrides: { ...srv(d => { d.server.hasKey = true; }), "/api/knock": { enabled: false, hasSeq: false, proto: "tcp", delay: "300", ports: "" } } });
  await h.click('[data-task="knock"]'); assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-title").textContent, "Port knocking"); assert.equal(h.$("#dlg-sub").textContent, "web1"); h.close();
});
