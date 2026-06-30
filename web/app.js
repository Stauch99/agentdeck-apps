// OpenMusic frontend — step-by-step creation wizard (left panel), workspace list, persistent player, lyrics modal.
// All API calls are relative so the app works behind AgentDeck's /agent/openmusic/ proxy.
(() => {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[m]));
  const fmtTime = (s) => { s = Math.max(0, s | 0); return `${(s / 60) | 0}:${String(s % 60).padStart(2, "0")}`; };
  const mediaURL = (id, ext) => "media/" + encodeURIComponent(id) + "." + ext;

  const state = { playing: null, songs: [] };

  // ============ STEP WIZARD ============
  // Each "Create" walks the user through the params one screen at a time, then assembles the same
  // GenerateRequest the old dense form produced. customMode is inferred (write/instrumental need it).
  const WIZ_MODES = [
    { k: "write", icon: "✎", t: "我自己写词", d: "把歌词敲进去" },
    { k: "prompt", icon: "✨", t: "给个想法,AI 写词", d: "描述就行,AI 来填词" },
    { k: "instrumental", icon: "♪", t: "纯音乐,不要词", d: "只要旋律" },
  ];
  const WIZ_MODELS = [["V4_5ALL", "v4.5-all"], ["V5_5", "v5.5"], ["V4_5PLUS", "v4.5-plus"], ["V4_5", "v4.5"], ["V4", "v4"], ["V3_5", "v3.5"]];
  const WIZ_DEFAULTS = { step: 0, mode: "", prompt: "", style: "", negative: "", vocal: "", model: "V4_5ALL", weird: 50, styleInf: 50, title: "", busy: false };
  const wiz = { ...WIZ_DEFAULTS };

  // write/instrumental carry lyrics-or-no-vocals, which kie only honors in customMode (=> style+title required).
  const wizCustom = () => wiz.mode === "write" || wiz.mode === "instrumental";
  function wizSteps() {
    const s = ["mode"];
    if (wiz.mode !== "instrumental") s.push("words");
    s.push("style", "tune");
    return s;
  }
  function canAdvance(k) {
    if (k === "mode") return !!wiz.mode;
    if (k === "words") return wiz.prompt.trim() !== "";
    if (k === "style") return wizCustom() ? wiz.style.trim() !== "" : true;
    if (k === "tune") return wizCustom() ? wiz.title.trim() !== "" : true;
    return true;
  }

  function wizBody(k) {
    if (k === "mode") {
      return `<div class="om-wiz-q">你想怎么开始这首歌?</div>
        <div class="om-choices">${WIZ_MODES.map((m) => `
          <button class="om-choice${wiz.mode === m.k ? " on" : ""}" data-mode="${m.k}">
            <span class="om-choice-ic">${m.icon}</span>
            <span class="om-choice-tx"><b>${m.t}</b><i>${m.d}</i></span>
          </button>`).join("")}</div>`;
    }
    if (k === "words") {
      const write = wiz.mode === "write";
      return `<div class="om-wiz-q">${write ? "写下你的歌词" : "描述这首歌讲什么"}</div>
        <textarea class="om-text om-grow" id="wz-prompt" placeholder="${write ? "一行一句,空行分段…" : "比如:一首关于深夜城市霓虹与追梦的歌…"}">${esc(wiz.prompt)}</textarea>`;
    }
    if (k === "style") {
      const opt = wizCustom() ? "" : ` <span class="om-opt">(可留空)</span>`;
      return `<div class="om-wiz-q">想要什么风格氛围?${opt}</div>
        <textarea class="om-text" id="wz-style" rows="4" placeholder="e.g. synthwave, retro 80s, warm analog pads, 110 BPM">${esc(wiz.style)}</textarea>
        <input class="om-input" id="wz-negative" placeholder="不想要的风格(可选)" value="${esc(wiz.negative)}" />
        ${wiz.mode === "instrumental" ? "" : `<div class="om-wiz-sub">人声</div>
          <div class="om-seg-mini" id="wz-vocal">
            <button data-vocal=""${wiz.vocal === "" ? ' class="on"' : ""}>自动</button>
            <button data-vocal="m"${wiz.vocal === "m" ? ' class="on"' : ""}>男声</button>
            <button data-vocal="f"${wiz.vocal === "f" ? ' class="on"' : ""}>女声</button>
          </div>`}`;
    }
    const titleOpt = wizCustom() ? "" : ` <span class="om-opt">(可选)</span>`;
    return `<div class="om-wiz-q">最后,微调和命名 <span class="om-opt">(可跳过)</span></div>
      <div class="om-wiz-sub">模型</div>
      <select class="om-input" id="wz-model">${WIZ_MODELS.map(([v, l]) => `<option value="${v}"${wiz.model === v ? " selected" : ""}>${l}</option>`).join("")}</select>
      <div class="om-wiz-sub">Weirdness <b id="wz-weird-v">${wiz.weird}%</b></div>
      <input class="om-slider" id="wz-weird" type="range" min="0" max="100" value="${wiz.weird}" />
      <div class="om-wiz-sub">Style Influence <b id="wz-styleinf-v">${wiz.styleInf}%</b></div>
      <input class="om-slider" id="wz-styleinf" type="range" min="0" max="100" value="${wiz.styleInf}" />
      <div class="om-wiz-sub">歌名${titleOpt}</div>
      <input class="om-input" id="wz-title" placeholder="${wizCustom() ? "给它起个名字" : "可留空"}" value="${esc(wiz.title)}" />`;
  }

  function renderWiz() {
    const steps = wizSteps();
    if (wiz.step >= steps.length) wiz.step = steps.length - 1;
    const k = steps[wiz.step];
    const last = wiz.step === steps.length - 1;
    const dots = steps.map((_, i) => `<b class="${i === wiz.step ? "on" : i < wiz.step ? "done" : ""}"></b>`).join("");
    $("om-wiz").innerHTML = `
      <div class="om-wiz-head">
        <span class="om-wiz-step">${wiz.step + 1} / ${steps.length}</span>
        <span class="om-dots">${dots}</span>
      </div>
      <div class="om-wiz-body">${wizBody(k)}</div>
      <div class="om-wiz-foot">
        ${wiz.step > 0 ? `<button class="om-wiz-back" id="wz-back">← 上一步</button>` : `<span></span>`}
        <button class="om-wiz-next" id="wz-next">${last ? "✨ 生成" : "下一步 →"}</button>
      </div>`;
    bindWiz(k, last);
    updateNext(k);
  }

  const updateNext = (k) => { const n = $("wz-next"); if (n) n.disabled = wiz.busy || !canAdvance(k); };

  function bindWiz(k, last) {
    const root = $("om-wiz");
    root.querySelectorAll("[data-mode]").forEach((b) => b.onclick = () => { wiz.mode = b.dataset.mode; wiz.step = 1; renderWiz(); });
    const bind = (id, key) => { const e = $(id); if (e) e.oninput = () => { wiz[key] = e.value; updateNext(k); }; };
    bind("wz-prompt", "prompt"); bind("wz-style", "style"); bind("wz-negative", "negative"); bind("wz-title", "title");
    if ($("wz-model")) $("wz-model").onchange = (e) => wiz.model = e.target.value;
    if ($("wz-weird")) $("wz-weird").oninput = (e) => { wiz.weird = +e.target.value; $("wz-weird-v").textContent = wiz.weird + "%"; };
    if ($("wz-styleinf")) $("wz-styleinf").oninput = (e) => { wiz.styleInf = +e.target.value; $("wz-styleinf-v").textContent = wiz.styleInf + "%"; };
    const vocal = $("wz-vocal");
    if (vocal) vocal.querySelectorAll("[data-vocal]").forEach((b) => b.onclick = () => { wiz.vocal = b.dataset.vocal; vocal.querySelectorAll("button").forEach((x) => x.classList.toggle("on", x === b)); });
    if ($("wz-back")) $("wz-back").onclick = () => { if (wiz.step > 0) { wiz.step--; renderWiz(); } };
    if ($("wz-next")) $("wz-next").onclick = () => { if (last) submitWiz(); else { wiz.step++; renderWiz(); } };
  }

  async function submitWiz() {
    const instrumental = wiz.mode === "instrumental";
    const body = {
      customMode: wizCustom(),
      model: wiz.model,
      instrumental,
      prompt: instrumental ? "" : wiz.prompt.trim(),
      style: wiz.style.trim(),
      negativeTags: wiz.negative.trim(),
      title: wiz.title.trim(),
      vocalGender: wiz.vocal,
      styleWeight: wiz.styleInf / 100,
      weirdnessConstraint: wiz.weird / 100,
    };
    wiz.busy = true;
    const n = $("wz-next"); if (n) { n.disabled = true; n.textContent = "生成中…"; }
    try {
      const res = await fetch("api/generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      if (!res.ok) { const e = await res.json().catch(() => ({})); alert("生成失败: " + (e.error || res.status)); wiz.busy = false; renderWiz(); return; }
      Object.assign(wiz, WIZ_DEFAULTS); // reset to step 1 for the next song
      startPolling();
    } catch (e) { alert("网络错误: " + e); }
    wiz.busy = false;
    renderWiz();
  }

  // ============ workspace list + polling ============
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

  // ============ lyrics modal ============
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

  // ============ persistent player ============
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
    audio.play().catch(() => {});
    $("om-player").classList.remove("idle");
    $("om-now-title").textContent = s.title || "Untitled";
    $("om-now-style").textContent = s.style || s.tags || "";
    setNowCover(s.hasCover ? mediaURL(id, "jpg") : "");
    renderList();
  }
  const playable = () => state.songs.filter((s) => s.status === "done" && s.hasAudio);
  $("om-play").onclick = () => {
    if (!state.playing) { const ps = playable(); if (ps.length) play(ps[0].id); return; }
    if (audio.paused) audio.play().catch(() => {}); else audio.pause();
  };
  const step = (d) => { const ps = playable(); if (!ps.length) return; const i = ps.findIndex((s) => s.id === state.playing); play(ps[(i + d + ps.length) % ps.length].id); };
  $("om-next").onclick = () => step(1);
  $("om-prev").onclick = () => step(-1);
  audio.onplay = () => { $("om-play").textContent = "⏸"; };
  audio.onpause = () => { $("om-play").textContent = "▶"; };
  audio.ontimeupdate = () => { $("om-cur").textContent = fmtTime(audio.currentTime); if (audio.duration) $("om-bar").value = (audio.currentTime / audio.duration) * 100; };
  audio.onloadedmetadata = () => $("om-dur").textContent = fmtTime(audio.duration);
  audio.onended = () => step(1);
  $("om-bar").oninput = (e) => { if (audio.duration) audio.currentTime = (e.target.value / 100) * audio.duration; };

  // ============ init ============
  renderWiz();
  setIdle();
  refresh();
  if (state.songs?.some?.((s) => s.status === "generating")) startPolling();
})();
