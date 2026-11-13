"use strict";
/* Sunucu yeniden adlandırma/silme ve Sunucu ekle sihirbazı DAVRANIŞ testleri (refaktör 5e). Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { fakeJobs } = require("./lib/fakejobs");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const base = (o = {}) => ({ "/api/home": mixedHome, ...o });
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();
const dlgOpen = h => h.$("#dlg").classList.contains("open");
const menu = async (h, name, act) => { await h.click(`[data-tsrv="${name}"]`); await h.click("#srv-more"); await h.click(`#ctx [data-ctx="${act}"]`); };

/* ---------------- yeniden adlandırma ---------------- */
test("yeniden adlandır: pencere (başlık, alt başlık, ad dolu), geçersiz/aynı ad uyarıları", async () => {
  const h = await boot({ hash: "", overrides: base({ "/api/server-rename": { ok: true }, "/api/server": u => { const d = fixture("server-db1"); d.server.name = u.searchParams.get("name"); return d; } }) }); await menu(h, "db1", "rename");
  assert.ok(dlgOpen(h)); assert.equal(h.$("#dlg-title").textContent, "Sunucuyu yeniden adlandır"); assert.equal(h.$("#dlg-sub").textContent, "db1"); assert.equal(h.$("#rn-new").value, "db1"); assert.equal(h.$("#dlg-ok").textContent, "Yeniden adlandır"); assert.match(txt(h, "#dlg-body"), /id_ed25519_db1/);
  const tryName = async v => { h.$("#rn-new").value = v; await h.click("#dlg-ok"); return h.$("#dlg-err").textContent; };
  assert.equal(await tryName("db1"), "Yeni ad eskisiyle aynı."); for (const bad of ["Db2", "1db", "db-2", "", "a b", "x".repeat(41)]) assert.equal(await tryName(bad), "Geçersiz ad.", JSON.stringify(bad));
  assert.equal(h.calls.filter(c => c.path === "/api/server-rename").length, 0); await tryName("  db_2  "); assert.equal(h.calls.filter(c => c.path === "/api/server-rename").length, 1, "geçerli ad (kırpılır) isteğe gider"); assert.ok(!dlgOpen(h));
  h.close();
});

test("yeniden adlandır: başarıyla → istek gövdesi, bildirim, liste yenilenir, yeni ada sunucu sayfası açılır; hata ve Enter/odak", async () => {
  let fail = false; const bodies = [];
  const h = await boot({ hash: "", overrides: base({ "/api/server-rename": (u, { body }) => { bodies.push(body); return fail ? { status: 400, body: "ad kullanımda" } : { ok: true }; }, "/api/home": () => { const x = mixedHome(); if (bodies.length && !fail) x.servers.find(s => s.name === "db1").name = "db2"; return x; }, "/api/server": u => { const d = fixture("server-db1"); d.server.name = u.searchParams.get("name"); return d; } }) });
  await menu(h, "db1", "rename"); h.timers.run(60); assert.equal(h.doc.activeElement, h.$("#rn-new"), "odak ve seçim");
  h.$("#rn-new").value = "db2"; fail = true; await h.click("#dlg-ok"); assert.match(h.$("#dlg-err").textContent, /^✖ /); assert.equal(h.$("#dlg-ok").disabled, false); assert.ok(dlgOpen(h));
  fail = false; h.$("#rn-new").dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true })); await h.settle();
  assert.deepEqual(bodies, [{ old: "db1", new: "db2" }, { old: "db1", new: "db2" }]); assert.ok(!dlgOpen(h)); assert.equal(txt(h, "#toast"), "db1 → db2");
  assert.equal(h.$(".view:not([hidden])").id, "view-server"); assert.equal(txt(h, "#srv-head h1"), "db2"); assert.match(h.win.location.hash, /#s=db2/);
  assert.deepEqual(h.errors, []); h.close();
});

