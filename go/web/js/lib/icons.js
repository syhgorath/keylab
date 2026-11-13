/* KeyLab — işletim sistemi simgeleri ve adları. app.js'ten çıkarıldı (refaktör 5), kod BİREBİR. */
import { esc } from "./util.js";

/* İşletim sistemi simgeleri: kendi çizdiğimiz sade işaretler (resmi logo değil), 48x48 karo.
   Resmi logoları kullanmak istersek burada aynı anahtarlarla değiştirilir. */
export const OS_LABEL = { ubuntu: "Ubuntu", debian: "Debian", proxmox: "Proxmox VE", linux: "Linux" };
const OS_ICON = {
  ubuntu: '<rect width="48" height="48" rx="11" fill="#E95420"/><circle cx="24" cy="24" r="10.5" fill="none" stroke="#fff" stroke-width="2.6"/><circle cx="13.5" cy="24" r="4" fill="#fff"/><circle cx="29.2" cy="14.9" r="4" fill="#fff"/><circle cx="29.2" cy="33.1" r="4" fill="#fff"/><circle cx="13.5" cy="24" r="1.6" fill="#E95420"/><circle cx="29.2" cy="14.9" r="1.6" fill="#E95420"/><circle cx="29.2" cy="33.1" r="1.6" fill="#E95420"/>',
  debian: '<rect width="48" height="48" rx="11" fill="#A80030"/><path d="M29.5 12.5c-9-3.2-18 2.3-17.5 12 .4 7.6 7 12 13.4 10.6 5-1.1 8-5.4 6.6-9.6-1.1-3.3-4.8-4.6-7.6-3.2-2.6 1.3-3 4.8-.8 6.4" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round"/>',
  proxmox: '<rect width="48" height="48" rx="11" fill="#2b2f36"/><path d="M13 13l22 22M35 13L13 35" stroke="#E57000" stroke-width="5.5" stroke-linecap="round"/><path d="M13 13l22 22M35 13L13 35" stroke="#fff" stroke-opacity=".0" stroke-width="1"/>',
  linux: '<rect width="48" height="48" rx="11" fill="var(--surface-3)"/><rect x="12" y="13" width="24" height="9" rx="2.5" fill="none" stroke="var(--muted)" stroke-width="2.4"/><rect x="12" y="26" width="24" height="9" rx="2.5" fill="none" stroke="var(--muted)" stroke-width="2.4"/><circle cx="17" cy="17.5" r="1.4" fill="var(--muted)"/><circle cx="17" cy="30.5" r="1.4" fill="var(--muted)"/>',
};
export const osIcon = (os, size = 44) => `<svg class="osico" width="${size}" height="${size}" viewBox="0 0 48 48" role="img" aria-label="${esc(OS_LABEL[os] || "Linux")}">${OS_ICON[os] || OS_ICON.linux}</svg>`;
