/* KeyLab — ortak yardımcılar (arayüzün bağımsız, durumsuz parçaları). app.js ve ileride ayrılacak modüller bunları içe aktarır. */

export const $ = (s, r = document) => r.querySelector(s);
export const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
export const t = (k, d) => (window.KeyLabI18n ? window.KeyLabI18n.t(k, d) : d);
export const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

/* ---------------- API ---------------- */
export async function api(path, body) {
  const r = await fetch(path, { method: body ? "POST" : "GET", headers: { "X-Requested-With": "sshpocket" }, body: body ? JSON.stringify(body) : undefined });
  if (r.status === 401) throw new Error("oturum yok ya da süresi doldu: terminalde yazılan adresi açın");
  if (!r.ok) throw new Error((await r.text()).trim() || r.status);
  return r;
}
export const json = async (p, b) => (await api(p, b)).json();
export function toast(t) {
  const e = $("#toast"); e.textContent = t; e.style.display = "block";
  clearTimeout(toast.h); toast.h = setTimeout(() => (e.style.display = "none"), 2400);
}

/* ---------------- Biçim yardımcıları ---------------- */
export function ago(iso) {
  const t = Date.parse(iso); if (!t) return "";
  const s = Math.max(0, (Date.now() - t) / 1000);
  if (s < 90) return "şimdi";
  if (s < 3600) return Math.round(s / 60) + " dk";
  if (s < 86400) return Math.round(s / 3600) + " sa";
  return Math.round(s / 86400) + " gün";
}
export const localTime = iso => { const d = new Date(iso); return isNaN(d) ? iso : d.toLocaleString("tr-TR"); };
export const isOk = r => r === "ok";
export const resHtml = a => !a ? '<span class="res none">— kayıt yok</span>'
  : isOk(a.result) ? '<span class="res ok">✔ başarılı</span>' : '<span class="res fail">✖ başarısız</span>';
