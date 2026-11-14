"use strict";
/* Paketler sayfası DAVRANIŞ testleri (refaktör 5b): paket listesi (arama, rozetler), ayrıntı penceresi, imzalı indeks kartı ve
   "Güncellemeler" / "Paketleri keşfet" pencereleri, yükseltme onayı girişi. Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const pk = (o = {}) => { const d = fixture("packages"); Object.assign(d, o); return d; };
const withPk = f => ({ "/api/home": mixedHome, "/api/packages": () => { const d = fixture("packages"); f(d); return d; } });
const pkgOf = (d, id) => d.packages.find(p => p.id === id);
const REPORT = { ok: true, serial: "7", valid: "2027-01-01", exit: 0, raw: "HAM-RAPOR", rows: [
  { id: "utility", installed: "0.1.0", verdict: "upgrade", version: "0.1.1", text: "" },
  { id: "kurulum", installed: "-", verdict: "install", version: "0.1.0", text: "" },
  { id: "linux-hardening", installed: "0.13.0", verdict: "current", version: "", text: "" },
  { id: "eski", installed: "0.1.0", verdict: "yanked", version: "", text: "" },
  { id: "tuhaf", installed: "0.1.0", verdict: "bilinmeyen-karar", version: "", text: "" },
] };
const upd = r => ({ "/api/updates": r });
const dlgOpen = h => h.$("#dlg").classList.contains("open");
const rowNames = h => h.$$("#pkg-page .pk-li").map(b => b.dataset.pk);
const txt = (h, s) => h.$(s).textContent.replace(/\s+/g, " ").trim();

/* ---------------- liste ---------------- */
test("paketler: liste sırası, sürüm/taban, kullanım sayısı; yükleme sonrası hata yok", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome } });
  assert.deepEqual(rowNames(h), ["linux-hardening", "utility", "kurulum"], "sabit sıra: hardening, utility, kurulum");
  const li = h.$('#pkg-page [data-pk="linux-hardening"]');
  assert.match(li.textContent, /Linux sertleştirme/); assert.match(li.textContent, /linux-hardening · syhgorath\/linux-hardening/); assert.match(li.textContent, /0\.13\.0/); assert.match(li.textContent, /taban 0\.13\.0/);
  assert.match(li.textContent, /1 sunucuda kullanıldı/); assert.match(h.$('#pkg-page [data-pk="kurulum"]').textContent, /hiçbir sunucuda kullanılmadı/);
  assert.equal(h.$('[data-pk="kurulum"] .pk-li-ico').textContent, "K"); assert.deepEqual(h.errors, []); h.close();
});

test("paketler: rozetler (taban altı, eski sürüm), 'taban —' ve arama süzgeci", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => {
    pkgOf(d, "linux-hardening").floor = "0.14.0";                       // current < floor
    pkgOf(d, "utility").uses[0].version = "0.0.9";                      // sunucuda eski
    pkgOf(d, "kurulum").floor = "";
  }) });
  assert.match(h.$('[data-pk="linux-hardening"]').textContent, /kilit tabanının altında/); assert.doesNotMatch(h.$('[data-pk="utility"]').textContent, /kilit tabanının altında/);
  assert.match(h.$('[data-pk="utility"]').textContent, /1 sunucuda eski sürüm/); assert.doesNotMatch(h.$('[data-pk="linux-hardening"]').textContent, /eski sürüm/);
  assert.match(h.$('[data-pk="kurulum"]').textContent, /taban —/);
  // arama süzgeci sayfa YÜKLENİRKEN uygulanır (yazarken liste yeniden çizilmez); sayfaya girişte etkili olur
  const go = async q => { await h.type("#search-input", q); await h.click('[data-nav="dash"]'); await h.click('[data-nav="packages"]'); };
  await go("UTIL"); assert.deepEqual(rowNames(h), ["utility"]);
  await go("syhgorath/kurulum"); assert.deepEqual(rowNames(h), ["kurulum"], "depo adıyla da aranır");
  await go("yokyokyok"); assert.match(h.$("#pkg-page").textContent, /Paket bulunamadı\./); assert.deepEqual(rowNames(h), []);
  assert.deepEqual(h.errors, []); h.close();
});

test("paketler: sürüm karşılaştırması noktalı sayısaldır (0.10.0 > 0.9.0, eksik parça 0)", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => {
    const u = pkgOf(d, "utility"); u.current = "0.10.0"; u.uses[0].version = "0.9.0";      // metin sırasıyla "0.9.0" > "0.10.0" olurdu
    const k = pkgOf(d, "linux-hardening"); k.current = "1.0"; k.uses[0].version = "1.0.0"; // 1.0 == 1.0.0 → eski değil
  }) });
  assert.match(h.$('[data-pk="utility"]').textContent, /1 sunucuda eski sürüm/); assert.doesNotMatch(h.$('[data-pk="linux-hardening"]').textContent, /eski sürüm/); h.close();
});

