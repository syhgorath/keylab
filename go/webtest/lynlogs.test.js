"use strict";
/* Lynis sayfası ve Günlükler sayfası DAVRANIŞ testleri (refaktör 5): süzgeçler, arama, açma, hata satırı atlama, giriş noktaları
   (goLogs/goLynis/openLastLog), grafik, yeniden boyutlandırmada yeniden çizim. Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const L = (server, pkg, file, extra = {}) => ({ server, package: pkg, file, size: 2048, mtime: "2026-11-10T10:00:00Z", ...extra });
const LOGS = [
  L("web1", "utility", "last-run.log"),
  L("web1", "linux-hardening", "last-run.log"),
  L("web1", "linux-hardening", "20261101-audit.log", { size: 100 }),
  L("db1", "utility", "20261102-durum.log"),
];
const LOGTEXT = "PLAY [x]\nok: [web1]\nfatal: [web1]: FAILED! => boom\nhello a.b*c (x)\nson satir";
const logApi = (over = {}) => ({ "/api/home": mixedHome, "/api/logs": LOGS, "/api/log": (u) => over.text ?? LOGTEXT, ...over.o });
const rows = h => h.$$("#loglist button[data-i]").map(b => b.textContent.replace(/\s+/g, " ").trim());
const chips = (h, attr) => h.$$(`#logfilters [data-${attr}]`).map(b => `${b.dataset[attr]}${b.getAttribute("aria-pressed") === "true" ? "*" : ""}`);

/* ---------------- Günlükler ---------------- */
test("günlükler: liste, sayaç ve süzgeç düğmeleri (sayılarla)", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  assert.equal(h.$("#logs-count").textContent, "4 / 4 dosya");
  assert.equal(rows(h).length, 4);
  assert.match(rows(h)[0], /web1 · utility · last-run\.log/); assert.match(rows(h)[0], /2 KB/); assert.match(rows(h)[2], /1 KB/); // en az 1 KB
  assert.deepEqual(chips(h, "srv"), ["*", "db1", "web1"]); assert.deepEqual(chips(h, "pkg"), ["*", "linux-hardening", "utility"]);
  assert.match(h.$('#logfilters [data-srv="web1"]').textContent, /web1\s*3/); assert.match(h.$('#logfilters [data-pkg="linux-hardening"]').textContent, /Hardening\s*2/); // paket kısa adı
  assert.ok(!h.$("#logclear"), "süzgeç yokken temizle düğmesi yok");
  await h.click('#logfilters [data-kind="all"]'); // ilk çizim ana veri gelmeden olabilir; yeniden çizimde simge görünür
  assert.ok(h.$("#loglist svg.osico[width='16']"), "satırda küçük OS simgesi");
  assert.deepEqual(h.errors, []); h.close();
});

test("günlükler: sunucu/paket/tür süzgeçleri, arama ve temizleme", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  await h.click('#logfilters [data-srv="web1"]'); assert.equal(rows(h).length, 3); assert.equal(h.$("#logs-count").textContent, "3 / 4 dosya"); assert.ok(h.$("#logclear"));
  await h.click('#logfilters [data-pkg="linux-hardening"]'); assert.equal(rows(h).length, 2);
  await h.click('#logfilters [data-kind="last"]'); assert.equal(rows(h).length, 1); assert.match(rows(h)[0], /linux-hardening · last-run/);
  await h.click('#logfilters [data-kind="arch"]'); assert.equal(rows(h).length, 1); assert.match(rows(h)[0], /audit\.log/);
  await h.click('#logfilters [data-kind="all"]'); assert.equal(rows(h).length, 2);
  await h.click("#logclear"); assert.equal(rows(h).length, 4); assert.ok(!h.$("#logclear"));
  assert.deepEqual(chips(h, "srv"), ["*", "db1", "web1"]); assert.equal(h.$('#logfilters [data-kind="all"]').getAttribute("aria-pressed"), "true");
  await h.type("#logq", "DURUM"); assert.equal(rows(h).length, 1); assert.match(rows(h)[0], /db1/); assert.ok(h.$("#logclear"));
  await h.type("#logq", "yokboyle"); assert.equal(rows(h).length, 0); assert.match(h.$("#loglist").textContent, /Süzgece uyan günlük yok/);
  assert.equal(h.$("#logs-count").textContent, "0 / 4 dosya");
  await h.click("#logclear"); assert.equal(h.$("#logq").value, ""); assert.equal(rows(h).length, 4);
  assert.deepEqual(h.errors, []); h.close();
});

