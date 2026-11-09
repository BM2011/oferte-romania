// Service worker: aplicația se poate instala pe telefon, merge și fără internet
// (arată ultimele oferte descărcate) și, pe Android, verifică în fundal produsele urmărite.
const CACHE = "oferte-v2";
const CORE = ["./", "index.html", "data.json", "manifest.json", "qrcode.js", "icon-192.png", "icon-512.png"];
self.addEventListener("install", e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(CORE)).then(() => self.skipWaiting()));
});
self.addEventListener("activate", e => {
  e.waitUntil(caches.keys().then(ks => Promise.all(ks.filter(k => k !== CACHE && k !== "oferte-alerte").map(k => caches.delete(k)))).then(() => self.clients.claim()));
});
self.addEventListener("fetch", e => {
  const u = new URL(e.request.url);
  if (e.request.method !== "GET" || u.origin !== location.origin || u.pathname.includes("/api/") || u.pathname.includes("__alerte")) return;
  e.respondWith(
    fetch(e.request).then(r => {
      if (r.ok) { const copy = r.clone(); caches.open(CACHE).then(c => c.put(e.request, copy)); }
      return r;
    }).catch(() => caches.match(e.request, { ignoreSearch: true }))
  );
});

// ---- verificare în fundal (Periodic Background Sync, Chrome Android, aplicație instalată)
const norm = s => (s || "").toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, "");
const STOPW = new Set(["de","cu","si","la","din","pt","pentru","fara","in","un","o"]);
const MODW = new Set(["cu","in","fara","si","pentru","la"]);
const SUF = /^(|a|e|i|u|ul|ui|ua|aua|uri|urile|ele|ii|ile|ea|le|lor)$/;
const toks = s => norm(s).replace(/[^a-z0-9]+/g, " ").split(" ").filter(w => w.length > 1 && !STOPW.has(w));
const stem = w => w.length > 4 ? w.slice(0, -1) : w;
const wmatch = (t, w) => (w.startsWith(t) && SUF.test(w.slice(t.length))) || (w.startsWith(stem(t)) && SUF.test(w.slice(stem(t).length)));
const goodMatch = (q, name) => { const T = toks(q); if (!T.length) return false; const W = toks(name);
  if (!T.every(t => W.some(w => wmatch(t, w)))) return false;
  const R = norm(name).replace(/[^a-z0-9]+/g, " ").trim().split(" "); const i = R.findIndex(w => wmatch(T[0], w));
  return i >= 0 && i <= 2 && !MODW.has(R[i - 1]); };
const NAMES = {lidl:"Lidl",kaufland:"Kaufland",carrefour:"Carrefour",profi:"Profi",dedeman:"Dedeman",brico:"Brico",leroy:"Leroy Merlin",hornbach:"Hornbach"};
async function checkAlerts() {
  const c = await caches.open("oferte-alerte");
  const cfgR = await c.match("./__alerte.json"); if (!cfgR) return;
  const cfg = await cfgR.json(); if (!cfg.on || !(cfg.watch || []).length) return;
  const seenR = await c.match("./__alerte-vazute.json"); const seen = new Set(seenR ? await seenR.json() : []);
  const d = await (await fetch("data.json", { cache: "no-store" })).json();
  const today = new Date().toISOString().slice(0, 10), hits = [];
  for (const it of d.offers || []) {
    if (it.to < today || it.from > today) continue;
    const k = it.store + "|" + it.name + "|" + it.size;
    if (seen.has(k)) continue;
    if (cfg.watch.some(w => goodMatch(w, it.name) || (it.q && goodMatch(w, it.q + " " + it.name)))) { seen.add(k); hits.push(it); }
  }
  await c.put("./__alerte-vazute.json", new Response(JSON.stringify([...seen])));
  if (!hits.length) return;
  const body = hits.slice(0, 4).map(it => `${NAMES[it.store] || it.store}: ${it.name}${it.price != null ? " — " + it.price.toFixed(2).replace(".", ",") + " lei" : ""}`).join("\n") + (hits.length > 4 ? `\n… și încă ${hits.length - 4}` : "");
  await self.registration.showNotification(`🔔 ${hits.length} ${hits.length === 1 ? "ofertă" : "oferte"} la produsele urmărite`, { body, icon: "icon-192.png", badge: "icon-192.png", tag: "oferte-urmarite", data: { url: "./#alerte" } });
}
self.addEventListener("periodicsync", e => { if (e.tag === "oferte-alerte") e.waitUntil(checkAlerts().catch(() => {})); });
self.addEventListener("notificationclick", e => {
  e.notification.close();
  e.waitUntil(self.clients.matchAll({ type: "window" }).then(ws => { for (const w of ws) if ("focus" in w) { w.navigate && w.navigate("./#alerte"); return w.focus(); } return self.clients.openWindow("./#alerte"); }));
});
