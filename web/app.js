// OpenMusic frontend — form state, generate, polling refresh, persistent audio player, lyrics modal.
// All API calls are relative so the app works behind AgentDeck's /agent/openmusic/ proxy.
(() => {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[m]));
  const fmtTime = (s) => { s = Math.max(0, s | 0); return `${(s / 60) | 0}:${String(s % 60).padStart(2, "0")}`; };
  const mediaURL = (id, ext) => "media/" + encodeURIComponent(id) + "." + ext;

  const state = { mode: "advanced", lyric: "write", vocal: "", playing: null, songs: [] };

  // ---- left console: segmented controls ----
  document.querySelectorAll("[data-mode]").forEach((b) => b.onclick = () => {
    state.mode = b.dataset.mode;
    document.querySelectorAll("[data-mode]").forEach((x) => x.classList.toggle("on", x === b));
  });
  document.querySelectorAll("[data-lyric]").forEach((b) => b.onclick = () => {
    state.lyric = b.dataset.lyric;
    document.querySelectorAll("[data-lyric]").forEach((x) => x.classList.toggle("on", x === b));
    $("om-prompt").disabled = state.lyric === "instrumental";
    $("om-prompt").placeholder = state.lyric === "write" ? "Write your lyrics…"
      : state.lyric === "prompt" ? "Describe the song; AI writes the lyrics…" : "Instrumental — no lyrics";
  });
  document.querySelectorAll("[data-vocal]").forEach((b) => b.onclick = () => {
    state.vocal = b.dataset.vocal;
    document.querySelectorAll("[data-vocal]").forEach((x) => x.classList.toggle("on", x === b));
  });
  $("om-weird").oninput = (e) => $("om-weird-v").textContent = e.target.value + "%";
  $("om-styleinf").oninput = (e) => $("om-style-v").textContent = e.target.value + "%";

  // ---- create ----
  $("om-create").onclick = async () => {
    const advanced = state.mode === "advanced";
    const instrumental = state.lyric === "instrumental";
    const customMode = advanced || instrumental; // instrumental always custom (kie needs style+title, no lyrics prompt)
    const body = {
      customMode,
      model: $("om-model").value,
      instrumental,
      prompt: instrumental ? "" : $("om-prompt").value.trim(),
      style: $("om-style").value.trim(),
      negativeTags: $("om-negative").value.trim(),
      title: $("om-title").value.trim(),
      vocalGender: state.vocal,
      styleWeight: (+$("om-styleinf").value) / 100,
      weirdnessConstraint: (+$("om-weird").value) / 100,
    };
    const btn = $("om-create"); btn.disabled = true; btn.textContent = "Creating…";
    try {
      const res = await fetch("api/generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      if (!res.ok) { const e = await res.json().catch(() => ({})); alert("Generate failed: " + (e.error || res.status)); }
      else { startPolling(); }
    } catch (e) { alert("Network error: " + e); }
    finally { btn.disabled = false; btn.textContent = "Create"; }
  };

  // ---- list + polling ----
  async function refresh() {
    try {
      const data = await (await fetch("api/songs")).json();
      state.songs = data.songs || [];
      renderList();
      if (!state.playing) setIdle();
      return state.songs.some((s) => s.status === "generating");
    } catch { return false; }
  }
  let pollTimer = null;
  function startPolling() {
    if (pollTimer) return;
    const tick = async () => { const busy = await refresh(); if (!busy) { clearInterval(pollTimer); pollTimer = null; } };
    pollTimer = setInterval(tick, 4000); tick();
  }

  function renderList() {
    const el = $("om-list");
    el.innerHTML = state.songs.map((s) => {
      const cover = s.hasCover ? mediaURL(s.id, "jpg") : "";
      const badge = s.status === "generating" ? "生成中…" : s.status === "error" ? "失败" : (s.model || "");
      const sub = s.status === "error" ? esc(s.errorMessage || "generation failed") : esc(s.style || s.tags || "");
      const playing = state.playing === s.id ? " playing" : "";
      const actions =
        (s.status === "done" && s.hasAudio ? `<button class="om-card-play" data-play="${esc(s.id)}">▶</button>` : "") +
        (s.status !== "generating" ? `<button class="om-card-del" data-del="${esc(s.id)}" title="删除">✕</button>` : "");
      return `<div class="om-card ${esc(s.status)}${playing}">
        <div class="om-cover">${cover ? `<img src="${esc(cover)}" alt="">` : "♪"}</div>
        <div class="om-card-body" data-lyrics="${esc(s.id)}" title="查看歌词">
          <div class="om-card-title">${esc(s.title || "Untitled")}</div>
          <div class="om-card-sub">${sub}</div>
        </div>
        <span class="om-card-badge">${esc(badge)}</span>${actions}
      </div>`;
    }).join("");
    el.querySelectorAll("[data-play]").forEach((b) => b.onclick = (e) => { e.stopPropagation(); play(b.dataset.play); });
    el.querySelectorAll("[data-del]").forEach((b) => b.onclick = (e) => { e.stopPropagation(); del(b.dataset.del); });
    el.querySelectorAll("[data-lyrics]").forEach((b) => b.onclick = () => showLyrics(b.dataset.lyrics));
  }

  async function del(id) {
    try { await fetch("api/songs/" + encodeURIComponent(id), { method: "DELETE" }); } catch (_) { /* best-effort */ }
    if (state.playing === id) { audio.pause(); state.playing = null; setIdle(); }
    refresh();
  }

  // ---- lyrics modal ----
  function showLyrics(id) {
    const s = state.songs.find((x) => x.id === id);
    if (!s) return;
    $("om-lyrics-title").textContent = s.title || "Untitled";
    $("om-lyrics-style").textContent = s.style || s.tags || "";
    const ly = (s.lyrics || "").trim();
    $("om-lyrics-body").textContent = ly && ly !== "[Instrumental]" ? ly
      : s.status === "generating" ? "生成中,歌词稍后可见…" : "纯音乐 · 无歌词";
    $("om-lyrics").hidden = false;
  }
  const closeLyrics = () => { $("om-lyrics").hidden = true; };
  $("om-lyrics-x").onclick = closeLyrics;
  $("om-lyrics").onclick = (e) => { if (e.target === $("om-lyrics")) closeLyrics(); };
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeLyrics(); });

  // ---- persistent player ----
  const audio = $("om-audio");
  function setNowCover(url) {
    const el = $("om-now-cover");
    if (url) { el.style.backgroundImage = `url("${url}")`; el.textContent = ""; }
    else { el.style.backgroundImage = ""; el.textContent = "♪"; }
  }
  function setIdle() {
    $("om-player").classList.add("idle");
    $("om-now-title").textContent = "—";
    $("om-now-style").textContent = "";
    setNowCover("");
    $("om-play").textContent = "▶";
  }
  function play(id) {
    const s = state.songs.find((x) => x.id === id);
    if (!s || !s.hasAudio) return;
    state.playing = id;
    audio.src = mediaURL(id, "mp3");
    audio.play().catch(() => {}); // ignore autoplay rejection; onplay/onpause keep the icon honest
    $("om-player").classList.remove("idle");
    $("om-now-title").textContent = s.title || "Untitled";
    $("om-now-style").textContent = s.style || s.tags || "";
    setNowCover(s.hasCover ? mediaURL(id, "jpg") : "");
    renderList(); // reflect the "playing" highlight
  }
  const playable = () => state.songs.filter((s) => s.status === "done" && s.hasAudio);
  $("om-play").onclick = () => {
    if (!state.playing) { const ps = playable(); if (ps.length) play(ps[0].id); return; }
    if (audio.paused) audio.play().catch(() => {}); else audio.pause();
  };
  const step = (d) => { const ps = playable(); if (!ps.length) return; const i = ps.findIndex((s) => s.id === state.playing); play(ps[(i + d + ps.length) % ps.length].id); };
  $("om-next").onclick = () => step(1);
  $("om-prev").onclick = () => step(-1);
  // the play/pause icon mirrors the REAL audio state — fixes the desync where it lied after a pause/end
  audio.onplay = () => { $("om-play").textContent = "⏸"; };
  audio.onpause = () => { $("om-play").textContent = "▶"; };
  audio.ontimeupdate = () => { $("om-cur").textContent = fmtTime(audio.currentTime); if (audio.duration) $("om-bar").value = (audio.currentTime / audio.duration) * 100; };
  audio.onloadedmetadata = () => $("om-dur").textContent = fmtTime(audio.duration);
  audio.onended = () => step(1);
  $("om-bar").oninput = (e) => { if (audio.duration) audio.currentTime = (e.target.value / 100) * audio.duration; };

  setIdle();
  refresh();
  if (state.songs?.some?.((s) => s.status === "generating")) startPolling();
})();