test("günlükler: arama kutusu yeniden çizimde odağı ve imleci korur", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  const q = h.$("#logq"); q.focus(); q.value = "web"; q.setSelectionRange(2, 2);
  q.dispatchEvent(new h.win.Event("input", { bubbles: true })); await h.settle();
  const n = h.$("#logq"); assert.notEqual(n, q, "kutu yeniden çizildi"); assert.equal(h.doc.activeElement, n); assert.equal(n.selectionStart, 2); assert.equal(n.value, "web");
  h.close();
});

test("günlükler: hiç günlük yok → boş durum; API hatası → bildirim", async () => {
  let h = await boot({ hash: "#v=logs", overrides: logApi({ o: { "/api/logs": [] } }) });
  assert.match(h.$("#loglist").textContent, /Henüz günlük yok/); assert.equal(h.$("#logs-count").textContent, "0 dosya"); assert.equal(h.$("#logfilters").innerHTML, ""); h.close();
  h = await boot({ hash: "#v=logs", overrides: logApi({ o: { "/api/logs": { status: 500, body: "bozuk" } } }) });
  assert.match(h.$("#toast").textContent, /Günlükler alınamadı/); h.close();
  h = await boot({ hash: "#v=logs", overrides: logApi({ o: { "/api/logs": () => null } }) }); // null → boş liste
  assert.match(h.$("#loglist").textContent, /Henüz günlük yok/); assert.deepEqual(h.errors, []); h.close();
});

test("günlükler: günlüğü aç (istek, başlık, geçerli satır, hata satırı sınıfı)", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  assert.ok(h.$("#logcard").hidden);
  await h.click(h.$$("#loglist button[data-i]")[2]);
  const c = h.calls.find(x => x.path.startsWith("/api/log?")); assert.ok(c);
  const q = new URLSearchParams(c.path.split("?")[1]); assert.equal(q.get("package"), "linux-hardening"); assert.equal(q.get("server"), "web1"); assert.equal(q.get("file"), "20261101-audit.log");
  assert.ok(!h.$("#logcard").hidden); assert.equal(h.$("#logtitle").textContent, "web1 · linux-hardening · 20261101-audit.log");
  assert.equal(h.$$("#loglist button[aria-current='true']").length, 1); assert.equal(h.$$("#loglist button[data-i]")[2].getAttribute("aria-current"), "true");
  const lines = h.$$("#logview div"); assert.equal(lines.length, 5); assert.deepEqual(lines.map(d => d.classList.contains("err")), [false, false, true, false, false]);
  // süzgeç değişince açık günlük işareti korunur (logKey)
  await h.click('#logfilters [data-srv="web1"]'); assert.equal(h.$$("#loglist button[aria-current='true']").length, 1);
  await h.click('#logfilters [data-srv="db1"]'); assert.equal(h.$$("#loglist button[aria-current='true']").length, 0);
  assert.deepEqual(h.errors, []); h.close();
});

test("günlükler: günlük açılamazsa bildirim, kart açılmaz", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi({ o: { "/api/log": { status: 404, body: "yok" } } }) });
  await h.click(h.$$("#loglist button[data-i]")[0]);
  assert.match(h.$("#toast").textContent, /Açılamadı/); assert.ok(h.$("#logcard").hidden); h.close();
});

test("günlükler: günlükte ara (işaretleme, özel karakterler, boş satır), açınca arama temizlenir", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi({ text: "ab\n\nA.B*c (x)\nAB ab" }) });
  await h.click(h.$$("#loglist button[data-i]")[0]);
  assert.equal(h.$$("#logview div")[1].textContent, " ", "boş satır tek boşluk");
  await h.type("#logfind", "ab"); assert.deepEqual(h.$$("#logview mark").map(m => m.textContent), ["ab", "AB", "ab"]);
  await h.type("#logfind", "a.b*c (x"); assert.deepEqual(h.$$("#logview mark").map(m => m.textContent), ["A.B*c (x"]); // normal ifade değil, düz metin
  assert.equal(h.$$("#logview div")[2].textContent, "A.B*c (x)");
  await h.type("#logfind", "yok"); assert.equal(h.$$("#logview mark").length, 0);
  await h.type("#logfind", ""); assert.equal(h.$$("#logview mark").length, 0); assert.equal(h.$$("#logview div").length, 4);
  await h.type("#logfind", "ab"); await h.click(h.$$("#loglist button[data-i]")[1]); assert.equal(h.$("#logfind").value, "", "yeni günlük açılınca arama sıfırlanır");
  assert.equal(h.$$("#logview mark").length, 0); h.close();
});

