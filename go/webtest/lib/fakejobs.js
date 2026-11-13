"use strict";
/* Sahte iş sunucusu: gerçek Go sunucusunun /api/run, /api/jobs, /api/run/cancel sözleşmesini taklit eder (testler arası ortak). */
const { fixture } = require("../harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const status = () => ({ db1: { up: true, conn: "up" }, vm1: { up: false, conn: "down" }, web1: { up: true, conn: "up" } });

function fakeJobs() {
  const jobs = new Map(); let n = 0; const posts = [];
  const running = () => [...jobs.values()].filter(j => !j.done).reverse();
  const api = {
    jobs, posts,
    feed: (id, text) => { jobs.get(id).text += text; },
    finish: (id, exit = 0) => { jobs.get(id).done = true; jobs.get(id).exit = exit; },
    start: (server, action = "durum", pkg = "utility") => { const id = "j" + (++n); jobs.set(id, { id, server, package: pkg, action, started: "2026-11-14T10:00:00Z", text: "", done: false, exit: 0 }); return id; },
    overrides: {
      "/api/run": (u, { method, body }) => {
        if (method === "POST") {
          posts.push(body);
          if (running().some(j => j.server === body.server)) return { status: 409, body: "bu sunucuda başka bir eylem çalışıyor" };
          const id = api.start(body.server, body.action, body.package); api.feed(id, `basladi ${body.server}\n`); return { id };
        }
        const j = jobs.get(u.searchParams.get("id")); const from = parseInt(u.searchParams.get("from") || "0", 10);
        return { id: j.id, server: j.server, package: j.package, action: j.action, started: j.started, text: j.text.slice(from), next: j.text.length, done: j.done, exit: j.exit };
      },
      "/api/jobs": () => ({ jobs: [...jobs.values()].map(({ id, server, package: p, action, started, done, exit }) => ({ id, server, package: p, action, started, done, exit })), running: running()[0]?.id || "", runningIds: running().map(j => j.id) }),
      "/api/run/cancel": (u, { body }) => { api.finish(body.id, 1); return { ok: true }; },
    },
  };
  return api;
}

module.exports = { fakeJobs, mixedHome, status };
