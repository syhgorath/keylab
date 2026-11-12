/* KeyLab — komut paleti (⌘K / Ctrl+K): sayfalara, sunuculara ve sık komutlara klavyeyle hızlı erişim.
   Kaynaklar: sayfalar, kayıtlı sunucular (aç / terminal / Lynis / günlükler) ve birkaç komut (dil, tema, ekranı gizle, yenile…).
   Aramada Türkçe karakterler ve büyük/küçük harf yok sayılır ("istanbul" ≈ "İstanbul"; "ş" ≈ "s"). Boşlukla ayrılan her sözcük
   eşleşmelidir. ARIA: combobox + listbox (aria-activedescendant). Güvenlik: yalnız mevcut ekran eylemlerini çağırır; yeni yetki
   ya da uç nokta yoktur. Metinler textContent ile yazılır. Terminal odaktayken (Ctrl+K kabukta "satır sonuna kadar sil") ve gizlilik
   kilidi açıkken tetiklenmez. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  const norm = s => String(s || "").toLocaleLowerCase("tr").normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/ı/g, "i");
  const MAX = 14;

  let hooks = null, items = [], shown = [], sel = 0, from = null;

  function build() {
    const home = hooks.getHome(), out = [];
    const add = (group, label, hint, keys, run) => out.push({ group, label, hint, hay: norm(label + " " + (keys || "")), run });
    const G = { page: L("Sayfalar", "Pages"), srv: L("Sunucular", "Servers"), act: L("Sunucu eylemleri", "Server actions"), cmd: L("Komutlar", "Commands") };
    [["dash", "Pano", "Dashboard", "ana sayfa home"], ["servers", "Sunucular", "Servers", "liste"], ["packages", "Paketler", "Packages", "components"],
     ["compare", "Karşılaştır", "Compare", "drift fark matris"], ["reports", "Raporlar", "Reports", "rapor yazdir pdf"], ["lynis", "Lynis denetimi", "Lynis audit", "guvenlik skor"],
     ["logs", "Günlükler", "Logs", "log kayit"], ["settings", "Ayarlar", "Settings", "tema dil yedek kilit vurgu"], ["add", "Sunucu ekle", "Add server", "yeni ssh config ice aktar"]]
      .forEach(([v, tr, en, k]) => add(G.page, L(tr, en), "", k, () => hooks.showView(v)));
    for (const s of home.servers) {
      const addr = s.valid ? `${s.user}@${s.host}:${s.port}` : "";
      add(G.srv, s.name, addr, `${s.host || ""} ${s.platform || ""} ${s.os || ""}`, () => hooks.openServer(s.name));
    }
    for (const s of home.servers) {
      if (!s.valid) continue;
      add(G.act, `${s.name} · ${L("Terminal", "Terminal")}`, L("gömülü ssh", "embedded ssh"), "ssh baglan shell", () => window.KeyLabTerminal.open(s.name));
      add(G.act, `${s.name} · Lynis`, L("denetim sonuçları", "audit results"), "guvenlik", () => hooks.goLynis(s.name, ""));
      add(G.act, `${s.name} · ${L("Günlükler", "Logs")}`, "", "log kayit", () => hooks.goLogs(s.name));
    }
    add(G.cmd, L("Dili değiştir (TR/EN)", "Switch language (TR/EN)"), "", "language dil turkce english", () => $("#lang-btn").click());
    add(G.cmd, L("Temayı değiştir", "Change theme"), "", "tema koyu acik dark light", () => $("#theme-btn").click());
    add(G.cmd, L("Ekranı gizle", "Hide the screen"), "L", "gizlilik kilit privacy lock", () => window.KeyLabPrivacy.lock("manual"));
    add(G.cmd, L("Klavye kısayolları", "Keyboard shortcuts"), "?", "kisayol", () => hooks.openShortcuts());
    add(G.cmd, L("Son işler", "Recent jobs"), "", "is gorev gecmis jobs history", () => window.KeyLabJobs.open());
    add(G.cmd, L("Listeyi yenile", "Refresh the list"), "", "yenile reload", () => hooks.reload());
    return out;
  }

  function filter(q) {
    const toks = norm(q).split(/\s+/).filter(Boolean);
    if (!toks.length) { // boş arama: sayfalar + komutlar + ilk sunucular
      const by = g => items.filter(i => i.group === g);
      const groups = [...new Set(items.map(i => i.group))];
      return [...by(groups[0]), ...by(groups[1]).slice(0, 6), ...by(groups[3])].slice(0, MAX);
    }
    const scored = [];
    for (const it of items) {
      let score = 0, ok = true;
      for (const t of toks) {
        const i = it.hay.indexOf(t);
        if (i < 0) { ok = false; break; }
        score += (i === 0 ? 0 : it.hay[i - 1] === " " ? 1 : 3) + i * 0.01;
      }
      if (ok) scored.push([score + it.label.length * 0.001, it]);
    }
    scored.sort((a, b) => a[0] - b[0]);
    return scored.slice(0, MAX).map(x => x[1]);
  }

  function render() {
    const list = $("#palette-list"), inp = $("#palette-in");
    list.replaceChildren();
    if (!shown.length) {
      const e = document.createElement("li"); e.className = "palette-empty"; e.setAttribute("role", "presentation"); e.textContent = L("Eşleşme yok.", "No match."); list.appendChild(e);
      inp.removeAttribute("aria-activedescendant"); return;
    }
    let last = "";
    shown.forEach((it, i) => {
      if (it.group !== last) { last = it.group; const h = document.createElement("li"); h.className = "palette-group"; h.setAttribute("role", "presentation"); h.textContent = it.group; list.appendChild(h); }
      const li = document.createElement("li"); li.id = "palette-opt-" + i; li.className = "palette-opt" + (i === sel ? " active" : ""); li.setAttribute("role", "option"); li.setAttribute("aria-selected", String(i === sel));
      const a = document.createElement("span"); a.className = "palette-label"; a.textContent = it.label;
      const b = document.createElement("span"); b.className = "palette-hint mono"; b.textContent = it.hint || "";
      li.append(a, b);
      li.addEventListener("mousemove", () => { if (sel !== i) { sel = i; mark(); } });
      li.addEventListener("click", () => run(i));
      list.appendChild(li);
    });
    inp.setAttribute("aria-activedescendant", "palette-opt-" + sel);
  }

  function mark() {
    document.querySelectorAll("#palette-list .palette-opt").forEach((el, i) => { el.classList.toggle("active", i === sel); el.setAttribute("aria-selected", String(i === sel)); });
    $("#palette-in").setAttribute("aria-activedescendant", "palette-opt-" + sel);
    const el = $("#palette-opt-" + sel); if (el) el.scrollIntoView({ block: "nearest" });
  }

  const isOpen = () => $("#palette").classList.contains("open");

  function open() {
    if (window.KeyLabPrivacy && window.KeyLabPrivacy.locked) return;
    from = document.activeElement; items = build(); sel = 0;
    $("#palette").classList.add("open"); $("#palette-in").setAttribute("aria-expanded", "true");
    const inp = $("#palette-in"); inp.value = ""; shown = filter(""); render(); inp.focus();
  }
  function close(restore = true) {
    $("#palette").classList.remove("open"); $("#palette-in").setAttribute("aria-expanded", "false");
    if (restore && from && from.isConnected && from.focus) from.focus();
    from = null;
  }
  function run(i) {
    const it = shown[i]; if (!it) return;
    close(false); setTimeout(() => it.run(), 0);
  }

  function init(h) {
    hooks = h;
    const inp = $("#palette-in");
    inp.addEventListener("input", () => { shown = filter(inp.value); sel = 0; render(); });
    inp.addEventListener("keydown", e => {
      if (e.key === "ArrowDown") { e.preventDefault(); if (shown.length) { sel = (sel + 1) % shown.length; mark(); } }
      else if (e.key === "ArrowUp") { e.preventDefault(); if (shown.length) { sel = (sel - 1 + shown.length) % shown.length; mark(); } }
      else if (e.key === "Home") { e.preventDefault(); sel = 0; mark(); }
      else if (e.key === "End") { e.preventDefault(); sel = Math.max(0, shown.length - 1); mark(); }
      else if (e.key === "Enter") { e.preventDefault(); run(sel); }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); close(); }
      else if (e.key === "Tab") e.preventDefault(); // odak paletin içinde kalsın
    });
    $("#palette").addEventListener("click", e => { if (e.target === $("#palette")) close(); });
    document.addEventListener("keydown", e => {
      if (!(e.metaKey || e.ctrlKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== "k") return;
      if (window.KeyLabPrivacy && window.KeyLabPrivacy.locked) return;
      const el = document.activeElement;
      if (el && el.closest && el.closest(".xterm")) return; // terminalde Ctrl+K kabuğa aittir
      if (isOpen()) { e.preventDefault(); close(); return; }
      if (document.querySelector(".modal-back.open")) return; // başka pencere açıkken açma
      e.preventDefault(); open();
    });
    window.addEventListener("keylab:langchange", () => { if (isOpen()) { items = build(); shown = filter(inp.value); sel = 0; render(); } });
  }

  window.KeyLabPalette = { init, open, close };
})();