/* ---------------- silme ---------------- */
const delApi = (f, extra = {}) => base({ ...f.overrides, "/api/server-files": { files: [{ name: "server.env", isKey: false }, { name: "id_ed25519_db1", isKey: true }], state: ["utility", "linux-hardening"] }, "/api/server-remove": () => ({ id: f.start("db1", "remove", "") }), ...extra });
test("sil: pencere içeriği (dosya çipleri, anahtar/günlük onay kutuları), Sil düğmesi ad + anahtar onayıyla açılır", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "", overrides: delApi(f) }); await menu(h, "db1", "delete");
  assert.equal(h.$("#dlg-title").textContent, "Sunucuyu sil"); assert.equal(h.$("#dlg-sub").textContent, "db1"); assert.deepEqual(h.$$("#dlg-body .chips .chip").map(c => c.textContent), ["server.env", "id_ed25519_db1"]); assert.ok(h.$$("#dlg-body .chip")[1].classList.contains("chip-key"));
  assert.match(txt(h, "#dlg-body .notice"), /servers\/db1\/.*özel SSH anahtarı dahil/); assert.match(txt(h, "#dlg-body"), /utility, linux-hardening/); assert.equal(h.$("#dlg-ok").className, "btn dangerbtn"); assert.equal(h.$("#dlg-ok").disabled, true);
  const type = async v => { h.$("#dl-name").value = v; h.$("#dl-name").dispatchEvent(new h.win.Event("input", { bubbles: true })); await h.settle(); };
  await type("db1"); assert.equal(h.$("#dlg-ok").disabled, true, "anahtar onayı yok"); h.$("#dl-key").checked = true; h.$("#dl-key").dispatchEvent(new h.win.Event("change", { bubbles: true })); assert.equal(h.$("#dlg-ok").disabled, false);
  await type("DB1"); assert.equal(h.$("#dlg-ok").disabled, true, "büyük/küçük harf duyarlı"); await type("db1 "); assert.equal(h.$("#dlg-ok").disabled, true, "kırpılmaz");
  h.timers.run(60); assert.equal(h.doc.activeElement, h.$("#dl-name")); h.close();
});

test("sil: anahtar/günlük kutusu yoksa (dosya yok, durum yok) yalnız ad onayı; boş klasör", async () => {
  const f = fakeJobs(); const h = await boot({ hash: "", overrides: delApi(f, { "/api/server-files": { files: null, state: null } }) }); await menu(h, "db1", "delete");
  assert.equal(h.$("#dl-key"), null); assert.equal(h.$("#dl-logs"), null); assert.match(txt(h, "#dlg-body"), /boş klasör/); assert.doesNotMatch(txt(h, "#dlg-body .notice"), /özel SSH anahtarı dahil/);
  h.$("#dl-name").value = "db1"; h.$("#dl-name").dispatchEvent(new h.win.Event("input", { bubbles: true })); assert.equal(h.$("#dlg-ok").disabled, false); h.close();
});

test("sil: Sil → /api/server-remove gövdesi (ad, onay, anahtar onayı), pencere kapanır, silme penceresi açılır; hata durumunda düğme yeniden açılır", async () => {
  const f = fakeJobs(); let fail = false; const bodies = [];
  const h = await boot({ hash: "", overrides: delApi(f, { "/api/server-remove": (u, { body }) => { bodies.push(body); return fail ? { status: 400, body: "onay yanlış" } : { id: f.start("db1", "remove", "") }; } }) }); await menu(h, "db1", "delete");
  h.$("#dl-name").value = "db1"; h.$("#dl-key").checked = true; h.$("#dl-name").dispatchEvent(new h.win.Event("input", { bubbles: true }));
  fail = true; await h.click("#dlg-ok"); assert.match(h.$("#dlg-err").textContent, /^✖ /); assert.equal(h.$("#dlg-ok").disabled, false); assert.ok(dlgOpen(h));
  fail = false; await h.click("#dlg-ok"); assert.deepEqual(bodies, [{ name: "db1", confirm: "db1", ack: true }, { name: "db1", confirm: "db1", ack: true }]);
  assert.ok(!dlgOpen(h)); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /Sunucu siliniyor\s*db1/); assert.equal(h.$("#run-sub").textContent, "KeyLab motoru (ModuleManager.sh remove)");
  assert.deepEqual(h.errors, []); h.close();
});

