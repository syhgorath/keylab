/* KeyLab — Sunucu ekle sayfası: form + canlı taslak doğrulaması, oluşturma, sonraki adımlar (hazırlık görevleri) ve görev penceresi.
   app.js'ten çıkarıldı (refaktör 5e), kod BİREBİR. Testler: go/webtest/ops.test.js, runmodal2.test.js */
import { $, $$, esc, json, toast } from "../lib/util.js";
import { appState } from "../state.js";
import { TAB_LABEL } from "../lib/labels.js";

// hooks: { load(), openServer(ad, sekme?), openVars(ad, paket), jobs (openRun, jobBusy, setMode) }
//   →  { init() (sayfa ilk açılışında), renderSteps(), openTask(sunucu, görev), markTaskDone(anahtar) }
export function initAddServer(hooks) {
  const { load, openServer, openVars, jobs } = hooks;
  const { openRun, jobBusy } = jobs;
  let addServer = "";
  const addDone = new Set();
  const TASKS = {
    keygen: ["Yeni anahtar üret", "Anahtar oluştur", "ed25519 anahtarı <span class=\"mono\">servers/&lt;ad&gt;/id_ed25519_&lt;ad&gt;</span> olarak oluşturulacak (var olan anahtarın üzerine yazılmaz). Passphrase belirlemeniz istenecek (<b>boş bırakmanız önerilmez</b>): parola penceresi iki kez açılır."],
    keycopy: ["Anahtarı sunucuya gönder", "Anahtarı sunucuya gönder", "Genel anahtar sunucuya eklenecek (<span class=\"mono\">ssh-copy-id</span>). Motor parmak izini gösterip onay ister; sunucunun <b>ilk bağlantı sorusunu</b> (host anahtarı) dikkatle okuyun. Sunucudaki kullanıcının parolası istenirse parola penceresine yazın."],
    keychain: ["Keychain parolası", "Passphrase'i Keychain'e kaydet / güncelle / sil", "Anahtarın passphrase'i macOS Keychain'e (<span class=\"mono\">USBMonitor_&lt;ÖNEK&gt;_Passphrase</span>) kaydedilir; eylemler anahtarı parola sormadan ajana ekleyebilir. Motor önce ne yapacağınızı sorar (1 kaydet · 2 sil · 0 geri); kayıttan sonra anahtar açılarak doğrulanır."],
    knock: ["Port knocking", "Port knocking ayarları", "SSH portu kapalıyken önceden kararlaştırılmış bir port sırasına 'vurarak' açtırma yöntemi (istemci tarafı). Protokol, bekleme ve dizi sorulur; dizi Keychain'e kaydedilir. Sunucuda knock kurulumu yönetici tarafından yapılmış olmalıdır. Bağlanırken ve Terminal'de açarken knock otomatik gönderilir."],
    sshadd: ["ssh-agent'a ekle", "Anahtarı ssh-agent'a ekle", "Anahtar ssh-agent'a eklenecek (macOS'ta Keychain'e kaydedilerek); eylemler parola sormadan bağlanabilsin. Anahtarın passphrase'i istenecek."],
  };
  function openTask(server, task) {
    if (jobBusy(server)) return;
    openRun(server, "", task);
    jobs.setMode("task");
    const [, title, text] = TASKS[task];
    $("#run-title").innerHTML = `${esc(title)} <span class=\"draft-tag sj-c8ceca\">${esc(server)}</span>`;
    $("#run-sub").textContent = "Bağlantı hazırlığı";
    $("#run-confirm-text").innerHTML = text;
    $("#run-confirm-note").textContent = "Motorun soruları çıktıda belirir: Evet/Hayır düğmeleriyle ya da kutuya yazarak yanıtlayın. Parolalar sistem parola penceresine yazılır.";
    $("#run-go").textContent = "Başlat";
  }
  function renderAddSteps() {
    const wrap = $("#add-steps");
    if (!addServer) { wrap.hidden = true; $("#add-form-wrap").hidden = false; return; }
    const sv = appState.home.servers.find(x => x.name === addServer);
    $("#add-form-wrap").hidden = true; wrap.hidden = false;
    const hasKey = !!(sv && sv.hasKey);
    const done = t => addDone.has(addServer + ":" + t);
    const step = (n, title, desc, st, btn) => `<div class="act sj-b4411e"><div class="badge ${st === "ok" ? "ok" : st === "todo" ? "warn" : "plain"} sj-03c4bb">${st === "ok" ? "✔" : n}</div><div><div class="sj-eed0f8">${title}</div><div class="small muted">${desc}</div></div><div>${btn || ""}</div></div>`;
    wrap.innerHTML = `<div class="notice ok sj-da12f2"><span class="ico" aria-hidden="true">✔</span><div><b>${esc(addServer)}</b> oluşturuldu. Sunucuyu kullanıma hazırlamak için adımları sırayla izleyin; her adım isteğe bağlıdır ve yeniden yapılabilir.</div></div>
      <div class="card"><div class="card-body tight">
        ${step(1, "Sunucu kaydı", `<span class="mono">servers/${esc(addServer)}/server.env</span> yazıldı`, "ok")}
        ${step(2, "SSH anahtarı", hasKey ? "Anahtar var" : "Yeni bir ed25519 anahtarı üretin (ya da mevcut anahtarı <span class=\"mono\">servers/" + esc(addServer) + "/</span> içine koyun)", hasKey ? "ok" : "todo", hasKey ? "" : '<button class="btn sm primary" data-task="keygen">Yeni anahtar üret</button>')}
        ${step(3, "Anahtarı sunucuya gönder", "Genel anahtarı sunucudaki ~/.ssh/authorized_keys dosyasına ekler (ssh-copy-id)", done("keycopy") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="keycopy">Gönder…</button>' : "")}
        ${step(4, "ssh-agent'a ekle", "Eylemlerin parola sormadan bağlanması için (macOS Keychain)", done("sshadd") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="sshadd">Ekle…</button>' : "")}
        ${step(5, "Keychain parolası", "Passphrase'i Keychain'e kaydeder (parolalı anahtar kullanıyorsanız)", done("keychain") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="keychain">Ayarla…</button>' : "")}
        ${step(6, "Port knocking (isteğe bağlı)", "Sunucu SSH portunu knock dizisiyle açıyorsa", done("knock") ? "ok" : "idle", '<button class="btn sm" data-task="knock">Ayarla…</button>')}
        ${step(7, "Bağlantıyı dene", "Salt okunur <span class=\"mono\">utility durum</span> çalıştırır: ssh bağlantısı, platform ve saat denetlenir", "idle", hasKey ? '<button class="btn sm" id="add-test">Dene</button>' : "")}
        ${step(8, "Değişkenler", "linux-hardening için <span class=\"mono\">hardening_ssh_allow_from</span> gibi zorunlu değerleri ayarlayın", "idle", '<button class="btn sm" id="add-vars">Değişkenleri ayarla</button>')}
      </div></div>
      <div class="row-flex sj-d6f2af"><button class="btn primary" id="add-open">Sunucu sayfasına git</button><button class="btn ghost" id="add-another">Yeni bir sunucu daha ekle</button></div>`;
    $$("#add-steps [data-task]").forEach(b => b.addEventListener("click", () => openTask(addServer, b.dataset.task)));
    $("#add-test")?.addEventListener("click", () => openRun(addServer, "utility", "durum"));
    $("#add-vars").addEventListener("click", async () => { await openServer(addServer, "linux-hardening"); openVars(addServer, "linux-hardening"); });
    $("#add-open").addEventListener("click", () => openServer(addServer));
    $("#add-another").addEventListener("click", () => { addServer = ""; $("#add-name").value = $("#add-host").value = $("#add-user").value = $("#add-port").value = ""; renderAddSteps(); schedule(); });
  }
  let addReady = false, addTimer = 0;
  const addTouched = new Set();
  async function initAdd() {
    if (addReady) return;
    addReady = true;
    try {
      const pr = (await json("/api/profiles")) || [];
      $("#add-os").insertAdjacentHTML("beforeend", pr.map(p => `<option value="${esc(p.id)}">${esc(p.id)} — ${esc(Object.entries(p.support).map(([k, v]) => `${TAB_LABEL[k] || k}: ${v === "supported" ? "destekli" : v === "planned" ? "planlı" : v}`).join(", "))}</option>`).join(""));
    } catch (e) { $("#e-os").textContent = "Profiller alınamadı: " + e.message; }
    ["name", "host", "user", "port", "os"].forEach(f => {
      const el = $("#add-" + f);
      el.addEventListener("input", () => { addTouched.add(f); schedule(); });
      el.addEventListener("change", () => { addTouched.add(f); schedule(); });
      el.addEventListener("blur", () => { addTouched.add(f); schedule(); });
    });
    schedule();
  }
  function schedule() { clearTimeout(addTimer); addTimer = setTimeout(checkAdd, 150); }
  async function checkAdd() {
    const body = { name: $("#add-name").value, host: $("#add-host").value, user: $("#add-user").value, port: $("#add-port").value, os: $("#add-os").value };
    let d;
    try { d = await json("/api/server-draft", body); } catch (e) { $("#add-preview").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Doğrulanamadı: ${esc(e.message)}</div></div>`; return; }
    ["name", "host", "user", "port", "os"].forEach(f => {
      const msg = d.errors && d.errors[f], show = !!msg && addTouched.has(f);
      const hint = $("#e-" + f);
      hint.closest(".field").classList.toggle("bad", show);
      hint.classList.toggle("err", show);
      hint.textContent = show ? msg : f === "name" ? "Küçük harf, rakam ve _ ; harfle başlar." : f === "os" ? "Paketlerin desteklediği profiller listelenir." : "";
    });
    if (!d.ok) { $("#add-preview").innerHTML = '<div class="empty-state sj-5417cf"><h3>Önizleme</h3><p>Alanlar geçerli olunca oluşturulacak dosya ve sonraki adımlar burada görünür.</p></div>'; return; }
    $("#add-preview").innerHTML = `
      <div class="small muted">Oluşturulacak dosya <span class="draft-tag">yazılmayacak</span></div>
      <div class="mono small sj-fe7b49">${esc(d.envPath)}</div>
      <div class="codeblock">${esc(d.envText)}</div>
      <div class="small muted">Anahtar</div>
      <div class="mono small sj-6926eb">${esc(d.keyPath)}</div>
      <div class="small muted">Sonraki adımlar (terminalde, parolayı ve passphrase'i orada siz girersiniz)</div>
      ${d.commands.map((c, i) => `<div class="codeblock" data-i="${i}">${esc(c)}</div>`).join("")}
      <div class="row-flex"><button class="btn sm" id="add-copy">Komutları kopyala</button><button class="btn sm primary" id="add-create">Sunucuyu oluştur</button></div>
      <div id="add-err" class="small sj-d64673" role="alert"></div>`;
    $("#add-create").addEventListener("click", async () => {
      $("#add-create").disabled = true; $("#add-err").textContent = "";
      try { await json("/api/server-create", { name: $("#add-name").value, host: $("#add-host").value, user: $("#add-user").value, port: $("#add-port").value, os: $("#add-os").value }); }
      catch (e) { $("#add-create").disabled = false; $("#add-err").textContent = "✖ " + e.message; return; }
      addServer = $("#add-name").value.trim(); toast("Sunucu oluşturuldu"); await load(); renderAddSteps();
    });
    $("#add-copy").addEventListener("click", async () => { try { await navigator.clipboard.writeText(d.commands.join("\n")); toast("Komutlar kopyalandı"); } catch { prompt("Kopyalayın:", d.commands.join("\n")); } });
  }

  return { init: initAdd, renderSteps: renderAddSteps, openTask, markTaskDone: k => addDone.add(k) };
}
