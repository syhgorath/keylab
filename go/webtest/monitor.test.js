"use strict";
/* Kaynaklar sayfası (js/ui/monitor.js): sayfa açılınca ölçer, 2 sn'de bir yeniler, kapanınca durur; süreç tablosu, bellek/CPU kutuları, disk çubukları. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const SAMPLE = (o = {}) => ({ pid: 1, uptimeSec: 3725, cores: 8, memTotalMB: 16384, cpu: 12.5, rssMB: 120, selfRssMB: 80, goroutines: 20, heapMB: 5, terms: 1, tasks: 2,
  procs: [{ pid: 3, name: "python3.13", kind: "python", cpu: 10, rssMB: 38, elapsedSec: 5 }, { pid: 1, name: "keylab", kind: "self", cpu: 2.5, rssMB: 80, elapsedSec: 3725 }, { pid: 2, name: "ssh-agent", kind: "agent", cpu: 0, rssMB: 2, elapsedSec: 60 }],
  dirs: [{ id: "servers", path: "/x/servers", bytes: 2048, temp: false }, { id: "cache", path: "/tmp/keylab-1/cache", bytes: 94371840, temp: true }, { id: "state", path: "/x/state", bytes: 0, temp: false }],
  volume: { path: "/x/servers", totalBytes: 1000000000000, freeBytes: 400000000000 }, ...o });
const base = (m, o = {}) => ({ "/api/home": () => fixture("home"), "/api/monitor": m, ...o });
const mon = h => h.calls.filter(c => c.path === "/api/monitor").length;

test("kaynaklar: sayfa açılmadan ölçüm yapılmaz; Kaynaklar'a girince hemen ölçer, 2 sn'de bir yeniler, başka sayfaya geçince durur", async () => {
  const h = await boot({ hash: "", overrides: base(SAMPLE) });
  assert.equal(mon(h), 0, "açılmadan ölçüm yok");
  await h.click('.nav-item[data-nav="monitor"]'); await h.settle();
  assert.equal(h.$(".view:not([hidden])").id, "view-monitor"); assert.equal(mon(h), 1);
  h.timers.run(2000); await h.settle(); h.timers.run(2000); await h.settle(); assert.equal(mon(h), 3, "2 sn'de bir");
  await h.click('.nav-item[data-nav="dash"]'); const n = mon(h); h.timers.run(10000); await h.settle();
  assert.equal(mon(h), n, "sayfa kapanınca ölçüm durur"); assert.deepEqual(h.errors, []); h.close();
});

test("kaynaklar: CPU/RAM/disk kutuları, süreç tablosu (CPU'ya göre sıralı, türler çevrilmiş) ve geçici klasör işareti", async () => {
  const h = await boot({ hash: "#v=monitor", overrides: base(SAMPLE) });
  assert.equal(h.$("#mon-cpu").textContent, "12.5%"); assert.match(h.$("#mon-cpu-sub").textContent, /tüm çekirdeklerin 1\.6%'i \(8 çekirdek\)/);
  assert.equal(h.$("#mon-ram").textContent, "120 MB"); assert.match(h.$("#mon-ram-sub").textContent, /bilgisayarın belleğinin 0\.73%'i \(16 GB\)/);
  assert.equal(h.$("#mon-disk").textContent, "90 MB"); assert.match(h.$("#mon-disk-sub").textContent, /birimde boş: 373 GB \/ 931 GB/);
  assert.match(h.$("#mon-up").textContent, /1sa 2dk önce açıldı · 1 terminal, 2 eylem çalışıyor · 20 goroutine/);
  const rows = h.$$("#mon-procs tr").map(r => [...r.children].map(c => c.textContent));
  assert.deepEqual(rows, [["python3.13", "Python / Ansible", "3", "10.0%", "38 MB", "5sn"], ["keylab", "KeyLab (ana süreç)", "1", "2.5%", "80 MB", "1sa 2dk"], ["ssh-agent", "Özel ssh-agent", "2", "0.0%", "2 MB", "1dk 0sn"]]);
  assert.equal(h.$("#mon-procs-count").textContent, "3 süreç");
  const dirs = h.$$("#mon-dirs li"); assert.equal(dirs.length, 3);
  assert.match(dirs[1].textContent, /Önbellek \(Python ortamı\) · geçici — çıkışta silinir.*90 MB/); assert.doesNotMatch(dirs[0].textContent, /geçici/);
  const bar = i => dirs[i].querySelector(".mon-track i"); assert.equal(bar(1).style.width, "100%", "en büyük klasör tam çubuk (CSSOM, satır içi stil özniteliği değil)"); assert.equal(bar(0).dataset.w, "0"); assert.equal(bar(2).dataset.w, "0");
  assert.ok(h.$("#mon-cpu-spark polyline").getAttribute("points").length > 0, "geçmiş çizgisi çizildi");
  assert.deepEqual(h.errors, []); h.close();
});

test("kaynaklar: geçmiş çizgisi örnek geldikçe uzar (en çok 60); ölçüm notu gösterilir; sunucu hatasında sayfa çökmez", async () => {
  let n = 0;
  const h = await boot({ hash: "#v=monitor", overrides: base(() => (++n === 3 ? { status: 500, body: "ölçüm hatası" } : SAMPLE({ cpu: n * 10, note: n === 2 ? "süreç tablosu okunamadı: ps yok" : "" }))) });
  const pts = () => h.$("#mon-cpu-spark polyline").getAttribute("points").split(" ").length;
  assert.equal(pts(), 1); h.timers.run(2000); await h.settle(); assert.equal(pts(), 2); assert.equal(h.$("#mon-note").textContent, "süreç tablosu okunamadı: ps yok");
  h.timers.run(2000); await h.settle(); assert.match(h.$("#mon-note").textContent, /ölçüm hatası/, "hata notta gösterilir"); assert.equal(pts(), 2, "hatalı örnek geçmişe eklenmez");
  for (let i = 0; i < 70; i++) { h.timers.run(2000); await h.settle(); } assert.equal(pts(), 60, "geçmiş 60 ile sınırlı");
  h.close();
});

test("kaynaklar: İngilizce arayüzde çevrilir", async () => {
  const h = await boot({ hash: "#v=monitor", lang: "en", overrides: base(SAMPLE) });
  assert.equal(h.$('.nav-item[data-nav="monitor"] span:last-child').textContent, "Resources"); assert.equal(h.$("#view-monitor .card-head h2").textContent, "Running now");
  assert.equal(h.$$("#mon-procs tr")[0].children[1].textContent, "Python / Ansible"); assert.match(h.$("#mon-up").textContent, /KeyLab started 1h 2m ago · 1 terminal\(s\), 2 action\(s\) running/); assert.equal(h.$$("#mon-procs tr")[1].children[5].textContent, "1h 2m");
  h.close();
});
