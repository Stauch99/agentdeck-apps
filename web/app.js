// OpenMusic frontend — beginner-friendly step wizard (left panel), workspace list, persistent player, lyrics modal.
// All API calls are relative so the app works behind AgentDeck's /agent/openmusic/ proxy.
(() => {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[m]));
  const fmtTime = (s) => { s = Math.max(0, s | 0); return `${(s / 60) | 0}:${String(s % 60).padStart(2, "0")}`; };
  const mediaURL = (id, ext) => "media/" + encodeURIComponent(id) + "." + ext;

  const state = { playing: null, songs: [] };

  // ============ STEP WIZARD (beginner-friendly, no jargon) ============
  // Plain-language screens; the assembled GenerateRequest stays hidden. Model / weirdness / style-influence
  // use sensible defaults, the song name auto-fills if left blank, and "style" is picked via friendly chips.
  const WIZ_MODES = [
    { k: "write", icon: "✎", t: "我自己写词", d: "把想好的词写进去" },
    { k: "prompt", icon: "✨", t: "我说想法,帮我写词", d: "描述一下就行" },
    { k: "instrumental", icon: "♪", t: "纯音乐就好", d: "只要旋律,不唱" },
  ];
  // friendly genre chips — label shown to the user, value is the style wording Suno understands best
  const WIZ_GENRES = [
    ["流行", "pop"], ["民谣", "acoustic folk"], ["电子", "electronic, synth"],
    ["摇滚", "rock"], ["嘻哈", "hip hop"], ["R&B", "R&B, soul"],
    ["古风", "chinese style, traditional"], ["治愈", "lo-fi, chill, soft"],
    ["浪漫", "romantic ballad"], ["欢快", "upbeat, happy"],
  ];
  const WIZ_DEFAULTS = { step: 0, mode: "", prompt: "", styleText: "", vocal: "", title: "", busy: false };
  const wiz = { ...WIZ_DEFAULTS, genres: [] };

  const wizCustom = () => wiz.mode === "write" || wiz.mode === "instrumental";
  const wizStyle = () => wiz.genres.map((i) => WIZ_GENRES[i][1]).concat(wiz.styleText.trim() ? [wiz.styleText.trim()] : []).join(", ");
  function wizSteps() {
    const s = ["mode"];
    if (wiz.mode !== "instrumental") s.push("words");
    s.push("vibe", "name");
    return s;
  }
  function canAdvance(k) {
    if (k === "mode") return !!wiz.mode;
    if (k === "words") return wiz.prompt.trim() !== "";
    if (k === "vibe") return wizCustom() ? wizStyle() !== "" : true; // 自己写词/纯音乐需要一个风格;AI 写词可跳过
    return true; // 起名永远可选(留空会自动起)
  }

  function wizBody(k) {
    if (k === "mode") {
      return `<div class="om-wiz-q">想做一首什么样的歌?</div>
        <div class="om-choices">${WIZ_MODES.map((m) => `
          <button class="om-choice${wiz.mode === m.k ? " on" : ""}" data-mode="${m.k}">
            <span class="om-choice-ic">${m.icon}</span>
            <span class="om-choice-tx"><b>${m.t}</b><i>${m.d}</i></span>
          </button>`).join("")}</div>`;
    }
    if (k === "words") {
      const write = wiz.mode === "write";
      return `<div class="om-wiz-q">${write ? "把你的词写在这里" : "这首歌想讲什么?"}</div>
        <textarea class="om-text om-grow" id="wz-prompt" placeholder="${write ? "一行一句,空一行换段…" : "随便说说,比如:一个关于夏天和海边的小故事…"}">${esc(wiz.prompt)}</textarea>`;
    }
    if (k === "vibe") {
      const chips = WIZ_GENRES.map(([label], i) => `<button class="om-chip${wiz.genres.includes(i) ? " on" : ""}" data-genre="${i}">${label}</button>`).join("");
      const vocal = wiz.mode === "instrumental" ? "" : `<div class="om-wiz-sub">想要谁来唱?</div>
        <div class="om-seg-mini" id="wz-vocal">
          <button data-vocal=""${wiz.vocal === "" ? ' class="on"' : ""}>都行</button>
          <button data-vocal="m"${wiz.vocal === "m" ? ' class="on"' : ""}>男声</button>
          <button data-vocal="f"${wiz.vocal === "f" ? ' class="on"' : ""}>女声</button>
        </div>`;
      return `<div class="om-wiz-q">想要什么感觉?${wizCustom() ? "" : ` <span class="om-opt">(可跳过)</span>`}</div>
        <div class="om-chips">${chips}</div>
        <textarea class="om-text" id="wz-styletext" rows="3" placeholder="也可以用大白话说说,比如:轻快一点,适合开车时听…">${esc(wiz.styleText)}</textarea>
        ${vocal}`;
    }
    return `<div class="om-wiz-q">给它起个名字吧 <span class="om-opt">(可留空,我帮你起)</span></div>
      <input class="om-input" id="wz-title" placeholder="比如:夏天的风" value="${esc(wiz.title)}" />
      <div class="om-wiz-hint">点「生成」就开始,大概要等一两分钟。</div>`;
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
    bind("wz-prompt", "prompt"); bind("wz-styletext", "styleText"); bind("wz-title", "title");
    root.querySelectorAll("[data-genre]").forEach((b) => b.onclick = () => {
      const i = +b.dataset.genre, at = wiz.genres.indexOf(i);
      if (at >= 0) wiz.genres.splice(at, 1); else wiz.genres.push(i);
      b.classList.toggle("on");
      updateNext(k);
    });
    const vocal = $("wz-vocal");
    if (vocal) vocal.querySelectorAll("[data-vocal]").forEach((b) => b.onclick = () => { wiz.vocal = b.dataset.vocal; vocal.querySelectorAll("button").forEach((x) => x.classList.toggle("on", x === b)); });
    if ($("wz-back")) $("wz-back").onclick = () => { if (wiz.step > 0) { wiz.step--; renderWiz(); } };
    if ($("wz-next")) $("wz-next").onclick = () => { if (last) submitWiz(); else { wiz.step++; renderWiz(); } };
  }

  async function submitWiz() {
    const instrumental = wiz.mode === "instrumental";
    let title = wiz.title.trim();
    if (!title && wizCustom()) { // custom mode (write/instrumental) needs a title; AI-write lets kie name it if blank
      const firstLine = (wiz.prompt || "").split("\n").map((s) => s.trim()).find((s) => s && !s.startsWith("["));
      title = (firstLine || "我的作品").slice(0, 24);
    }
    const body = {
      customMode: wizCustom(),
      model: "V4_5ALL",
      instrumental,
      prompt: instrumental ? "" : wiz.prompt.trim(),
      style: wizStyle(),
      negativeTags: "",
      title,
      vocalGender: wiz.vocal,
      styleWeight: 0.5,
      weirdnessConstraint: 0.5,
    };
    wiz.busy = true;
    const n = $("wz-next"); if (n) { n.disabled = true; n.textContent = "生成中…"; }
    try {
      const res = await fetch("api/generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      if (!res.ok) { const e = await res.json().catch(() => ({})); alert("没成功,再试一次:" + (e.error || res.status)); wiz.busy = false; renderWiz(); return; }
      Object.assign(wiz, WIZ_DEFAULTS, { genres: [] });
      startPolling();
    } catch (e) { alert("网络好像断了,检查下连接:" + e); }
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
      const badge = s.status === "generating" ? "生成中…" : s.status === "error" ? "失败" : "";
      const sub = s.status === "error" ? esc(s.errorMessage || "没生成成功") : esc(s.style || s.tags || "");
      const playing = state.playing === s.id ? " playing" : "";
      const actions =
        (s.status === "done" && s.hasAudio ? `<button class="om-card-play" data-play="${esc(s.id)}">▶</button>` : "") +
        (s.status !== "generating" ? `<button class="om-card-del" data-del="${esc(s.id)}" title="删除">✕</button>` : "");
      return `<div class="om-card ${esc(s.status)}${playing}">
        <div class="om-cover">${cover ? `<img src="${esc(cover)}" alt="">` : "♪"}</div>
        <div class="om-card-body" data-lyrics="${esc(s.id)}" title="查看歌词">
          <div class="om-card-title">${esc(s.title || "未命名")}</div>
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
    $("om-lyrics-title").textContent = s.title || "未命名";
    $("om-lyrics-style").textContent = s.style || s.tags || "";
    const ly = (s.lyrics || "").trim();
    $("om-lyrics-body").textContent = ly && ly !== "[Instrumental]" ? ly
      : s.status === "generating" ? "生成中,歌词稍后可见…" : "纯音乐 · 没有歌词";
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
    $("om-now-title").textContent = s.title || "未命名";
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
