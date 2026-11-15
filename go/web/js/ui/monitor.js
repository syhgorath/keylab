/* KeyLab — Kaynaklar sayfası (API: GET /api/monitor, bkz. monitor.go): KeyLab'ın KENDİ kullanımı — CPU, RAM, diskte kapladığı yer ve şu an çalışanlar
   (macOS Etkinlik Monitörü gibi ama yalnız KeyLab ve başlattığı süreçler). Sayfa açıkken 2 sn'de bir ölçer, kapanınca durur (arka planda ölçüm yok).
   Küçük geçmiş çizgileri (son 60 örnek) SVG'dir; çubuk genişlikleri CSP style-src-attr 'none' ile uyumlu olarak data-w + CSSOM ile verilir. */
import { $, $$, t } from "../lib/util.js";

const fmt = (s, o) => Object.keys(o).reduce((a, k) => a.split("{" + k + "}").join(o[k]), s);
export const bytesText = n => {
  n = Number(n) || 0;
  if (n < 1024) return n + " B";
  const u = ["KB", "MB", "GB", "TB"]; let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  const x = n >= 100 ? n.toFixed(0) : n >= 10 ? n.toFixed(1) : n.toFixed(2);
  return (x.includes(".") ? x.replace(/\.?0+$/, "") : x) + " " + u[i]; // yalnız ondalık kısımdaki sondaki sıfırlar atılır ("120" → "120", "1.50" → "1.5")
};
export const durText = sec => {
  sec = Math.max(0, Math.round(sec));
  const d = Math.floor(sec / 86400), h = Math.floor(sec % 86400 / 3600), m = Math.floor(sec % 3600 / 60), s = sec % 60;
  const U = { d: t("mon_u_d", "g"), h: t("mon_u_h", "sa"), m: t("mon_u_m", "dk"), s: t("mon_u_s", "sn") };
  if (d) return `${d}${U.d} ${h}${U.h}`; if (h) return `${h}${U.h} ${m}${U.m}`; if (m) return `${m}${U.m} ${s}${U.s}`; return `${s}${U.s}`;
};

const KINDS = { self: ["mon_k_self", "KeyLab (ana süreç)"], agent: ["mon_k_agent", "Özel ssh-agent"], ssh: ["mon_k_ssh", "ssh"], python: ["mon_k_python", "Python / Ansible"], task: ["mon_k_task", "Eylem (keylab run)"], other: ["mon_k_other", "Diğer"] };
const DIRS = { servers: ["mon_d_servers", "Sunucular"], components: ["mon_d_components", "Paketler"], state: ["mon_d_state", "Durum ve günlükler"], cache: ["mon_d_cache", "Önbellek (Python ortamı)"] };
const HIST = 60;