test("günlükler: arama ilk eşleşmeye kaydırır", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  await h.click(h.$$("#loglist button[data-i]")[0]);
  const seen = []; h.win.HTMLElement.prototype.scrollIntoView = function (o) { seen.push([this.tagName, o && o.block]); };
  await h.type("#logfind", "son"); assert.deepEqual(seen, [["MARK", "center"]]);
  seen.length = 0; await h.type("#logfind", "yokyok"); assert.deepEqual(seen, [], "eşleşme yoksa kaydırma yok"); h.close();
});

test("günlükler: 'hata satırına git' vurgular ve 1,5 sn sonra kaldırır; hata yoksa bildirir", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  await h.click(h.$$("#loglist button[data-i]")[0]);
  const seen = []; h.win.HTMLElement.prototype.scrollIntoView = function (o) { seen.push([this.className, o && o.block]); };
  await h.click("#logerr"); assert.ok(h.$("#logview .err").classList.contains("search-hit")); assert.equal(seen.length, 1); assert.equal(seen[0][1], "center");
  h.timers.run(1400); assert.ok(h.$("#logview .err").classList.contains("search-hit")); h.timers.run(200); assert.ok(!h.$("#logview .err").classList.contains("search-hit"));
  h.close();
  const g = await boot({ hash: "#v=logs", overrides: logApi({ text: "temiz\nsatir" }) });
  await g.click(g.$$("#loglist button[data-i]")[0]); await g.click("#logerr"); assert.match(g.$("#toast").textContent, /Hata satırı yok/); g.close();
});

test("günlükler: sunucu sayfasından 'günlükler' sunucuyu süzer, eski süzgeçler sıfırlanır", async () => {
  const h = await boot({ hash: "#v=logs", overrides: logApi() });
  await h.click('#logfilters [data-pkg="utility"]'); await h.click('#logfilters [data-kind="arch"]'); await h.type("#logq", "zzz");
  await h.click('[data-tsrv="db1"]'); // sunucu sayfası
  const b = h.$("#srv-body [data-go-logs]"); assert.ok(b, "sunucu özetinde günlükler düğmesi"); await h.click(b);
  assert.equal(h.$("#view-logs").hidden, false);
  assert.deepEqual(chips(h, "srv"), ["", "db1*", "web1"].map(x => x === "" ? "" : x)); assert.equal(h.$('#logfilters [data-pkg=""]').getAttribute("aria-pressed"), "true");
  assert.equal(h.$('#logfilters [data-kind="all"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$("#logq").value, "");
  assert.equal(rows(h).length, 1); assert.deepEqual(h.errors, []); h.close();
});

test("günlükler: son çalıştırma günlüğünü aç (dock'taki [data-log] düğmesi → openLastLog) → süzgeç + günlük açık", async () => {
  const h = await boot({ hash: "", overrides: logApi() });
  h.$("#dock-body").insertAdjacentHTML("beforeend", '<button id="fake-log" data-log="linux-hardening" data-srv="web1">son günlük</button>');
  await h.click("#fake-log"); await h.settle();
  assert.equal(h.$(".view:not([hidden])").id, "view-logs");
  assert.equal(h.$("#logs-count").textContent, "1 / 4 dosya"); assert.deepEqual(chips(h, "srv"), ["", "db1", "web1*"]); assert.deepEqual(chips(h, "pkg"), ["", "linux-hardening*", "utility"]);
  assert.equal(h.$('#logfilters [data-kind="last"]').getAttribute("aria-pressed"), "true");
  assert.ok(!h.$("#logcard").hidden); assert.equal(h.$("#logtitle").textContent, "web1 · linux-hardening · last-run.log"); assert.equal(h.$$("#loglist button[aria-current='true']").length, 1);
  // eşleşen son günlük yoksa liste süzülür ama günlük açılmaz
  h.$("#logcard").hidden = true;
  h.$("#dock-body").insertAdjacentHTML("beforeend", '<button id="fake2" data-log="kurulum" data-srv="web1">x</button>'); await h.click("#fake2"); await h.settle();
  assert.equal(h.$("#logs-count").textContent, "0 / 4 dosya"); assert.ok(h.$("#logcard").hidden);
  assert.deepEqual(h.errors, []); h.close();
});

/* ---------------- Lynis */
const LYN = {
  web1: { server: "web1", report: "r.dat", lynis: { date: "2026-11-10", dates: ["2026-11-01T10:00:00", "2026-11-05", "2026-11-10"], history: [61, 68, 74], score: 74, suggestions: 12, warnings: 1 },
    findings: [
      { id: "SSH-7408", kind: "uyari", text: "Zayıf SSH ayarı", value: "PermitRootLogin=no", closesByDefault: "evet", area: "ssh", detail: "ayrıntı", suggestion: "öneri metni" },
      { id: "KRNL-6000", kind: "oneri", text: "Kernel parametresi", value: "net.x=1", closesByDefault: "hayir" },
      { id: "AUTH-9286", kind: "oneri", text: "Parola yaşı", value: "", detail: "sadece ayrıntı" },
    ] },
  db1: { server: "db1", report: "", lynis: { date: "2026-11-12", dates: ["2026-11-12"], history: [50], score: 50, suggestions: 3, warnings: 0 }, findings: [{ id: "DB-1", kind: "oneri", text: "tek bulgu", value: "" }] },
};
const homeLyn = () => { const h = mixedHome(); h.servers.find(s => s.name === "db1").lynis = LYN.db1.lynis; h.servers.find(s => s.name === "web1").lynis = LYN.web1.lynis; return h; };
const lynApi = (o = {}) => ({ "/api/home": homeLyn, "/api/lynis": u => LYN[u.searchParams.get("server")] || { server: u.searchParams.get("server"), lynis: null, findings: null }, ...o });
const frows = h => h.$$("#lyn-rows .frow").map(r => r.textContent.replace(/\s+/g, " ").trim());

test("lynis: sunucu seçici (puanlar, 'ölçüm yok'), en yeni ölçümlü sunucu seçili açılır", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: lynApi() });
  const btns = h.$$("#lynis-pick .pickbtn"); assert.equal(btns.length, 3);
  assert.deepEqual(btns.filter(b => b.getAttribute("aria-selected") === "true").map(b => b.dataset.n), ["db1"]);
  assert.match(h.$('#lynis-pick [data-n="db1"]').textContent, /db1\s*50/); assert.match(h.$('#lynis-pick [data-n="vm1"]').textContent, /ölçüm yok/);
  assert.ok(h.$("#lynis-pick svg.osico[width='26']")); assert.deepEqual(h.errors, []);
  assert.equal(h.calls.filter(c => c.path.startsWith("/api/lynis")).length, 1); assert.match(h.calls.find(c => c.path.startsWith("/api/lynis")).path, /server=db1$/); h.close();
});