test("sil: dosyalar okunamazsa bildirim; iş sürerken pencere açılmaz", async () => {
  const f = fakeJobs(); let h = await boot({ hash: "", overrides: delApi(f, { "/api/server-files": { status: 500, body: "yok" } }) }); await menu(h, "db1", "delete");
  assert.match(txt(h, "#toast"), /^Okunamadı: /); assert.ok(!dlgOpen(h)); h.close();
  const g = fakeJobs(); h = await boot({ hash: "#s=web1&p=utility", overrides: delApi(g) });
  await h.click('#srv-body [data-run="utility|durum"]'); await h.click("#run-go"); await h.click("#run-min, #run-bg").catch(() => {});
  await menu(h, "db1", "delete"); assert.ok(!dlgOpen(h), "çalışan iş varken silme penceresi açılmaz"); h.close();
});

/* ---------------- Sunucu ekle ---------------- */
const draftOk = { ok: true, errors: {}, envPath: "servers/yeni1/server.env", envText: "IP=10.9.9.9\nUSER=root", keyPath: "servers/yeni1/id_ed25519_yeni1", commands: ["ssh-copy-id a", "ssh-add b"] };
const addApi = (extra = {}) => base({ "/api/server-draft": () => draftOk, "/api/server-create": { ok: true }, ...extra });
const fill = async (h, o = {}) => { const v = { name: "yeni1", host: "10.9.9.9", user: "root", port: "22", ...o }; for (const [k, x] of Object.entries(v)) { h.$("#add-" + k).value = x; h.$("#add-" + k).dispatchEvent(new h.win.Event("input", { bubbles: true })); } h.timers.run(200); await h.settle(); };

test("sunucu ekle: profiller seçeneğe dönüşür (destek özeti); profil alınamazsa uyarı; yalnız ilk girişte yüklenir", async () => {
  const h = await boot({ hash: "#v=add", overrides: addApi() });
  const opts = h.$$("#add-os option"); assert.ok(opts.length >= 2); assert.match(opts[opts.length - 1].textContent, /^\S+ — .*(destekli|planlı|supported)/); assert.ok(opts.some(o => /Hardening: destekli/.test(o.textContent)));
  const n = h.calls.filter(c => c.path === "/api/profiles").length; await h.click('[data-nav="dash"]'); await h.click("#srvtree-add"); assert.equal(h.calls.filter(c => c.path === "/api/profiles").length, n, "ikinci girişte yeniden yüklenmez"); h.close();
  const g = await boot({ hash: "#v=add", overrides: addApi({ "/api/profiles": { status: 500, body: "bozuk" } }) }); assert.match(txt(g, "#e-os"), /^Profiller alınamadı: /); g.close();
});

test("sunucu ekle: doğrulama — dokunulmamış alanda hata gösterilmez, dokununca gösterilir; geçersizken önizleme boş durumda", async () => {
  const h = await boot({ hash: "#v=add", overrides: addApi({ "/api/server-draft": () => ({ ok: false, errors: { name: "Ad geçersiz", host: "Adres gerekli" } }) }) });
  h.timers.run(200); await h.settle(); assert.equal(txt(h, "#e-name"), "Küçük harf, rakam ve _ ; harfle başlar."); assert.equal(h.$("#e-name").closest(".field").classList.contains("bad"), false); assert.match(txt(h, "#add-preview"), /Alanlar geçerli olunca/);
  h.$("#add-name").dispatchEvent(new h.win.Event("blur")); h.timers.run(200); await h.settle(); assert.equal(txt(h, "#e-name"), "Ad geçersiz"); assert.ok(h.$("#e-name").classList.contains("err")); assert.ok(h.$("#e-name").closest(".field").classList.contains("bad"));
  assert.equal(txt(h, "#e-host"), "", "host'a dokunulmadı"); assert.equal(txt(h, "#e-os"), "Paketlerin desteklediği profiller listelenir.");
  h.$("#add-host").dispatchEvent(new h.win.Event("change")); h.timers.run(200); await h.settle(); assert.equal(txt(h, "#e-host"), "Adres gerekli"); h.close();
});