export function initMonitor({ json, setTimer = (f, ms) => setInterval(f, ms), clearTimer = id => clearInterval(id) } = {}) {
  let timer = 0, busy = false;
  const hist = { cpu: [], mem: [] };

  const spark = (id, arr, max) => {
    const svg = $(id); if (!svg) return;
    const pts = arr.map((v, i) => `${(i / (HIST - 1) * 120).toFixed(1)},${(32 - Math.min(1, v / (max || 1)) * 30 - 1).toFixed(1)}`).join(" ");
    svg.querySelector("polyline").setAttribute("points", pts);
  };
  const bar = (el, pct) => { el.dataset.w = String(Math.max(0, Math.min(100, Math.round(pct)))); el.style.width = el.dataset.w + "%"; };

  function paint(s) {
    const cores = s.cores || 1;
    $("#mon-cpu").textContent = s.cpu.toFixed(1) + "%";
    $("#mon-cpu-sub").textContent = fmt(t("mon_cpu_sub", "tek çekirdeğe göre · tüm çekirdeklerin {n}%'i ({c} çekirdek)"), { n: (s.cpu / cores).toFixed(1), c: cores });
    $("#mon-ram").textContent = bytesText(s.rssMB * 1048576);
    $("#mon-ram-sub").textContent = s.memTotalMB > 0 ? fmt(t("mon_ram_sub", "bilgisayarın belleğinin {n}%'i ({total})"), { n: (s.rssMB / s.memTotalMB * 100).toFixed(2), total: bytesText(s.memTotalMB * 1048576) }) : t("mon_ram_sub0", "KeyLab ve alt süreçleri");
    const used = (s.dirs || []).reduce((a, d) => a + d.bytes, 0);
    $("#mon-disk").textContent = bytesText(used);
    $("#mon-disk-sub").textContent = s.volume ? fmt(t("mon_disk_sub", "diskte kapladığı yer · birimde boş: {free} / {total}"), { free: bytesText(s.volume.freeBytes), total: bytesText(s.volume.totalBytes) }) : t("mon_disk_sub0", "diskte kapladığı yer");
    hist.cpu.push(s.cpu); hist.mem.push(s.rssMB);
    for (const k of Object.keys(hist)) while (hist[k].length > HIST) hist[k].shift();
    spark("#mon-cpu-spark", hist.cpu, Math.max(100, ...hist.cpu)); spark("#mon-ram-spark", hist.mem, Math.max(1, ...hist.mem));
    $("#mon-up").textContent = fmt(t("mon_up", "KeyLab {d} önce açıldı · {t} terminal, {a} eylem çalışıyor · {g} goroutine"), { d: durText(s.uptimeSec), t: s.terms, a: s.tasks, g: s.goroutines });

    const tb = $("#mon-procs"); tb.replaceChildren();
    const procs = (s.procs || []).slice().sort((a, b) => b.cpu - a.cpu || b.rssMB - a.rssMB);
    for (const p of procs) {
      const tr = document.createElement("tr");
      const k = KINDS[p.kind] || KINDS.other;
      for (const [txt, cls] of [[p.name, "mono"], [t(k[0], k[1]), ""], [String(p.pid), "mono num"], [p.cpu.toFixed(1) + "%", "num"], [bytesText(p.rssMB * 1048576), "num"], [durText(p.elapsedSec), "num"]]) {
        const td = document.createElement("td"); td.textContent = txt; if (cls) td.className = cls; tr.append(td);
      }
      tb.append(tr);
    }
    $("#mon-procs-count").textContent = fmt(t("mon_procs_n", "{n} süreç"), { n: procs.length });

    const ul = $("#mon-dirs"); ul.replaceChildren();
    const total = Math.max(1, ...(s.dirs || []).map(d => d.bytes));
    for (const d of s.dirs || []) {
      const li = document.createElement("li");
      const k = DIRS[d.id] || [d.id, d.id];
      const head = document.createElement("div"); head.className = "mon-dir-head";
      const nm = document.createElement("span"); nm.textContent = t(k[0], k[1]) + (d.temp ? " · " + t("mon_temp", "geçici — çıkışta silinir") : "");
      const sz = document.createElement("b"); sz.className = "num"; sz.textContent = bytesText(d.bytes);
      head.append(nm, sz);
      const track = document.createElement("div"); track.className = "mon-track"; const fill = document.createElement("i"); track.append(fill); bar(fill, d.bytes / total * 100);
      const path = document.createElement("div"); path.className = "mon-path mono small muted"; path.textContent = d.path;
      li.append(head, track, path); ul.append(li);
    }
    $("#mon-note").textContent = s.note || "";
  }

  async function tick() {
    if (busy) return; busy = true;
    try { paint(await json("/api/monitor")); } catch (e) { $("#mon-note").textContent = String(e.message || e); } finally { busy = false; }
  }
  const show = () => { hide(); tick(); timer = setTimer(tick, 2000); };
  function hide() { if (timer) clearTimer(timer); timer = 0; }
  return { show, hide, tick };
}
