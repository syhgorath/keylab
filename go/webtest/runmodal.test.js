"use strict";
/* PAKET EYLEMLERİ GÖMÜLÜ TERMİNALDE: değiştiren ve sudo isteyen eylemler ayrı onay penceresi olmadan doğrudan görev terminalinde açılır;
   sudo parolası ve motorun onayları (e/h, yıkıcı eylemde sunucu adı) terminalde yazılır (kapılar sunucuda ve yürütücüde).
   Çalıştırma penceresinin kalan davranışı (parolasız okuma, sunucu görevleri, yükseltme, silme): runmodal2.test.js ve jobs.test.js. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");
const { mixedHome, status } = require("./lib/fakejobs");

const ov = extra => ({ "/api/home": mixedHome, "/api/status": status, ...extra });
const isOpen = (h, s) => h.$(s).classList.contains("open");
const serverOv = mut => ({ "/api/server": u => { const d = fixture("server-" + u.searchParams.get("name")); if (mut) mut(d, u.searchParams.get("name")); return d; } });
const act = (d, pkg, id) => d.packages.find(p => p.id === pkg).actions.find(a => a.id === id);
const WRITE = '#srv-body [data-write="linux-hardening|audit"]';
const TERM = '#srv-body [data-term="linux-hardening|rollback_status"]';
const taskOv = tasks => ({ "/api/term/task": (u, { body }) => { tasks.push(JSON.parse(JSON.stringify(body))); return { id: "k1", kind: "task", title: "görev" }; }, "/api/term/close": { ok: true }, "/api/term/resize": { ok: true }, "/api/term/input": { ok: true } });
const SZ = { rows: 24, cols: 80 };
const start = async (sel, extra = {}) => {
  const tasks = []; const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov({ ...taskOv(tasks), ...extra }) });
  await h.click(sel); return { h, tasks };
};

test("değiştiren eylem: onay penceresi yok; yalnız sunucu, paket ve eylemle terminal görevi açılır", async () => {
  const { h, tasks } = await start(WRITE);
  assert.deepEqual(tasks, [{ server: "web1", package: "linux-hardening", action: "audit", ...SZ }], "arayüz onay/snapshot bilgisi göndermez");
  assert.ok(isOpen(h, "#termmodal")); assert.ok(!isOpen(h, "#runmodal")); assert.equal(h.$("#w-snap"), null);
  assert.equal(h.$("#term-title").textContent, "⚙ linux-hardening · audit");
  assert.match(h.$("#toast").textContent, /web1: audit terminalde çalışıyor — parola ve onayları orada yazın/);
  assert.equal(h.calls.filter(c => c.path === "/api/run").length, 0); assert.deepEqual(h.errors, []); h.close();
});

test("yıkıcı eylem de doğrudan terminalde başlar: onay (sunucu adı) arayüzde değil terminalde", async () => {
  const { h, tasks } = await start('#srv-body [data-write="linux-hardening|rollback_now"]', serverOv(d => { act(d, "linux-hardening", "rollback_now").destructive = true; }));
  assert.deepEqual(tasks.map(t => t.action), ["rollback_now"]); assert.equal(tasks[0].confirm, undefined);
  assert.ok(!isOpen(h, "#runmodal")); assert.deepEqual(h.errors, []); h.close();
});

test("sudo isteyen okuma: doğrudan terminal; görev bitince sonuç yenilenir ve bildirilir; eksik olay yok sayılır", async () => {
  let phase = 0;
  const { h, tasks } = await start(TERM, serverOv(d => { if (phase) { const a = act(d, "linux-hardening", "rollback_status"); a.lastTime = "2026-11-14T10:00:05Z"; a.lastResult = "ok"; } }));
  assert.deepEqual(tasks.map(t => t.action), ["rollback_status"]);
  const srvCalls = () => h.calls.filter(c => c.path === "/api/server?name=web1").length;
  const n0 = srvCalls(); h.win.dispatchEvent(new h.win.CustomEvent("keylab:taskexit", { detail: { server: "web1" } })); await h.settle();
  assert.equal(srvCalls(), n0, "eksik alanlı olay yok sayılır");
  phase = 1; h.win.__esObj[0].emit("exit", "0"); await h.settle();
  assert.ok(srvCalls() > n0); assert.match(h.$("#toast").textContent, /✔ rollback_status · Hardening tamamlandı/); assert.deepEqual(h.errors, []); h.close();
});

test("görev başarısız biterse ve açılamazsa söylenir; boş terminal penceresi kalmaz", async () => {
  const a = await start(TERM);
  a.h.win.__esObj[0].emit("exit", "4"); await a.h.settle();
  assert.match(a.h.$("#toast").textContent, /✖ rollback_status · Hardening başarısız \(çıkış kodu 4\)/); a.h.close();
  const b = await start(TERM, { "/api/term/task": { status: 409, body: "bu sunucuda zaten çalışan bir eylem var" } });
  assert.equal(b.h.$("#toast").textContent, "Eylem başlatılamadı: bu sunucuda zaten çalışan bir eylem var");
  assert.ok(!isOpen(b.h, "#termmodal")); assert.ok(!isOpen(b.h, "#runmodal")); assert.deepEqual(b.h.errors, []); b.h.close();
});

test("tek tık tek görev: sayfa art arda çizilse de ve düğmeye iki kez basılsa da ikinci istek gitmez", async () => {
  const tasks = []; const h = await boot({ hash: "#s=web1&p=linux-hardening", overrides: ov(taskOv(tasks)) });
  const click = el => el.dispatchEvent(new h.win.MouseEvent("click", { bubbles: true }));
  // sayfayı aynı turda iki kez çizdir (sekmeye art arda basmak gibi)
  click(h.$('#srv-tabs [data-p="linux-hardening"]')); click(h.$('#srv-tabs [data-p="linux-hardening"]')); await h.settle();
  await h.click(WRITE); assert.equal(tasks.length, 1, "tek tık tek görev");
  tasks.length = 0; h.win.__esObj[0].emit("exit", "0"); await h.settle();
  const b = h.$(TERM); click(b); click(b); await h.settle();
  assert.equal(tasks.length, 1, "çift tıklama"); assert.deepEqual(h.errors, []); h.close();
});