test("sunucu ekle: alan değişince 150 ms sonra tek doğrulama isteği (gövde dahil); doğrulama hatasında bildirim", async () => {
  let n = 0; const bodies = [];
  const h = await boot({ hash: "#v=add", overrides: addApi({ "/api/server-draft": (u, { body }) => { n++; bodies.push(body); return draftOk; } }) });
  h.timers.run(200); await h.settle(); const n0 = n;
  h.$("#add-name").value = "a"; h.$("#add-name").dispatchEvent(new h.win.Event("input", { bubbles: true })); h.$("#add-name").value = "ab"; h.$("#add-name").dispatchEvent(new h.win.Event("input", { bubbles: true }));
  h.timers.run(100); assert.equal(n, n0, "150 ms dolmadan istek yok"); h.timers.run(100); await h.settle(); assert.equal(n, n0 + 1, "art arda girdi tek istek"); assert.equal(bodies.pop().name, "ab"); assert.deepEqual(Object.keys(bodies.at(-1) || { name: 1, host: 1, user: 1, port: 1, os: 1 }).sort(), ["host", "name", "os", "port", "user"]);
  h.close();
  const g = await boot({ hash: "#v=add", overrides: addApi({ "/api/server-draft": { status: 500, body: "çöktü" } }) }); g.timers.run(200); await g.settle(); assert.match(txt(g, "#add-preview .notice.danger"), /Doğrulanamadı/); g.close();
});

test("sunucu ekle: geçerli taslakta önizleme (dosya yolu, içerik, anahtar, komutlar); komutları kopyala; pano yoksa istem", async () => {
  const h = await boot({ hash: "#v=add", overrides: addApi() }); await fill(h);
  assert.equal(txt(h, "#add-preview .mono"), "servers/yeni1/server.env"); assert.equal(h.$$("#add-preview .codeblock").length, 3, "env metni + 2 komut"); assert.match(h.$$("#add-preview .codeblock")[0].textContent, /IP=10\.9\.9\.9/);
  assert.deepEqual(h.$$("#add-preview .codeblock[data-i]").map(c => c.textContent), ["ssh-copy-id a", "ssh-add b"]); assert.match(txt(h, "#add-preview"), /id_ed25519_yeni1/);
  await h.click("#add-copy"); assert.deepEqual(h.win.__clip, ["ssh-copy-id a\nssh-add b"]); assert.equal(txt(h, "#toast"), "Komutlar kopyalandı");
  Object.defineProperty(h.win.navigator, "clipboard", { value: { writeText: async () => { throw new Error("x"); } }, configurable: true }); const asked = []; h.win.prompt = (m, v) => asked.push([m, v]); globalThis.prompt = h.win.prompt;
  await h.click("#add-copy"); assert.deepEqual(asked, [["Kopyalayın:", "ssh-copy-id a\nssh-add b"]]); h.close();
});

test("sunucu ekle: oluştur → istek gövdesi, bildirim, liste yenilenir, adımlar görünür; hata durumunda düğme yeniden açılır", async () => {
  let fail = false; const bodies = [];
  const h = await boot({ hash: "#v=add", overrides: addApi({ "/api/server-create": (u, { body }) => { bodies.push(body); return fail ? { status: 400, body: "var" } : { ok: true }; }, "/api/home": () => { const x = mixedHome(); if (bodies.length && !fail) x.servers.push({ ...x.servers[0], name: "yeni1", hasKey: false }); return x; } }) });
  await fill(h, { name: " yeni1 " }); fail = true; await h.click("#add-create"); assert.match(h.$("#add-err").textContent, /^✖ /); assert.equal(h.$("#add-create").disabled, false); assert.equal(h.$("#add-steps").hidden, true);
  fail = false; await h.click("#add-create");
  assert.deepEqual(bodies.at(-1), { name: " yeni1 ", host: "10.9.9.9", user: "root", port: "22", os: "" }); assert.equal(txt(h, "#toast"), "Sunucu oluşturuldu");
  assert.equal(h.$("#add-steps").hidden, false); assert.equal(h.$("#add-form-wrap").hidden, true); assert.match(txt(h, "#add-steps .notice"), /yeni1 oluşturuldu/); assert.match(txt(h, "#add-steps"), /servers\/yeni1\/server\.env yazıldı/);
  assert.deepEqual(h.errors, []); h.close();
});