test("paketler: paket yok → boş durum; API hatası → hata kutusu; null listeler boş sayılır", async () => {
  let h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/packages": () => ({ packages: [], servers: [] }) } });
  assert.match(h.$("#pkg-page").textContent, /Paket bulunamadı/); assert.match(h.$("#pkg-page").textContent, /Henüz kurulu paket yok/); assert.doesNotMatch(h.$("#pkg-page").textContent, /components\/<paket>\/current/); h.close();
  // boş durumdaki düğme "Paketleri keşfet" penceresini açar (ilk kurulum buradan yapılır; kurulum onay akışından geçer)
  const REPORT = { ok: true, serial: "7", valid: "2027-01-01", exit: 0, raw: "HAM", rows: [{ id: "utility", installed: "-", verdict: "install", version: "0.1.0", text: "" }] };
  h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/packages": () => ({ packages: [], servers: [] }), "/api/updates": REPORT } });
  assert.equal(h.$("#pkg-empty-disc").textContent, "Paketleri keşfet ve kur");
  await h.click("#pkg-empty-disc");
  assert.ok(h.$("#dlg").classList.contains("open")); assert.equal(h.$("#dlg-title").textContent, "Paketleri keşfet");
  assert.ok(h.$("#disc-list .disc-row"), "indeksteki kurulabilir paket listelendi"); assert.match(h.$("#disc-list").textContent, /Kur…/);
  assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1); assert.deepEqual(h.errors, []); h.close();
  h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/packages": () => ({ packages: null, servers: null }) } });
  assert.match(h.$("#pkg-page").textContent, /Paket bulunamadı/); assert.deepEqual(h.errors, []); h.close();
  h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/packages": { status: 500, body: "bozuk" } } });
  assert.match(h.$("#pkg-page .notice.danger").textContent, /Yüklenemedi/); h.close();
});

/* ---------------- ayrıntı penceresi ---------------- */
test("paket ayrıntısı: sürüm havuzu, kilit tabanı, sunucu özeti ve sunucu satırları", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => { pkgOf(d, "linux-hardening").uses.push({ server: "db1", version: "0.12.0", action: "audit", result: "fail", time: "2026-11-09T10:00:00Z" }); }) });
  await h.click('[data-pk="linux-hardening"]');
  assert.ok(dlgOpen(h)); assert.equal(h.$("#dlg-title").textContent, "Linux sertleştirme"); assert.equal(h.$("#dlg-sub").textContent, "linux-hardening · syhgorath/linux-hardening");
  assert.ok(h.$("#dlg-ok").hidden); assert.equal(h.$("#dlg-cancel").textContent, "Kapat");
  assert.equal(h.$$("#dlg-body .vchip.cur").map(x => x.textContent).join(), "0.13.0"); assert.equal(h.$$("#dlg-body .spec-box")[0].querySelectorAll(".vchip").length, 17);
  assert.match(h.$$("#dlg-body .spec-box")[1].textContent, /0\.13\.0/); assert.match(h.$$("#dlg-body .spec-box")[1].textContent, /bu sürümün altı çalıştırılmaz/);
  assert.match(txt(h, "#dlg-body .spec-box:nth-child(3)"), /2 kullandı · 1 eski · 1 kullanmadı/);
  assert.equal(h.$("#pd-q"), null, "8 sunucudan azda arama kutusu yok"); assert.ok(h.$("#pd-stale")); assert.equal(h.$("#pd-stale").checked, false, "az sunucuda başlangıçta işaretsiz");
  const names = () => h.$$("#pd-rows .urow .who").map(x => x.textContent);
  assert.ok(h.$("#pd-none")); assert.equal(h.$("#pd-none").checked, true, "az sunucuda kullanılmayanlar da gösterilir");
  assert.deepEqual(names().sort(), ["db1", "vm1", "web1"]);
  await h.click("#pd-stale"); assert.deepEqual(names(), ["db1"], "yalnız eski sürümlüler");
  await h.click("#pd-stale");
  const row = n => [...h.$$("#pd-rows .urow")].find(r => r.querySelector(".who").textContent === n).textContent.replace(/\s+/g, " ");
  assert.match(row("web1"), /0\.13\.0USB ile aynı✔ audit/); assert.match(row("db1"), /0\.12\.0USB'de daha yeni var✖ audit/); assert.match(row("vm1"), /—kullanılmadı/);
  await h.click("#pd-none"); assert.deepEqual(names().sort(), ["db1", "web1"]);
  assert.deepEqual(h.errors, []); h.close();
});