test("lynis: sunucu seç → skor, delta, grafik, bulgular; süzgeç ve arama; seçince süzgeç sıfırlanır", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: lynApi() });
  await h.click('#lynis-pick [data-n="web1"]');
  assert.match(h.$("#lynis-detail h2").textContent, /Skor geçmişi — web1/); assert.equal(h.$(".lynis-score").textContent, "74"); assert.equal(h.$(".lynis-delta").textContent, "▲ +13");
  assert.match(h.$(".lynis-meta").textContent, /uyarı 1/); assert.match(h.$(".lynis-meta").textContent, /öneri 12/); assert.match(h.$(".lynis-meta").textContent, /61 → 74/);
  assert.match(h.$("#lynis-detail .hint").textContent, /3 ölçüm · rapor r\.dat/);
  const svg = h.$(".chartbox svg"); assert.ok(svg); assert.equal(svg.querySelectorAll("circle.pt").length, 3); assert.equal(svg.querySelectorAll("circle.pt.last").length, 1); assert.ok(svg.querySelector("polyline.ln")); assert.ok(svg.querySelector("path.ar"));
  assert.match(svg.getAttribute("aria-label"), /61, 68, 74/); assert.match(svg.querySelector("circle.pt title").textContent, /2026-11-01 10:00 · skor 61/);
  assert.deepEqual([...svg.querySelectorAll("text.axis")].slice(-2).map(x => x.textContent), ["11-01", "11-10"]);
  assert.equal(h.$("#lyn-count").textContent, "3 / 3 bulgu");
  const r = frows(h); assert.match(r[0], /SSH-7408/); assert.match(r[0], /uyarı/); assert.match(r[0], /paket uygulayınca kapanır/); assert.match(r[0], /alan: ssh/); assert.match(r[0], /ayrıntı · öneri metni/); assert.match(r[0], /PermitRootLogin=no/);
  assert.match(r[1], /isteğe bağlı \(varsayılan kapalı\)/); assert.match(r[1], /öneri/); assert.doesNotMatch(r[1], /alan:/);
  assert.doesNotMatch(r[2], /fchips|paket uygulayınca/); assert.equal(h.$$("#lyn-rows .frow")[2].querySelectorAll(".chip").length, 0); assert.match(r[2], /sadece ayrıntı/);
  await h.click('#lynis-detail [data-k="uyari"]'); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu"); assert.equal(h.$('#lynis-detail [data-k="uyari"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$('#lynis-detail [data-k="all"]').getAttribute("aria-pressed"), "false");
  await h.click('#lynis-detail [data-k="oneri"]'); assert.equal(h.$("#lyn-count").textContent, "2 / 3 bulgu");
  await h.type("#lyn-q", "KERNEL"); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu"); await h.type("#lyn-q", "ssh"); assert.match(h.$("#lyn-rows").textContent, /Eşleşen bulgu yok/);
  await h.click('#lynis-detail [data-k="all"]'); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu"); // 'ssh' alan/kimlikte
  await h.type("#lyn-q", "net.x"); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu", "değer alanında da aranır");
  await h.type("#lyn-q", "ssh"); await h.click('#lynis-pick [data-n="db1"]'); assert.equal(h.$("#lyn-q").value, ""); assert.equal(h.$('#lynis-detail [data-k="all"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$("#lyn-count").textContent, "1 / 1 bulgu");
  assert.deepEqual(h.errors, []); h.close();
});

