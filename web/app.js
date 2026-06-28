// OpenMusic frontend — form state, generate, polling refresh, audio player.
// All API calls are relative so the app works behind AgentDeck's /agent/openmusic/ proxy.
(() => {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[m]));
  const fmtTime = (s) => { s = Math.max(0, s | 0); return `${(s / 60) | 0}:${String(s % 60).padStart(2, "0")}`; };
  // Build a media URL safely: id is percent-encoded for the URL path (defense-in-depth vs odd ids).
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
    // Instrumental always uses custom mode (kie requires style+title for instrumental, never a lyrics prompt).
    const customMode = advanced || instrumental;
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
      const actions =
        (s.status === "done" && s.hasAudio ? `<button class="om-card-play" data-play="${esc(s.id)}">▶</button>` : "") +
        (s.status !== "generating" ? `<button class="om-card-del" data-del="${esc(s.id)}" title="删除">✕</button>` : "");
      return `<div class="om-card ${esc(s.status)}">
        <div class="om-cover">${cover ? `<img src="${esc(cover)}" alt="">` : "♪"}</div>
        <div class="om-card-body">
          <div class="om-card-title">${esc(s.title || "Untitled")}</div>
          <div class="om-card-sub">${sub}</div>
        </div>
        <span class="om-card-badge">${esc(badge)}</span>${actions}
      </div>`;
    }).join("");
    el.querySelectorAll("[data-play]").forEach((b) => b.onclick = () => play(b.dataset.play));
    el.querySelectorAll("[data-del]").forEach((b) => b.onclick = () => del(b.dataset.del));
  }

  async function del(id) {
    try { await fetch("api/songs/" + encodeURIComponent(id), { method: "DELETE" }); } catch (_) { /* best-effort */ }
    if (state.playing === id) { audio.pause(); state.playing = null; $("om-player").hidden = true; }
    refresh();
  }

  // ---- player ----
  const audio = $("om-audio");
  function play(id) {
    const s = state.songs.find((x) => x.id === id);
    if (!s) return;
    state.playing = id;
    audio.src = mediaURL(id, "mp3");
    audio.play();
    $("om-player").hidden = false;
    $("om-now-title").textContent = s.title || "Untitled";
    $("om-now-style").textContent = s.style || s.tags || "";
    $("om-now-cover").src = s.hasCover ? mediaURL(id, "jpg") : "";
    $("om-play").textContent = "⏸";
  }
  $("om-play").onclick = () => { if (audio.paused) { audio.play(); $("om-play").textContent = "⏸"; } else { audio.pause(); $("om-play").textContent = "▶"; } };
  const playable = () => state.songs.filter((s) => s.status === "done" && s.hasAudio);
  const step = (d) => { const ps = playable(); const i = ps.findIndex((s) => s.id === state.playing); if (ps.length) play(ps[(i + d + ps.length) % ps.length].id); };
  $("om-next").onclick = () => step(1);
  $("om-prev").onclick = () => step(-1);
  audio.ontimeupdate = () => { $("om-cur").textContent = fmtTime(audio.currentTime); if (audio.duration) $("om-bar").value = (audio.currentTime / audio.duration) * 100; };
  audio.onloadedmetadata = () => $("om-dur").textContent = fmtTime(audio.duration);
  audio.onended = () => step(1);
  $("om-bar").oninput = (e) => { if (audio.duration) audio.currentTime = (e.target.value / 100) * audio.duration; };

  refresh(); // initial load (and resume polling if something is still generating)
  if (state.songs?.some?.((s) => s.status === "generating")) startPolling();
})();