test("paket ayrıntısı: USB'den yeni sürüm rozeti, taban altı uyarısı, kilitsiz paket", async () => {
  const h = await boot({ hash: "#v=packages", overrides: withPk(d => { const u = pkgOf(d, "utility"); u.uses[0].version = "0.2.0"; u.floor = "0.5.0"; pkgOf(d, "kurulum").floor = ""; }) });
  await h.click('[data-pk="utility"]'); assert.match(h.$("#pd-rows").textContent, /USB'den yeni/); assert.match(h.$("#dlg-body").textContent, /USB sürümü tabanın altında/); assert.doesNotMatch(h.$("#dlg-body").textContent, /bu sürümün altı çalıştırılmaz/);
  await h.click("#dlg-cancel"); assert.ok(!dlgOpen(h));
  await h.click('[data-pk="kurulum"]'); assert.match(h.$("#dlg-body").textContent, /kilitte yok/); assert.match(h.$("#dlg-body").textContent, /0 kullandı/); assert.doesNotMatch(h.$("#dlg-body").textContent, /eski/); assert.match(h.$("#dlg-body").textContent, /3 kullanmadı/);
  assert.deepEqual(h.errors, []); h.close();
});

test("paket ayrıntısı: 8'den çok sunucuda arama kutusu ve süzgeçler (kullanılmayanlar başta gizli)", async () => {
  const many = d => { for (let i = 0; i < 9; i++) d.servers.push({ name: "x" + i, os: "debian" }); const u = pkgOf(d, "utility"); u.uses.push({ server: "x1", version: "0.0.1", action: "durum", result: "ok", time: "2026-11-09T10:00:00Z" }, { server: "x2", version: "0.1.0", action: "durum", result: "ok", time: "2026-11-09T10:00:00Z" }); };
  const h = await boot({ hash: "#v=packages", overrides: withPk(many) });
  await h.click('[data-pk="utility"]');
  assert.ok(h.$("#pd-q")); assert.equal(h.$("#pd-stale").checked, true); assert.equal(h.$("#pd-none").checked, false);
  const names = () => h.$$("#pd-rows .urow .who").map(x => x.textContent);
  assert.deepEqual(names(), ["x1"]); await h.click("#pd-stale"); assert.deepEqual(names().sort(), ["db1", "x1", "x2"]);
  await h.click("#pd-none"); assert.equal(names().length, 12);
  await h.type("#pd-q", " X1 "); assert.deepEqual(names(), ["x1"]); await h.type("#pd-q", "zzz"); assert.match(h.$("#pd-rows").textContent, /Gösterilecek sunucu yok/);
  assert.match(h.$("#dlg-body").textContent, /kullanılmayanları da göster \(9\)/, "kullanılmayan sayısı"); assert.deepEqual(h.errors, []); h.close();
});

/* ---------------- imzalı indeks kartı ve Güncellemeler penceresi ---------------- */
test("güncellemeler kartı: ilk durum, denetle → özet; liste satırlarında 'çıktı' rozeti", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, ...upd(REPORT) } });
  assert.match(txt(h, "#upd-card"), /henüz denetlenmedi · hiçbir şey kurmaz/); assert.equal(txt(h, "#upd-check"), "Güncellemeleri denetle");
  assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 0, "sayfa açılınca ağdan denetlemez");
  await h.click("#upd-check");
  assert.ok(dlgOpen(h)); assert.equal(h.$("#dlg-title").textContent, "Güncellemeler"); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1);
  assert.match(txt(h, "#upd-card"), /⬆ 2 güncelleme var/); assert.equal(txt(h, "#upd-check"), "Ayrıntı");
  assert.equal(txt(h, '[data-upd="utility"]'), "⬆ 0.1.1 çıktı"); assert.equal(txt(h, '[data-upd="kurulum"]'), "⬆ 0.1.0 çıktı"); assert.equal(txt(h, '[data-upd="linux-hardening"]'), "");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncellemeler penceresi: satırlar, karar etiketleri, Kur/Yükselt düğmeleri, ham rapor, başlık özeti", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, ...upd(REPORT) } });
  await h.click("#upd-check");
  const rows = h.$$("#dlg-body .pkg-row"); assert.equal(rows.length, 5);
  const byId = id => rows.find(r => r.querySelector(".pkg-name").textContent === id);
  assert.match(byId("utility").textContent, /0\.1\.0.*0\.1\.1.*⬆ yükseltilebilir/); assert.equal(byId("utility").querySelector("button").textContent, "Yükselt…");
  assert.match(byId("kurulum").textContent, /kurulu değil.*0\.1\.0.*📥 kurulabilir/); assert.equal(byId("kurulum").querySelector("button").textContent, "Kur…");
  assert.match(byId("linux-hardening").textContent, /✔ güncel/); assert.equal(byId("linux-hardening").querySelector("button"), null);
  assert.match(byId("eski").textContent, /🚫 önerilmiyor \(yanked\)/); assert.match(byId("tuhaf").textContent, /bilinmiyor/);
  assert.ok(byId("eski").querySelector(".badge.danger")); assert.ok(byId("tuhaf").querySelector(".badge.plain")); assert.ok(byId("utility").querySelector(".badge.warn")); assert.ok(byId("kurulum").querySelector(".badge.info"));
  assert.match(txt(h, "#dlg-body"), /İndeks seri 7 · geçerlilik 2027-01-01 · 2 güncelleme var · denetim \d/); assert.match(h.$("#dlg-body pre").textContent, /HAM-RAPOR/);
  assert.equal(byId("utility").querySelector("button").dataset.from, "0.1.0"); assert.equal(byId("utility").querySelector("button").dataset.to, "0.1.1");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncellemeler: her şey güncel; indeks doğrulanamadı (güvenli kapanma); hata; yeniden denetle", async () => {
  let rep = { ...REPORT, rows: REPORT.rows.filter(r => r.verdict === "current") };
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": () => rep } });
  await h.click("#upd-check"); assert.match(txt(h, "#upd-card"), /✔ hepsi güncel/); assert.match(txt(h, "#dlg-body"), /hepsi güncel/);
  rep = { ok: false, raw: "IMZA-HATASI", rows: [] };
  await h.click("#upd-recheck"); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 2);
  assert.match(txt(h, "#upd-card"), /✕ indeks doğrulanamadı \(güvenli kapanma\)/); assert.match(txt(h, "#dlg-body"), /İndeks doğrulanamadı \(güvenli kapanma\): yükseltme yapılmaz\./); assert.match(h.$("#dlg-body pre").textContent, /IMZA-HATASI/); assert.equal(h.$$("#dlg-body .pkg-row").length, 0);
  rep = { status: 500, body: "ag yok" };
  await h.click("#upd-recheck"); assert.match(txt(h, "#upd-card"), /✕ denetlenemedi/); assert.match(h.$("#dlg-body .notice.danger").textContent, /Denetlenemedi: /); assert.equal(txt(h, "#upd-check"), "Ayrıntı");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncellemeler: denetim sürerken kart ve düğmeler meşgul; ikinci istek başlamaz; bitince pencere yeniden çizilir", async () => {
  let release; const gate = new Promise(r => { release = r; });
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": async () => { await gate; return REPORT; } } });
  h.$("#upd-check").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r)); await new Promise(r => setImmediate(r));
  assert.match(txt(h, "#upd-card"), /İmzalı indeks alınıyor ve doğrulanıyor/); assert.equal(h.$("#upd-check").disabled, true); assert.equal(h.$("#upd-recheck").disabled, true);
  assert.ok(h.$("#dlg-body .spinner")); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1);
  h.$("#disc-open").click(); await h.settle(); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1, "meşgulken yeni istek yok");
  release(); await h.settle(); assert.match(txt(h, "#upd-card"), /⬆ 2 güncelleme var/);
  assert.equal(h.$("#dlg-title").textContent, "Paketleri keşfet"); assert.ok(h.$("#disc-list .disc-row"), "açık keşfet penceresi sonuçla yeniden çizildi");
  assert.deepEqual(h.errors, []); h.close();
});