test("lynis: tek ölçüm → delta ve çizgi yok; ölçüm yoksa yönlendirme; bulgu null → boş", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: lynApi() });
  assert.equal(h.$(".lynis-delta"), null); assert.equal(h.$(".chartbox polyline"), null); assert.equal(h.$(".chartbox path.ar"), null); assert.equal(h.$$(".chartbox circle.pt").length, 1);
  assert.equal(h.$$(".chartbox text.axis").length, 5 + 1, "5 ızgara + tek tarih etiketi");
  await h.click('#lynis-pick [data-n="vm1"]'); assert.match(h.$("#lynis-detail").textContent, /Bu sunucu için ölçüm yok/); assert.deepEqual(h.errors, []); h.close();
});

test("lynis: sunucu yok → boş durum; yükleme hatası → bildirim kutusu", async () => {
  let h = await boot({ hash: "#v=lynis", overrides: { "/api/home": () => ({ ...fixture("home"), servers: [] }) } });
  assert.match(h.$("#lynis-detail").textContent, /Sunucu yok/); assert.equal(h.$("#lynis-pick").innerHTML, ""); h.close();
  h = await boot({ hash: "#v=lynis", overrides: lynApi({ "/api/lynis": { status: 500, body: "patladi" } }) });
  assert.match(h.$("#lynis-detail .notice.danger").textContent, /Yüklenemedi/); h.close();
});

test("lynis: sunucu sayfasından (hardening sekmesi) aç → o sunucu seçili; uyarı varsa 'uyarı' süzgeci hazır, yoksa 'tümü'", async () => {
  const srv = warn => u => { const d = fixture("server-" + u.searchParams.get("name")); d.server.lynis.warnings = warn; return d; };
  let h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: lynApi({ "/api/server": srv(2) }) });
  const b = h.$("#srv-body [data-go-lynis-p]"); assert.ok(b); await h.click(b);
  assert.equal(h.$(".view:not([hidden])").id, "view-lynis"); assert.equal(h.$('#lynis-pick [aria-selected="true"]').dataset.n, "web1");
  assert.equal(h.$('#lynis-detail [data-k="uyari"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu"); h.close();
  h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: lynApi({ "/api/server": srv(0) }) });
  await h.click("#srv-body [data-go-lynis-p]"); assert.equal(h.$('#lynis-detail [data-k="all"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$("#lyn-count").textContent, "3 / 3 bulgu"); h.close();
});

test("lynis: pencere yeniden boyutlanınca 250 ms sonra grafik yeniden çizilir (süzgeç korunur)", async () => {
  const h = await boot({ hash: "#v=lynis", overrides: lynApi() });
  await h.click('#lynis-pick [data-n="web1"]'); await h.click('#lynis-detail [data-k="uyari"]');
  const before = h.$(".chartbox svg"); h.win.dispatchEvent(new h.win.Event("resize")); h.timers.run(200); assert.equal(h.$(".chartbox svg"), before, "250 ms dolmadan çizilmez");
  h.timers.run(100); await h.settle(); assert.notEqual(h.$(".chartbox svg"), before); assert.equal(h.$('#lynis-detail [data-k="uyari"]').getAttribute("aria-pressed"), "true"); assert.equal(h.$("#lyn-count").textContent, "1 / 3 bulgu");
  assert.deepEqual(h.errors, []); h.close();
});
