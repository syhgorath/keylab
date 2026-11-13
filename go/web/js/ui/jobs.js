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

  let runMode = "exec", termTimer = 0, runUpgTo = ""; // runUpgTo: yükseltme modunda hedef sürüm (setUpgradeTo ile verilir)
  function openRun(server, pkg, action, item) {
    if (jobBusy(server)) return;
    runMode = "exec"; $("#run-log").hidden = false; $("#run-cancel").hidden = false;
    $("#run-yes").hidden = $("#run-no").hidden = true;
    $("#run-confirm-extra").hidden = true; $("#run-confirm-extra").innerHTML = ""; $("#run-go").classList.remove("warnbtn", "dangerbtn"); $("#run-ext").hidden = true; $("#run-ext").disabled = false; $("#run-input").hidden = true;
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
  function closeRun() { if (runBusy()) return; clearInterval(termTimer); termTimer = 0; runm.classList.remove("open"); pill = null; renderPill(); }

  /* değişiklik yapan eylem: plan → snapshot + onay → Terminal (sudo parolası orada) → canlı ilerleme.
     Motor Terminal'de aynı onayları yeniden ister (tek doğruluk kaynağı); arayüz onayı bunun ÖN adımıdır. */
  const EFFECT_TXT = { "installs-package": "paket kurar", "changes-config": "yapılandırmayı değiştirir", destructive: "yıkıcı" };
  let wAct = null, logSince = 0;
  function openWrite(server, pkg, action, item) {
    if (jobBusy(server)) return;
    openRun(server, pkg, action, item);
    runMode = "write";
    const p = appState.srv.packages.find(x => x.id === pkg), a = actionOf(pkg, action); wAct = a;
    const pv = (p.vars || []).filter(v => v.set);
    const prereq = (a.prereqs || []).map(q => {
      const act = p.actions.find(x => x.id === q.action);
      const chips = act && act.keys ? Object.entries(act.keys).map(([k, v]) => `<span class="chip">${esc(k)}=${esc(v)}</span>`).join("") : "";
      return `<div class="sj-fe7b49"><span class="ok">✔ ${esc(q.action)}</span> <span class="muted small">${Math.max(1, Math.round(q.ageSec / 60))} dk önce</span>${chips ? `<div class="chips">${chips}</div>` : ""}</div>`;
    }).join("");
    const warns = (a.warnings || []);
    const destr = !!a.destructive;
    const typeName = destr || a.confirm === "type-server-name";
    $("#run-title").innerHTML = `<span class="mono">${esc(action)}</span> · ${esc(TAB_LABEL[pkg] || pkg)}${runItem ? ` · <b>${esc(runItem)}</b>` : ""} <span class="draft-tag sj-0996f7">${destr ? "YIKICI" : "yazar"}</span>`;
    $("#run-confirm-text").innerHTML = destr
      ? `<b>⚠ YIKICI EYLEM.</b> <b>${esc(server)}</b> sunucusunda dosya/veri <b>silinebilir ya da değiştirilebilir ve geri alınamayabilir</b>: ${(a.effects || []).map(e => esc(EFFECT_TXT[e] || e)).join(", ")}. Aşağıdaki uyarıların tamamını okuyun.`
      : `<b>${esc(server)}</b> sunucusuna <b>gerçekten yazılacak</b>: ${(a.effects || []).map(e => esc(EFFECT_TXT[e] || e)).join(", ")}.`;
    $("#run-confirm-extra").hidden = false;
    $("#run-confirm-extra").innerHTML = `
      ${prereq ? `<div class="small muted sj-56f435">Önkoşul (motor 30 dk tazelik ister)</div>${prereq}` : ""}
      ${pv.length ? `<div class="small muted sj-56f435">Bu değişkenlerle</div><div class="chips">${pv.map(v => `<span class="chip">${esc(v.name)}=${esc(v.value.length > 48 ? v.value.slice(0, 48) + "…" : v.value)}</span>`).join("")}</div>` : ""}
      ${warns.length ? `<div class="small muted sj-56f435">Paketin uyarıları${destr ? " (tamamı)" : ""}</div><ul class="small sj-cf5387">${warns.slice(0, destr ? warns.length : 3).map(w => `<li>${esc(w)}</li>`).join("")}</ul>${!destr && warns.length > 3 ? `<details class="small sj-96ad60"><summary class="muted sj-3b6a3a">${warns.length - 3} uyarı daha</summary><ul class="sj-e15e9c">${warns.slice(3).map(w => `<li>${esc(w)}</li>`).join("")}</ul></details>` : ""}` : ""}
      <label class="small sj-6693ed"><input type="checkbox" id="w-snap"> <span><b>VM snapshot'ı aldım</b>; sorun çıkarsa geri dönebilirim.</span></label>
      ${destr ? `<label class="small sj-b97486"><input type="checkbox" id="w-destr"> <span><b>Bu eylemin geri alınamayabileceğini anlıyorum</b>; yedeğim var ve sonuçlarını üstleniyorum.</span></label>` : ""}
      ${typeName ? `<div class="field sj-56f435"><label for="w-confirm">Onay için sunucu adını yazın: <span class="mono">${esc(server)}</span></label><input id="w-confirm" autocomplete="off" spellcheck="false" placeholder="${esc(server)}"><div class="hint">Büyük/küçük harf duyarlıdır.</div></div>`
        : `<label class="small sj-6abd0a"><input type="checkbox" id="w-yes"> <span>Evet, bu eylemi <b>${esc(server)}</b> üzerinde çalıştırmak istiyorum.</span></label>`}`;
    $("#run-confirm-note").textContent = "Motor aynı onayları (snapshot, e/h ya da ad) ve sudo parolasını çıktıda yeniden sorabilir: kurallar menüdekiyle aynıdır, yanıtları burada verirsiniz. Çalışırken ilerleme izlenir. Parolayı web sayfasına yazmak istemezseniz \"macOS Terminal'de aç\" düğmesini kullanın.";
    $("#run-go").textContent = destr ? "Yıkıcı eylemi çalıştır" : "Çalıştır (onaylı)"; $("#run-go").classList.add(destr ? "dangerbtn" : "warnbtn"); $("#run-go").disabled = true; $("#run-ext").hidden = !!runItem; // öğe eylemleri macOS Terminal'de açılamaz (yalnız uygulama içinde, kapılı iş olarak)
    const check = () => { const okc = $("#w-snap").checked && (!destr || $("#w-destr").checked) && (typeName ? $("#w-confirm").value === server : $("#w-yes").checked); $("#run-go").disabled = !okc; $("#run-ext").disabled = !okc; };
    $("#run-ext").disabled = true;
    ["#w-snap", "#w-destr", "#w-confirm", "#w-yes"].forEach(id => { const e = $(id); if (e) { e.addEventListener("input", check); e.addEventListener("change", check); } });
  }

  // last-run.log'u izle: SSHPOCKET_PROGRESS satırlarından alan alan ilerleme, geri alma güvenlik ağı durumu
  // ilerleme çubuğu genişliği: satır içi stil özniteliği yerine data-w + CSSOM (CSP style-src-attr 'none' ile uyumlu)
  const applyBars = root => $$("i[data-w]", root).forEach(i => { i.style.width = (parseInt(i.dataset.w, 10) || 0) + "%"; });
  async function pollProgress(pkg, server, since) {
    try {
      const q = new URLSearchParams({ package: pkg, server, file: "last-run.log" });
      const r = await api("/api/log?" + q);
      const m = Date.parse(r.headers.get("X-Log-Mtime") || "");
      if (!m || m < since - 3000) return; // önceki çalıştırmanın günlüğü: yok say
      const txt = await r.text();
      const prog = [...txt.matchAll(/SSHPOCKET_PROGRESS alan=(\S+) sira=(\d+)\/(\d+) durum=(\S+)/g)];
      let total = 0, done = new Set(), cur = "";
      prog.forEach(x => { total = +x[3]; if (x[4] === "bitti") done.add(x[1]); else cur = x[1]; });
      if (done.has(cur)) cur = "";
      const armed = /TASK \[hardening_rollback : Arm[^\]]*\][^\n]*\n(ok|changed):/.test(txt), disarmed = /TASK \[hardening_rollback : Disarm[^\]]*\][^\n]*\n(ok|changed):/.test(txt);
      const tail = txt.trimEnd().split("\n").slice(-6).map(esc).join("\n");
      $("#run-result").innerHTML = (total ? `<div class="run-meta"><span>${done.size} / ${total} alan tamam${cur ? ` · şu an: <b>${esc(cur)}</b>` : ""}</span></div>
          <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done.size}"><i data-w="${Math.round(done.size / total * 100)}"></i></div>` : '<div class="small muted">Motor başlıyor…</div>') +
        (armed && !disarmed ? '<div class="notice sj-d2c171"><span class="ico" aria-hidden="true">⏱</span><div><b>Geri alma güvenlik ağı devrede.</b> Erişim doğrulanınca kaldırılır; doğrulanamazsa önceki hâl geri yüklenir.</div></div>' : "") +
        (disarmed ? '<div class="small muted sj-8a77e5">Geri alma güvenlik ağı kaldırıldı (erişim doğrulandı).</div>' : "") +
        `<pre class="mono small sj-b12f21">${tail}</pre>`;
      applyBars($("#run-result"));
    } catch (e) { /* günlük henüz yok ya da okunamadı: bir sonraki turda */ }
  }

  /* sudo gerektiren salt okunur eylem: macOS Terminal'de sshpocket motoru; parolayı kullanıcı orada girer, arayüz görmez */
  // runItem doluysa eylem durumu O ÖĞENİN kapsamında hesaplanmış olandır (önkoşul, son sonuç); yoksa paketin şimdiki seçimine göre.
  function actionOf(pkg, action) {
    const p = appState.srv && appState.srv.packages.find(x => x.id === pkg); if (!p) return undefined;
    if (runItem) { const it = (p.items || []).find(x => x.id === runItem); return it && it.states && it.states[action]; }
    return p.actions.find(x => x.id === action);
  }
  function openTerm(server, pkg, action, item) {
    if (jobBusy(server)) return;
    openRun(server, pkg, action, item);
    runMode = "term";
    $("#run-confirm-text").innerHTML = `<b>${esc(server)}</b> sunucusunda <span class="mono">${esc(action)}</span> (<b>salt okunur</b>) çalışacak. Motor <b>sudo parolası</b> isteyecek: bu pencerede çıkan parola kutusuna yazarsınız; doğrudan Ansible'a iletilir, arayüz onu saklamaz ya da günlüğe yazmaz.`;
    $("#run-go").textContent = "Çalıştır"; $("#run-ext").hidden = !!runItem;
    const pv = (actionOf(pkg, action) && appState.srv && appState.srv.packages.find(x => x.id === pkg).vars || []).filter(v => v.set);
    if (pv.length) $("#run-confirm-text").insertAdjacentHTML("beforeend", `<div class="chips sj-8a77e5"><span class="small muted sj-9162a2">Bu değişkenlerle:</span>${pv.map(v => `<span class="chip">${esc(v.name)}=${esc(v.value.length > 48 ? v.value.slice(0, 48) + "…" : v.value)}</span>`).join("")}</div>`);
    $("#run-confirm-note").textContent = "Parola istemi çıktıda belirince burada bir kutu açılır. Sonuç menüdekiyle aynı şekilde kaydedilir. ssh anahtarınızın ssh-agent'ta yüklü olması gerekir. Parolayı bir web sayfasına yazmak istemezseniz \"macOS Terminal'de aç\" düğmesini kullanın.";
  }
  async function startTerm() {
    $("#run-go").disabled = true; $("#run-ext").hidden = true;
    const a0 = actionOf(runPkg, runAct), base = a0 ? a0.lastTime || "" : "";
    const body = { server: runSrv, package: runPkg, action: runAct };
    if (runItem) body.item = runItem;
    if (runMode === "write") { body.snapshot = $("#w-snap").checked; body.ackDestructive = !!($("#w-destr") && $("#w-destr").checked); body.confirm = $("#w-confirm") ? $("#w-confirm").value : $("#w-yes").checked ? "evet" : ""; }
    logSince = Date.now();
    try { await json("/api/run-terminal", body); }
    catch (e) { $("#run-go").disabled = false; $("#run-ext").hidden = false; toast("Terminal açılamadı: " + e.message); return; }
    $("#run-confirm").hidden = true; $("#run-live").hidden = false; $("#run-go").hidden = true; $("#run-close").hidden = false; $("#run-close").textContent = "Kapat";
    $("#run-log").hidden = true; $("#run-cancel").hidden = true; $("#run-result").innerHTML = "";
    const t0 = Date.now();
    const tick = () => { $("#run-state").innerHTML = `<span class="spinner" aria-hidden="true"></span> Terminal'de çalışıyor… parolayı orada girin · ${Math.round((Date.now() - t0) / 1000)} sn`; };
    tick();
    clearInterval(termTimer);
    let prgTimer = 0;
    if (runMode === "write") { prgTimer = setInterval(() => pollProgress(runPkg, runSrv, logSince), 2000); }
    termTimer = setInterval(async () => {
      tick();
      if (Date.now() - t0 > (runMode === "write" ? 40 : 10) * 60 * 1000) { clearInterval(termTimer); clearInterval(prgTimer); $("#run-state").textContent = "Sonuç beklenirken süre doldu; sayfa yine de sonucu gösterecek."; return; }
      try {
        const r = await json("/api/server?name=" + encodeURIComponent(runSrv));
        const a1 = r.packages.find(x => x.id === runPkg)?.actions.find(x => x.id === runAct);
        if (a1 && (a1.lastTime || "") !== base) {
          clearInterval(termTimer); termTimer = 0; clearInterval(prgTimer); appState.srv = r; if (appState.view === "server") renderServer(); load();
          $("#run-state").innerHTML = a1.lastResult === "ok" ? '<span class="res ok">✔ tamamlandı</span> · sonuç kartta güncellendi' : '<span class="res fail">✖ başarısız</span> · ayrıntı için Terminal penceresine ve Günlükler sayfasına bakın';
          $("#run-result").innerHTML = (a1.keys ? `<div class="chips">${Object.entries(a1.keys).map(([k, v]) => `<span class="chip">${esc(k)}=${esc(v)}</span>`).join("")}</div>` : "") +
            `<div class="sj-d2c171"><button class="btn sm" id="run-open-log">Çıktıyı aç (son çalıştırma günlüğü)</button></div>`;
          $("#run-open-log").addEventListener("click", () => { const pk = runPkg, sv = runSrv; closeRun(); openLastLog(pk, sv); });
        }
      } catch (e) { /* bir sonraki turda yeniden denenir */ }
    }, 3000);
  }

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

  async function startRun() {
    $("#run-go").disabled = true; askedAt = askCancelledAt = -1; askBusy = false;
    const body = { server: runSrv, package: runPkg, action: runAct };
    if (runItem) body.item = runItem;
    if (runMode === "write") { body.snapshot = $("#w-snap").checked; body.ackDestructive = !!($("#w-destr") && $("#w-destr").checked); body.confirm = $("#w-confirm") ? $("#w-confirm").value : $("#w-yes").checked ? "evet" : ""; }
    let r;
    try { r = runMode === "upgrade" ? await json("/api/upgrade", { package: runPkg, version: runUpgTo, ack: true }) : runMode === "task" ? await json("/api/server-task", { server: runSrv, task: runAct }) : await json("/api/run", body); }
    catch (e) { $("#run-go").disabled = false; toast("Çalıştırılamadı: " + e.message); return; }
    watchJob(r.id);
  }

  function watchJob(id, startedAt, resume) {
    runJob = id; runStart = startedAt || Date.now(); pill = { label: runLabel(), state: "run" }; $("#run-min").hidden = false;
    const w = curWatch = { id, server: runSrv, pkg: runPkg, act: runAct, item: runItem, mode: runMode, started: runStart, raw: (resume && resume.raw) || "", from: (resume && resume.from) || 0, idle: 0 };
    $("#run-confirm").hidden = true; $("#run-live").hidden = false; $("#run-go").hidden = true; $("#run-ext").hidden = true; $("#run-close").hidden = true;
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
    if (runMode === "task") { if (ok) markTaskDone(runSrv + ":" + runAct); await load(); renderAddSteps(); if (appState.view === "server" && appState.srvName) { try { appState.srv = await json("/api/server?name=" + encodeURIComponent(appState.srvName)); renderServer(); } catch (e) { /* sonraki otomatik yenileme alır */ } } return; }
    if (runMode === "remove") {
      const { name: delName, purge: delPurge } = getRemoval();
      if (ok && delPurge) { try { await json("/api/server-purge", { name: delName }); } catch (e) { toast("Günlükler silinemedi: " + e.message); } }
      await load(); if (ok) { appState.srvName = ""; appState.srv = null; showView("servers"); toast(`${delName} silindi`); }
      return;
    }
    try { appState.srv = await json("/api/server?name=" + encodeURIComponent(runSrv)); if (appState.view === "server") renderServer(); load(); } catch (e) { /* bir sonraki turda yenilenir */ }
  }
  $("#run-go").addEventListener("click", startRun);
  $("#run-ext").addEventListener("click", startTerm);
  $("#run-close").addEventListener("click", closeRun);
  $("#run-cancel").addEventListener("click", async () => { $("#run-cancel").disabled = true; try { await json("/api/run/cancel", { id: runJob }); } catch (e) { toast("İptal edilemedi: " + e.message); } });

  return {
    openRun, openWrite, openTerm, jobBusy, watchJob, closeRun, openJob, adoptJob,
    setMode: m => { runMode = m; },          // openRun'dan sonra çağıran akışlar (silme, görev, yükseltme) kipi belirler
    setUpgradeTo: v => { runUpgTo = v; },
  };
}