test("güncellemeler: pencerede Yükselt → pencere kapanır, onay penceresi açılır (yükseltme kipi)", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, ...upd(REPORT) } });
  await h.click("#upd-check"); await h.click('#dlg-body [data-upg="kurulum"]');
  assert.ok(!dlgOpen(h)); assert.ok(h.$("#runmodal").classList.contains("open"));
  assert.match(h.$("#run-title").textContent, /^upgrade · kurulum USB'ye yazar$/); assert.match(h.$("#run-confirm-text").textContent, /kurulacak/); assert.match(h.$("#run-confirm-text").textContent, /0\.1\.0 sürümü/);
  assert.match(h.$("#run-confirm-extra").textContent, /\(syhgorath\/kurulum\)/); assert.equal(h.$("#run-go").disabled, true); await h.click("#u-ack"); assert.equal(h.$("#run-go").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

/* ---------------- Paketleri keşfet ---------------- */
const dsc = h => h.$$("#disc-list .disc-row").map(r => r.querySelector(".pkg-name").textContent.replace(/\s+/g, " ").trim());
test("keşfet: açılınca indeksi alır; süzgeç sayıları, süzgeçler, arama, açıklamalar, Kur/Yükselt", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, ...upd(REPORT) } });
  await h.click("#disc-open");
  assert.equal(h.$("#dlg-title").textContent, "Paketleri keşfet"); assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1);
  assert.deepEqual(h.$$("#dlg-body [data-df]").map(b => b.textContent), ["Tümü (5)", "USB'de var (4)", "Kurulabilir (1)", "Güncellenebilir (1)"]);
  assert.equal(h.$('[data-df="all"]').classList.contains("primary"), true); assert.equal(dsc(h).length, 5);
  assert.match(txt(h, "#dlg-body"), /seri 7/); assert.match(h.$$("#disc-list .disc-row")[0].textContent, /Linux sertleştirme|utility/);
  const rowOf = id => h.$$("#disc-list .disc-row").find(r => r.textContent.includes(id));
  assert.match(rowOf("Kurulum merkezi").textContent, /kurulu değil/); assert.match(rowOf("Kurulum merkezi").textContent, /Kur…/); assert.match(rowOf("Kurulum merkezi").textContent, /Sunucuya Docker taban kurulumu/);
  assert.match(rowOf("eski").textContent, /Açıklama, paket USB'ye indirildikten sonra görünür/.test(rowOf("eski").textContent) ? /Açıklama/ : /eski/);  // kurulu → açıklama ipucu yok
  assert.doesNotMatch(rowOf("eski").textContent, /Açıklama, paket USB/); assert.equal(rowOf("Kurulum merkezi").querySelector(".pk-li-ico").textContent, "K");
  await h.click('[data-df="installed"]'); assert.equal(dsc(h).length, 4); assert.equal(h.$('[data-df="installed"]').classList.contains("primary"), true);
  await h.click('[data-df="install"]'); assert.equal(dsc(h).length, 1); await h.click('[data-df="upgrade"]'); assert.equal(dsc(h).length, 1); assert.match(dsc(h)[0], /Sunucu araçları/);
  await h.click('[data-df="all"]'); await h.type("#disc-q", " DOCKER "); assert.equal(dsc(h).length, 1); assert.match(dsc(h)[0], /Kurulum merkezi/);
  await h.type("#disc-q", "yokyok"); assert.match(h.$("#disc-list").textContent, /Bu süzgeçle eşleşen paket yok/);
  assert.deepEqual(h.errors, []); h.close();
});

