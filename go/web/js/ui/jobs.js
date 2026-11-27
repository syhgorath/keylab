/* KeyLab — iş izleyici + çalıştırma penceresi: eylem onayı, canlı çıktı, parola/e-h istemleri, arka plana alma (paralel işler), alt çubuk/pill,
   Terminal modu, silme/görev/yükseltme modları. app.js'ten çıkarıldı (refaktör 4b), kod BİREBİR; değişenler yalnız dış bağlantılar:
   hooks (load, renderServer, showView, loadPackages, checkUpdatesUI, renderAddSteps, markTaskDone, getRemoval, openLastLog) ve dışa verilen
   setMode/setUpgradeTo (runMode/runUpgTo'ya dışarıdan atama yerine). Parola girişi tercihi (askMode/askAvail) Ayarlar ile paylaşıldığı için
   appState'tedir. Karakterizasyon testleri: go/webtest/runmodal*.test.js, jobs.test.js. */
import { $, $$, esc, api, json, toast } from "../lib/util.js";
import { appState } from "../state.js";
import { TAB_LABEL } from "../lib/labels.js";

// hooks: { load, renderServer, showView, loadPackages, checkUpdatesUI, renderAddSteps  — uygulama düzeyi yenileme/gezinme;
//          markTaskDone(anahtar) — Sunucu ekle sihirbazına "bu görev bitti" bilgisi; getRemoval() → {name, purge} — silme onayından;
//          openLastLog(paket, sunucu) — son günlüğü Günlükler sayfasında aç }
export function initJobs(hooks) {
  const { load, renderServer, showView, loadPackages, checkUpdatesUI, renderAddSteps, markTaskDone, getRemoval, openLastLog } = hooks;
  const pyGate = hooks.pyGate || (async () => true); // Python ön denetimi (js/ui/python.js): uygun Python yoksa pencere gösterir, eylem başlamaz

  /* ---------------- Eylem çalıştırma (salt okunur eylemler) ---------------- */
  const runm = $("#runmodal");
  let runJob = "", runPoll = 0, runTick = 0, runStart = 0, runSrv = "", runPkg = "", runAct = "", runItem = ""; // runItem: katalog öğesi eylemi (kapsam o öğeye göre)
  const runBusy = () => !!runJob;

  /* ---------------- İş izleyici: arka plana alma, üst çubuk iş düğmesi, sayfa yenilenince yeniden bağlanma ---------------- */
  // Sunucu tek iş çalıştırır (a.running). Pencere küçültülünce iş sürer; düğme çalışan/biten işi gösterir ve pencereyi geri açar.
  let pill = null; // { label, state: "run" | "ok" | "fail" }
  const runLabel = () => `${runAct} · ${TAB_LABEL[runPkg] || runPkg} · ${runSrv}`;
  function renderPill() {
    const el = $("#job-pill");
    if (!pill || runm.classList.contains("open")) { el.hidden = true; return; }
    el.hidden = false; el.className = "job-pill " + pill.state;
    const sec = Math.max(0, Math.round((Date.now() - runStart) / 1000));
    el.textContent = pill.state === "run" ? `⏳ ${pill.label} · ${sec} sn` : (pill.state === "ok" ? "✔ " : "✖ ") + pill.label;
    el.setAttribute("aria-label", el.textContent);
  }
  const showRunModal = () => { runm.classList.add("open"); renderPill(); setTimeout(() => ($("#run-cancel").hidden ? $("#run-close") : $("#run-cancel")).focus(), 40); };
  const minimizeRun = () => { runm.classList.remove("open"); renderPill(); };
  // başka eylem başlatmayı engelle: tek pencere durumu çalışan işe ait (sunucu da ikinci işi reddeder)
  // Farklı sunucularda işler paralel çalışır: pencereye bağlı iş (exec/write) başka sunucuda yeni iş başlatılırken arka plana alınır ve
  // alt çubuktan geri açılır. Aynı sunucuda (ve genel işlerde: kurulum, silme) tek iş; sunucu da aynı kuralı uygular.
  function jobBusy(server) {
    if (!runBusy()) return false;
    if (server && server !== runSrv && (runMode === "exec" || runMode === "write")) { detachCurrent(); return false; }
    toast(server && server === runSrv ? "Bu sunucuda bir eylem çalışıyor; bitmesini bekleyin ya da alt çubuktan açın." : "Bir eylem çalışıyor; bitmesini bekleyin ya da alt çubuktan açın.");
    showRunModal(); return true;
  }
  $("#job-pill").addEventListener("click", showRunModal);
  $("#run-min").addEventListener("click", minimizeRun);
  // sayfa yenilenmiş olabilir: sunucuda çalışan iş varsa yeniden bağlan (çıktı baştan okunur), pencere kapalı kalır
  const attachUi = w => {
    runMode = w.mode; runSrv = w.server; runPkg = w.pkg; runAct = w.act; runItem = w.item || "";
    $("#run-title").innerHTML = `<span class="mono">${esc(w.act)}</span> · ${esc(TAB_LABEL[w.pkg] || w.pkg)}${runItem ? ` · <b>${esc(runItem)}</b>` : ""}`;
    $("#run-sub").textContent = `Sunucu: ${w.server}${runItem ? ` · Öğe: ${runItem}` : ""}`;
    watchJob(w.id, w.started, { raw: w.raw, from: w.from });
  };
  const recFromSummary = j => ({ id: j.id, server: j.server, pkg: j.package, act: j.action, item: "", mode: "exec", started: Date.parse(j.started) || Date.now(), raw: "", from: 0 });
  // sayfa yenilenmiş olabilir: sunucuda çalışan işler varsa yeniden bağlan (çıktı baştan okunur). En yeni iş pencereye bağlanır (pencere
  // kapalı kalır), diğerleri arka planda izlenir; hepsi alt çubukta görünür.
  async function adoptJob() {
    try {
      const r = await json("/api/jobs");
      const live = (r.runningIds || (r.running ? [r.running] : [])).map(id => (r.jobs || []).find(x => x.id === id)).filter(j => j && !j.done);
      if (!live.length) return;
      live.forEach((j, i) => { const w = recFromSummary(j); if (i === 0 && !runBusy()) { attachUi(w); pill = { label: runLabel(), state: "run" }; renderPill(); } else if (!bg.has(w.id) && !(curWatch && curWatch.id === w.id)) { bg.set(w.id, w); startBg(); } });
    } catch (e) { /* iş yok ya da liste alınamadı */ }
  }

  /* ---- arka plan işleri: pencereye bağlı olmayan çalışan işler (farklı sunucularda paralel çalışanlar) ---- */
  const bg = new Map(); // id -> { id, server, pkg, act, item, mode, started, raw, from, idle, warned }
  let curWatch = null, bgTimer = 0;
  function detachCurrent() {
    if (!curWatch) return;
    const w = curWatch; curWatch = null;
    clearInterval(runTick); clearTimeout(runPoll); runJob = ""; pill = null; renderPill();
    $("#run-min").hidden = true; $("#run-input").hidden = true;
    bg.set(w.id, w); startBg();
    toast(`⏳ ${w.server}: ${w.act} arka planda sürüyor (alt çubuktan açın)`);
  }
  function startBg() { if (!bgTimer) bgTimer = setInterval(bgTick, 1500); }
  async function bgTick() {
    for (const w of Array.from(bg.values())) {
      try {
        const d = await json(`/api/run?id=${encodeURIComponent(w.id)}&from=${w.from}`);
        if (!bg.has(w.id)) continue; // bu arada pencereye bağlandı
        if (d.text) { w.raw += d.text; w.idle = 0; } else w.idle = (w.idle || 0) + 1;
        w.from = d.next;
        if (d.done) {
          bg.delete(w.id);
          toast((d.exit === 0 ? "✔ " : "✖ ") + `${w.server}: ${w.act}`);
          load(); if (appState.view === "server" && appState.srvName === w.server) { try { appState.srv = await json("/api/server?name=" + encodeURIComponent(w.server)); renderServer(); } catch (e) { /* sonraki turda */ } }
        } else if (w.idle >= 2 && !w.warned && w.raw && !/\n$/.test(w.raw.replace(/\r/g, ""))) {
          w.warned = true; toast(`⏳ ${w.server}: ${w.act} sizden yanıt bekliyor; alt çubuktan açın.`);
        }
      } catch (e) { /* sonraki turda */ }
    }
    if (!bg.size) { clearInterval(bgTimer); bgTimer = 0; }
  }
  // alt çubuktan: çalışan işi pencerede aç (başka iş bağlıysa o arka plana geçer)
  async function openJob(id) {
    if (curWatch && curWatch.id === id) { showRunModal(); return; }
    let w = bg.get(id);
    if (!w) {
      try { const r = await json("/api/jobs"); const j = (r.jobs || []).find(x => x.id === id && !x.done); if (!j) { toast("İş bitmiş; sonuç alt çubuktaki tabloda."); return; } w = recFromSummary(j); }
      catch (e) { toast("İş açılamadı: " + e.message); return; }
    }
    if (runBusy()) detachCurrent();
    bg.delete(id); attachUi(w); showRunModal();
  }

  let runMode = "exec", runUpgTo = ""; // runUpgTo: yükseltme modunda hedef sürüm (setUpgradeTo ile verilir)
  // Dışarıya açılan giriş: paket eylemleri (sunucu + paket verilmiş, "upgrade" hariç) önce Python denetiminden geçer; eski eşzamanlı davranış Python gerekmeyen/denetlenemeyen durumda aynen sürer.
  const needsPy = (server, pkg, action) => !!(server && pkg && action !== "upgrade");
  function gated(inner) { return (server, pkg, action, item) => { if (!needsPy(server, pkg, action)) return inner(server, pkg, action, item); if (jobBusy(server)) return; return pyGate(pkg, () => inner(server, pkg, action, item)).then(ok => ok && inner(server, pkg, action, item)); }; }
  function openRunBase(server, pkg, action, item) {
    if (jobBusy(server)) return;
    runMode = "exec"; $("#run-log").hidden = false; $("#run-cancel").hidden = false;
    $("#run-yes").hidden = $("#run-no").hidden = true;
    $("#run-confirm-extra").hidden = true; $("#run-confirm-extra").innerHTML = ""; $("#run-go").classList.remove("warnbtn", "dangerbtn"); $("#run-input").hidden = true;
    runSrv = server; runPkg = pkg; runAct = action; runJob = ""; runItem = item || "";
    $("#run-title").innerHTML = `<span class="mono">${esc(action)}</span> · ${esc(TAB_LABEL[pkg] || pkg)}${runItem ? ` · <b>${esc(runItem)}</b>` : ""}`;
    $("#run-confirm-note").textContent = "Çalıştırma uygulamanın kendi motoruyla yapılır; kapılar, günlük ve sonuç kaydı menüdekiyle aynıdır. Parola sorulmaz: ssh anahtarınız ssh-agent'ta yüklü olmalı.";
    $("#run-sub").textContent = `Sunucu: ${server}${runItem ? ` · Öğe: ${runItem}` : ""}`;
    $("#run-confirm-text").innerHTML = `<b>${esc(server)}</b> sunucusuna ssh ile bağlanılacak ve <span class="mono">${esc(action)}</span> <b>salt okunur</b> çalıştırılacak: sunucuda hiçbir şey değişmez.`;
    $("#run-confirm-note").textContent = "Çalıştırma uygulamanın kendi motoruyla yapılır; kapılar, günlük ve sonuç kaydı menüdekiyle aynıdır. Parola sorulmaz: ssh anahtarınız ssh-agent'ta yüklü olmalı.";
    $("#run-confirm").hidden = false; $("#run-live").hidden = true;
    $("#run-go").hidden = false; $("#run-go").disabled = false; $("#run-go").textContent = "Çalıştır";
    $("#run-close").hidden = false; $("#run-close").textContent = "Vazgeç";
    runm.classList.add("open"); setTimeout(() => $("#run-go").focus(), 40);
  }
  function closeRun() { if (runBusy()) return; runm.classList.remove("open"); pill = null; renderPill(); }

  // ilerleme çubuğu genişliği: satır içi stil özniteliği yerine data-w + CSSOM (CSP style-src-attr 'none' ile uyumlu)
  const applyBars = root => $$("i[data-w]", root).forEach(i => { i.style.width = (parseInt(i.dataset.w, 10) || 0) + "%"; });

  const refreshServer = async server => {
    const r = await json("/api/server?name=" + encodeURIComponent(server));
    if (appState.view === "server" && appState.srvName === server) { appState.srv = r; renderServer(); }
    load();
    return r;
  };

  /* Değiştiren bir eylem başarıyla bitince ayar kartlarının okumaları kendiliğinden koşar; kullanıcı ayrıca "Kontrol et" demek zorunda kalmaz.
     Yalnız arayüzün doğrudan çalıştırabildiği okumalar (salt okunur, sudo'suz, onaysız, önkoşulsuz): parola isteyen okuma elle kalır.
     Kısa bekleme, yeniden başlatılan hizmetin (ör. chrony) toparlanması içindir. Sunucu aynı anda tek iş çalıştırdığından sırayla koşarlar.
     Sağlık özeti hâlâ uyarı veriyorsa (hizmet henüz toparlanmamış olabilir) yalnız o özetin okuması RETRY_MS aralıklarıyla yeniden koşar. */
  const SETTLE_MS = 5000, RETRY_MS = [15000, 25000], READ_POLL_MS = 700, READ_POLL_MAX = 200;
  const autoReading = new Map(); // "sunucu|paket" -> sıra belirteci: yeni bir uygulama bitince eski sıra bırakılır
  const wait = ms => new Promise(done => setTimeout(done, ms));
  function cardReads(p) {
    const wanted = new Set([...(p.settings || []), ...(p.settingCards || [])].map(x => x.readAction).filter(Boolean));
    return (p.actions || []).filter(a => wanted.has(a.id) && a.runnable && !(a.effects || []).length).map(a => a.id);
  }
  // verilen okumalardan hangilerinin sağlık özeti uyarı veriyor
  const ailing = (p, actions) => actions.filter(id => ((p && p.settingCards) || []).some(c => c.readAction === id && (c.health || []).some(h => h.state === "warn")));
  async function readOnce(server, pkg, action) {
    const { id } = await json("/api/run", { server, package: pkg, action });
    let d = { next: 0 };
    for (let i = 0; !d.done; i++) {
      if (i >= READ_POLL_MAX) throw new Error("zaman aşımı");
      await wait(READ_POLL_MS);
      d = await json(`/api/run?id=${encodeURIComponent(id)}&from=${d.next}`);
    }
    return d.exit;
  }
  // zincirin okumadığı kartlardan uygulama yüzünden eskiyenler ("güncel olmayabilir")
  const outdated = (p, actions) => actions.filter(id => [...(p.settings || []), ...(p.settingCards || [])].some(x => x.readAction === id && x.stale));
  async function autoRead(server, pkg, actions) {
    const key = server + "|" + pkg, token = {};
    autoReading.set(key, token);
    const dropped = () => autoReading.get(key) !== token;
    try {
      let todo = actions, delay = SETTLE_MS;
      for (let round = 0; ; round++) {
        await wait(delay);
        if (dropped()) return;
        toast(`${server}: durum okunuyor…`);
        let failed = 0;
        for (const action of todo) {
          if (await readOnce(server, pkg, action) !== 0) failed++;
          if (dropped()) return;
        }
        const r = await refreshServer(server);
        if (failed) { toast(`${server}: durum okunamadı; ayrıntı Günlükler sayfasında`); return; }
        todo = ailing((r.packages || []).find(x => x.id === pkg), todo);
        if (!todo.length) { toast(`${server}: ✔ durum güncellendi`); return; }
        if (round >= RETRY_MS.length) { toast(`${server}: durum güncellendi; sağlık özetinde uyarı sürüyor`); return; }
        delay = RETRY_MS[round];
        toast(`${server}: sağlık henüz toparlanmadı; birazdan yeniden okunacak`);
      }
    } catch (err) {
      if (!dropped()) toast(`${server}: durum kendiliğinden okunamadı (${err.message}); "Kontrol et" ile okuyun`);
    } finally { if (!dropped()) autoReading.delete(key); }
  }

  /* Paket eylemleri ayrı onay penceresi olmadan doğrudan gömülü terminalde açılır: sudo parolası ve motorun onayları (değiştiren eylemde
     e/h, yıkıcı eylemde sunucu adı) orada yazılır; sunucu ve yürütücü kapıları yeniden denetler. Ayar kartlarının "Kur" zinciri ayrıca
     önizleme → eylem → kontrol adımlarını tek koşuda yürütür. */
  const chained = new Set(); // "sunucu|paket|eylem": biten görev zincirse kontrolü zaten koşmuştur
  const starting = new Set(); // açılmakta olan görevler: çift tıklama ikinci istek göndermez
  async function startDirect(server, pkg, action, chain, item) {
    const key = `${server}|${pkg}|${action}`;
    if (starting.has(key)) return;
    starting.add(key);
    if (chain) chained.add(key); else chained.delete(key);
    let started;
    try { started = await window.KeyLabTerminal.openTask({ server, package: pkg, action, ...(item ? { item } : {}), ...(chain ? { chain: true } : {}) }); }
    finally { starting.delete(key); }
    if (!started.ok) { chained.delete(key); toast("Eylem başlatılamadı: " + started.error); return; }
    autoReading.delete(`${server}|${pkg}`); // bekleyen arka plan okuması yeni eylemle çakışmasın; bitince yeniden kurulur
    toast(`${server}: ${action} terminalde çalışıyor — parola ve onayları orada yazın`);
  }

  // Sunucu hazırlık görevi (anahtar üret/gönder, ajana ekle, Keychain, knock): motorun soruları terminalde yanıtlanır
  async function openServerTask(server, task, hint) {
    const started = await window.KeyLabTerminal.openTask({ server, task });
    if (!started.ok) { toast("Eylem başlatılamadı: " + started.error); return; }
    toast(hint ? `${server}: ${hint}` : `${server}: ${task} terminalde çalışıyor — parola ve onayları orada yazın`);
  }
  // Sunucu silme: onay penceresinden sonra motor terminalde koşar; hata (ör. çalışan eylem) onay penceresinde gösterilsin diye sonuç döner
  const openRemoval = (name, confirm, ack) => window.KeyLabTerminal.openTask({ server: name, task: "remove", confirm, ack });

  async function serverTaskDone(d) {
    if (d.action === "remove") {
      const wanted = getRemoval(), purge = wanted.name === d.server && wanted.purge;
      if (d.code === 0 && purge) { try { await json("/api/server-purge", { name: d.server }); } catch (e) { toast("Günlükler silinemedi: " + e.message); } }
      await load();
      if (d.code !== 0) { toast(`${d.server}: ✖ silme başarısız (çıkış kodu ${d.code}); ayrıntı terminal sekmesinde`); return; }
      appState.srvName = ""; appState.srv = null; showView("servers");
      toast(`${d.server} silindi`);
      return;
    }
    if (d.code === 0) markTaskDone(d.server + ":" + d.action);
    await load(); renderAddSteps();
    if (appState.view === "server" && appState.srvName === d.server) { try { await refreshServer(d.server); } catch (e) { /* sonraki otomatik yenileme alır */ } }
    toast(d.code === 0 ? `${d.server}: ✔ ${d.action} tamamlandı` : `${d.server}: ✖ ${d.action} başarısız (çıkış kodu ${d.code}); ayrıntı terminal sekmesinde`);
  }

  // görev terminali bitince: sonuç kartları yenilenir ve sonuç bildirilir; modal gerekmez
  window.addEventListener("keylab:taskexit", async e => {
    const d = e.detail || {}; if (!d.server || !d.action) return;
    if (!d.pkg) { await serverTaskDone(d); return; }
    let r = null;
    try { r = await refreshServer(d.server); } catch (err) { /* sunucu yanıt vermezse sayfa bir sonraki yenilemede toparlanır */ }
    const label = `${d.action} · ${TAB_LABEL[d.pkg] || d.pkg}`;
    toast(d.code === 0 ? `${d.server}: ✔ ${label} tamamlandı` : `${d.server}: ✖ ${label} başarısız (çıkış kodu ${d.code}); ayrıntı terminal sekmesinde`);
    const chain = chained.delete(`${d.server}|${d.pkg}|${d.action}`);
    const p = r && (r.packages || []).find(x => x.id === d.pkg), a = p && (p.actions || []).find(x => x.id === d.action);
    if (d.code !== 0 || !a || !(a.effects || []).length) return;
    // zincir yalnız eylemin kartlarını okudu: burada eskiyen diğer okumalar ve toparlanmamış sağlık özeti sessizce yenilenir
    let reads = cardReads(p);
    if (chain) { const again = new Set([...outdated(p, reads), ...ailing(p, reads)]); reads = reads.filter(id => again.has(id)); }
    if (reads.length) autoRead(d.server, d.pkg, reads);
  });

  // ham terminal çıktısı → okunur metin: ANSI kaçışlarını at, "\r" ile satır başına dönmeyi uygula
  function cleanTerm(t) {
    t = t.replace(/\x1b\[[0-9;?]*[ -\/]*[@-~]/g, "").replace(/\x1b\][^\x07]*\x07/g, "").replace(/\^D|\x08/g, "");
    return t.split("\n").map(l => { let out = ""; for (const p of l.split("\r")) if (p.length) out = p + out.slice(p.length); return out; }).join("\n");
  }
  const lineHtml = l => { const c = /fatal:|FAILED|failed=[1-9]|HATA|ERROR/.test(l) ? "err" : /^ok:|^changed:/.test(l) ? "ok" : /^TASK |^PLAY /.test(l) ? "t" : ""; return `<div${c ? ` class="${c}"` : ""}>${esc(l) || " "}</div>`; };

  // SSHPOCKET_PROGRESS satırlarından alan alan ilerleme + geri alma güvenlik ağı durumu
  function progressHtml(txt) {
    const prog = [...txt.matchAll(/SSHPOCKET_PROGRESS alan=(\S+) sira=(\d+)\/(\d+) durum=(\S+)/g)];
    if (!prog.length) return "";
    let total = 0, done = new Set(), cur = "";
    prog.forEach(x => { total = +x[3]; if (x[4] === "bitti") done.add(x[1]); else cur = x[1]; });
    if (done.has(cur)) cur = "";
    const armed = /TASK \[hardening_rollback : Arm[^\]]*\][^\n]*\n(ok|changed):/.test(txt), disarmed = /TASK \[hardening_rollback : Disarm[^\]]*\][^\n]*\n(ok|changed):/.test(txt);
    return `<div class="run-meta"><span>${done.size} / ${total} alan tamam${cur ? ` · şu an: <b>${esc(cur)}</b>` : ""}</span></div>
      <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done.size}"><i data-w="${Math.round(done.size / total * 100)}"></i></div>` +
      (armed && !disarmed ? '<div class="notice sj-d2c171"><span class="ico" aria-hidden="true">⏱</span><div><b>Geri alma güvenlik ağı devrede.</b> Erişim doğrulanınca kaldırılır; doğrulanamazsa önceki hâl geri yüklenir.</div></div>' : "") +
      (disarmed ? '<div class="small muted sj-8a77e5">Geri alma güvenlik ağı kaldırıldı (erişim doğrulandı).</div>' : "");
  }

  // çalıştırma penceresinden yalnız paket yükseltme başlatılır (USB'ye yazar; sunucuya dokunmaz)
  async function startRun() {
    $("#run-go").disabled = true; askedAt = askCancelledAt = -1; askBusy = false;
    let r;
    try { r = await json("/api/upgrade", { package: runPkg, version: runUpgTo, ack: true }); }
    catch (e) { $("#run-go").disabled = false; toast("Çalıştırılamadı: " + e.message); return; }
    watchJob(r.id);
  }

  function watchJob(id, startedAt, resume) {
    runJob = id; runStart = startedAt || Date.now(); pill = { label: runLabel(), state: "run" }; $("#run-min").hidden = false;
    const w = curWatch = { id, server: runSrv, pkg: runPkg, act: runAct, item: runItem, mode: runMode, started: runStart, raw: (resume && resume.raw) || "", from: (resume && resume.from) || 0, idle: 0 };
    $("#run-confirm").hidden = true; $("#run-live").hidden = false; $("#run-go").hidden = true; $("#run-close").hidden = true;
    $("#run-log").innerHTML = ""; $("#run-log").hidden = false; $("#run-progress").innerHTML = ""; $("#run-result").innerHTML = ""; $("#run-input").hidden = true;
    $("#run-cancel").hidden = false; $("#run-cancel").disabled = false;
    const logEl = $("#run-log");
    const paintLog = () => {
      const stick = logEl.scrollTop + logEl.clientHeight >= logEl.scrollHeight - 30;
      const txt = cleanTerm(w.raw);
      logEl.innerHTML = txt.split("\n").map(lineHtml).join("");
      if (stick) logEl.scrollTop = logEl.scrollHeight;
      $("#run-progress").innerHTML = progressHtml(txt); applyBars($("#run-progress"));
    };
    const tick = () => { $("#run-state").innerHTML = `<span class="spinner" aria-hidden="true"></span> çalışıyor… ${Math.round((Date.now() - runStart) / 1000)} sn`; renderPill(); };
    tick(); runTick = setInterval(tick, 1000);
    if (w.raw) paintLog(); // arka plandan geri açıldı: o ana kadarki çıktı hemen görünsün
    const poll = async () => {
      if (curWatch !== w) return; // arka plana alındı
      try {
        const d = await json(`/api/run?id=${encodeURIComponent(w.id)}&from=${w.from}`);
        if (curWatch !== w) return;
        if (d.text) { w.raw += d.text; w.idle = 0; paintLog(); } else w.idle++;
        w.from = d.next;
        if (d.done) { $("#run-input").hidden = true; curWatch = null; finishRun(d, cleanTerm(w.raw)); return; }
        // istem: çıktı satır sonuyla bitmiyor ve bir süredir yeni bayt gelmedi → girdi kutusunu göster
        const txt = cleanTerm(w.raw);
        if (w.idle >= 2 && w.raw && !/\n$/.test(w.raw.replace(/\r/g, ""))) {
          const prompt = txt.slice(txt.lastIndexOf("\n") + 1).trim();
          if (prompt) showInput(prompt, w.raw.length);
        } else if (w.idle < 2) $("#run-input").hidden = true;
      } catch (e) { if (curWatch !== w) return; clearInterval(runTick); runJob = ""; curWatch = null; pill = null; $("#run-min").hidden = true; renderPill(); showRunModal(); $("#run-state").textContent = "Bağlantı koptu: " + e.message; $("#run-close").hidden = false; $("#run-close").textContent = "Kapat"; return; }
      runPoll = setTimeout(poll, 600);
    };
    poll();
  }

  // parola girişi: "system" = işletim sisteminin kendi penceresi (tarayıcıya hiç uğramaz), "inline" = sayfa içi kutu
  let askBusy = false, askedAt = -1, askCancelledAt = -1;
  try { appState.askMode = localStorage.getItem("sshpocket.askpass") === "inline" ? "inline" : "system"; } catch (e) { /* depolama kapalı olabilir */ }
  const maskSupported = typeof CSS !== "undefined" && CSS.supports && CSS.supports("-webkit-text-security", "disc");

  async function askNative(prompt, rawLen) {
    askBusy = true; $("#run-input").hidden = true;
    $("#run-state").innerHTML = '<span class="spinner" aria-hidden="true"></span> parola için sistem penceresi açıldı…';
    try { await json("/api/run/askpass", { id: runJob, prompt }); askedAt = rawLen; }
    catch (e) { askCancelledAt = rawLen; toast("Parola penceresi: " + e.message + " — sayfa içi kutu açıldı"); }
    askBusy = false;
  }

  // motor bir istem bekliyor (parola, e/h, ad onayı): kullanıcı yanıtı yazar
  function showInput(prompt, rawLen) {
    const secret = /pass(word|phrase)|parola|şifre|sifre/i.test(prompt);
    if (secret && appState.askMode === "system" && appState.askAvail && askCancelledAt !== rawLen) {
      if (!askBusy && askedAt !== rawLen) askNative(prompt, rawLen);   // aynı çıktı uzunluğunda yeniden sorma; yanlış parolada Ansible yeni çıktı üretir
      return;
    }
    const box = $("#run-input"), inp = $("#run-in");
    if (!runm.classList.contains("open")) { showRunModal(); toast("İş sizden yanıt bekliyor."); }
    if (!box.hidden && $("#run-prompt").textContent === prompt) return;
    $("#run-prompt").textContent = prompt;
    const qa = /\[e\/h\]|\(e\/h\)/i.test(prompt) ? ["e", "H"] : /\(yes\/no/i.test(prompt) ? ["yes", "no"] : /\(y\/n\)|\[y\/n\]/i.test(prompt) ? ["y", "n"] : null;
    $("#run-yes").hidden = $("#run-no").hidden = !qa;
    if (qa) { $("#run-yes").dataset.v = qa[0]; $("#run-no").dataset.v = qa[1]; $("#run-yes").textContent = `Evet (${qa[0]})`; $("#run-no").textContent = `Hayır (${qa[1]})`; }
    // sayfa içi parola kutusu: Chrome'un şifre önerisini/parola yöneticisini tetiklememek için type=text + noktalı maske (destek yoksa password)
    inp.classList.toggle("mask-disc", secret && maskSupported);
    inp.type = secret && !maskSupported ? "password" : "text"; inp.value = "";
    inp.setAttribute("autocomplete", "off"); inp.setAttribute("name", "yanit-" + Math.random().toString(36).slice(2)); inp.setAttribute("data-lpignore", "true"); inp.setAttribute("data-1p-ignore", "true");
    box.hidden = false; inp.focus();
  }
  async function sendInput() {
    const inp = $("#run-in"), text = inp.value;
    inp.value = ""; $("#run-input").hidden = true;      // gönderince hemen temizle ve gizle
    try { await json("/api/run/input", { id: runJob, text }); }
    catch (e) { toast("Gönderilemedi: " + e.message); }
  }
  $("#run-send").addEventListener("click", sendInput);
  const quick = async t => { $("#run-input").hidden = true; try { await json("/api/run/input", { id: runJob, text: t }); } catch (e) { toast("Gönderilemedi: " + e.message); } };
  $("#run-yes").addEventListener("click", () => quick($("#run-yes").dataset.v || "e")); $("#run-no").addEventListener("click", () => quick($("#run-no").dataset.v || "H"));
  $("#run-in").addEventListener("keydown", e => { if (e.key === "Enter") { e.preventDefault(); sendInput(); } });

  async function finishRun(d, txt) {
    clearInterval(runTick); clearTimeout(runPoll); runJob = "";
    $("#run-cancel").hidden = true; $("#run-min").hidden = true;
    const ok = d.exit === 0, secs = Math.round((Date.now() - runStart) / 1000);
    pill = { label: runLabel(), state: ok ? "ok" : "fail" }; if (!runm.classList.contains("open")) toast((ok ? "✔ " : "✖ ") + runLabel()); renderPill();
    $("#run-state").innerHTML = ok ? `<span class="res ok">✔ başarılı</span> · ${secs} sn` : `<span class="res fail">✖ başarısız</span> · çıkış kodu ${d.exit} · ${secs} sn`;
    // sonuç özeti: motorun "Sonuç:" bölümündeki anahtar=değer satırları
    const all = (txt || "").split("\n");
    const i = all.lastIndexOf("Sonuç:");
    if (i >= 0) {
      const chips = all.slice(i + 1).filter(l => /^\s{2}[A-Za-z0-9_]+=/.test(l)).map(l => `<span class="chip">${esc(l.trim())}</span>`).join("");
      if (chips) $("#run-result").innerHTML = `<div class="small muted sj-4e420a">Sonuç</div><div class="chips">${chips}</div>`;
    }
    if (!ok) $("#run-result").insertAdjacentHTML("beforeend", `<div class="notice danger sj-d2c171"><span class="ico" aria-hidden="true">✕</span><div>Eylem başarısız oldu. Ayrıntı için yukarıdaki çıktıya bakın; tam günlük <b>Günlükler</b> sayfasında.</div></div>`);
    $("#run-close").hidden = false; $("#run-close").textContent = "Kapat";
    // kartları yenile (sonuç dosyaları motor tarafından yazıldı)
    if (runMode === "upgrade") { if (appState.view === "packages") { await loadPackages(); checkUpdatesUI(); } load(); return; }
    try { appState.srv = await json("/api/server?name=" + encodeURIComponent(runSrv)); if (appState.view === "server") renderServer(); load(); } catch (e) { /* bir sonraki turda yenilenir */ }
  }
  $("#run-go").addEventListener("click", startRun);
  $("#run-close").addEventListener("click", closeRun);
  $("#run-cancel").addEventListener("click", async () => { $("#run-cancel").disabled = true; try { await json("/api/run/cancel", { id: runJob }); } catch (e) { toast("İptal edilemedi: " + e.message); } });

  const openAction = gated((server, pkg, action, item) => startDirect(server, pkg, action, false, item));
  // openRun: paket eylemi terminalde; paket eylemi olmayan tek kullanım paket yükseltmedir (çalıştırma penceresi)
  const openRun = (server, pkg, action, item) => (needsPy(server, pkg, action) ? openAction(server, pkg, action, item) : openRunBase(server, pkg, action, item));
  const openWrite = openAction, openTerm = openAction, openCheck = openAction;
  const openChain = gated((server, pkg, action) => startDirect(server, pkg, action, true));
  return {
    openRun, openWrite, openTerm, openChain, openCheck, openServerTask, openRemoval, jobBusy, watchJob, closeRun, openJob, adoptJob,
    setMode: m => { runMode = m; },          // openRun'dan sonra çağıran akışlar (silme, görev, yükseltme) kipi belirler
    setUpgradeTo: v => { runUpgTo = v; },
  };
}