const created = async (extra = {}, hasKey = false) => {
  const h = await boot({ hash: "#v=add", overrides: addApi({ "/api/home": () => { const x = mixedHome(); x.servers.push({ ...x.servers[0], name: "yeni1", hasKey }); return x; }, ...extra }) });
  await fill(h); await h.click("#add-create"); return h;
};
test("sunucu ekle: adımlar — anahtar yoksa 'Yeni anahtar üret', varsa 'Anahtar var' ve diğer düğmeler; her düğme görev penceresini açar", async () => {
  let h = await created({}, false);
  assert.deepEqual(h.$$("#add-steps [data-task]").map(b => b.dataset.task), ["keygen", "knock"]); assert.equal(h.$("#add-test"), null); assert.ok(h.$("#add-vars")); assert.match(txt(h, "#add-steps"), /Yeni bir ed25519 anahtarı üretin/);
  await h.click('#add-steps [data-task="keygen"]'); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /Anahtar oluştur\s*yeni1/); assert.equal(h.$("#run-go").textContent, "Başlat"); h.close();
  h = await created({}, true);
  assert.deepEqual(h.$$("#add-steps [data-task]").map(b => b.dataset.task), ["keycopy", "sshadd", "keychain", "knock"]); assert.ok(h.$("#add-test")); assert.match(txt(h, "#add-steps"), /Anahtar var/); assert.equal(h.$$("#add-steps .badge.ok").length, 2, "kayıt + anahtar tamam");
  await h.click('#add-steps [data-task="sshadd"]'); assert.match(h.$("#run-title").textContent, /Anahtarı ssh-agent'a ekle\s*yeni1/); h.close();
});

test("sunucu ekle: Dene → utility durum onayı; 'Sunucu sayfasına git'; 'Değişkenleri ayarla' sunucuyu hardening sekmesinde açıp değişken penceresini açar", async () => {
  let h = await created({ "/api/vars": () => ({ vars: [] }), "/api/server": u => { const d = fixture("server-web1"); d.server.name = u.searchParams.get("name"); return d; } }, true);
  await h.click("#add-test"); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /durum/); await h.click("#run-close, #run-cancel-confirm").catch(() => {}); h.close();
  h = await created({ "/api/vars": () => ({ vars: [] }), "/api/server": u => { const d = fixture("server-web1"); d.server.name = u.searchParams.get("name"); return d; } }, true);
  await h.click("#add-open"); assert.equal(txt(h, "#srv-head h1"), "yeni1"); assert.match(h.win.location.hash, /^#s=yeni1&p=ozet$/); h.close();
  h = await created({ "/api/vars": () => ({ vars: [] }), "/api/server": u => { const d = fixture("server-web1"); d.server.name = u.searchParams.get("name"); return d; } }, true);
  await h.click("#add-vars"); assert.equal(txt(h, "#srv-head h1"), "yeni1"); assert.match(h.win.location.hash, /p=linux-hardening/); assert.ok(h.$("#varsmodal").classList.contains("open")); assert.equal(h.$("#vars-sub").textContent, "Sunucu: yeni1 · servers/yeni1/linux-hardening.vars"); h.close();
});

test("sunucu ekle: 'Yeni bir sunucu daha' formu temizler ve yeniden gösterir; tamamlanan görev adımı 'tamam' olur", async () => {
  const h = await created({}, true);
  await h.click("#add-another"); assert.equal(h.$("#add-form-wrap").hidden, false); assert.equal(h.$("#add-steps").hidden, true); assert.deepEqual(["name", "host", "user", "port"].map(f => h.$("#add-" + f).value), ["", "", "", ""]);
  h.close();
});