test("keşfet: arama ve süzgeç yeniden açılışta korunur; Kur → pencere kapanır, yükseltme onayı açılır", async () => {
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, ...upd(REPORT) } });
  await h.click("#disc-open"); await h.click('[data-df="install"]'); await h.type("#disc-q", "kur");
  await h.click("#dlg-cancel"); await h.click("#disc-open");
  assert.equal(h.$("#disc-q").value, "kur"); assert.equal(h.$('[data-df="install"]').classList.contains("primary"), true); assert.equal(dsc(h).length, 1);
  assert.equal(h.calls.filter(c => c.path === "/api/updates").length, 1, "rapor varsa yeniden almaz");
  await h.click("#disc-list [data-dget]"); assert.ok(!dlgOpen(h)); assert.ok(h.$("#runmodal").classList.contains("open")); assert.match(h.$("#run-title").textContent, /upgrade · kurulum/);
  assert.deepEqual(h.errors, []); h.close();
});

test("keşfet: indeks doğrulanamazsa / hata olursa uyarı + 'Yeniden dene'", async () => {
  let rep = { ok: false, raw: "x", rows: [] };
  const h = await boot({ hash: "#v=packages", overrides: { "/api/home": mixedHome, "/api/updates": () => rep } });
  await h.click("#disc-open"); assert.match(h.$("#dlg-body .notice.danger").textContent, /İndeks doğrulanamadı \(güvenli kapanma\): paket listesi gösterilmez\./);
  rep = { status: 500, body: "ag yok" }; await h.click("#disc-retry"); assert.match(h.$("#dlg-body .notice.danger").textContent, /Denetlenemedi: /);
  rep = REPORT; await h.click("#disc-retry"); assert.equal(dsc(h).length, 5); assert.deepEqual(h.errors, []); h.close();
});
