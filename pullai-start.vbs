<!DOCTYPE html>
<html lang="it">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PullAI</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Söhne:wght@300;400;450;500&family=Söhne+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
:root {
  --bg: #ffffff;
  --bg2: #f9f9f9;
  --bg3: #f4f4f4;
  --border: #e5e5e5;
  --border2: #d0d0d0;
  --text: #0d0d0d;
  --text2: #555;
  --text3: #999;
  --accent: #10a37f;
  --accent-light: #e6f7f2;
  --sans: 'Söhne', 'Helvetica Neue', Arial, sans-serif;
  --mono: 'Söhne Mono', 'JetBrains Mono', monospace;
  --radius: 16px;
  --radius-sm: 10px;
  --radius-xs: 8px;
}

*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html, body { height: 100%; overflow: hidden; background: var(--bg); color: var(--text); font-family: var(--sans); font-size: 15px; }

::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--border2); border-radius: 3px; }

/* ── LAYOUT ─────────────────────────────────── */
.app { display: flex; height: 100vh; }

/* Sidebar */
.sidebar {
  width: 260px; flex-shrink: 0;
  background: var(--bg3);
  border-right: 1px solid var(--border);
  display: flex; flex-direction: column;
  padding: 12px 8px;
  overflow: hidden;
}
.sidebar-logo {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 10px; margin-bottom: 4px;
}
.logo-mark {
  width: 32px; height: 32px; background: var(--text); border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  font-family: var(--mono); font-size: 11px; font-weight: 500; color: #fff;
}
.logo-name { font-weight: 500; font-size: 15px; }
.logo-ver  { font-size: 11px; color: var(--text3); font-family: var(--mono); margin-left: 2px; }

.new-chat-btn {
  display: flex; align-items: center; gap: 8px;
  width: 100%; padding: 9px 12px; border-radius: var(--radius-xs);
  border: 1px solid var(--border); background: var(--bg);
  cursor: pointer; font-size: 14px; color: var(--text2);
  font-family: var(--sans); transition: all .14s; margin-bottom: 8px;
}
.new-chat-btn:hover { background: var(--bg3); border-color: var(--border2); color: var(--text); }

.sidebar-section { font-size: 11px; font-weight: 500; color: var(--text3); padding: 8px 10px 4px; letter-spacing: .04em; text-transform: uppercase; }

.history-list { flex: 1; overflow-y: auto; }
.hist-item {
  padding: 7px 10px; border-radius: var(--radius-xs);
  cursor: pointer; font-size: 13px; color: var(--text2);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  transition: background .1s;
}
.hist-item:hover { background: var(--border); color: var(--text); }
.hist-item.active { background: var(--border); color: var(--text); }

.sidebar-footer {
  border-top: 1px solid var(--border);
  padding-top: 10px; margin-top: 8px;
  font-size: 12px; color: var(--text3);
  display: flex; align-items: center; gap: 8px;
  padding: 10px 10px 4px;
}
.hw-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); flex-shrink: 0; }
.hw-dot.off { background: #e04040; }

/* ── MAIN ────────────────────────────────────── */
.main {
  flex: 1; display: flex; flex-direction: column;
  overflow: hidden; background: var(--bg);
}

/* Canvas */
.canvas {
  flex: 1; overflow-y: auto;
  padding: 32px 0 0;
  display: flex; flex-direction: column;
}

/* Welcome screen */
.welcome {
  flex: 1; display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  gap: 16px; padding: 40px 24px 80px;
}
.welcome-logo { width: 52px; height: 52px; background: var(--text); border-radius: 14px; display: flex; align-items: center; justify-content: center; font-family: var(--mono); font-size: 16px; font-weight: 500; color: #fff; }
.welcome-title { font-size: 26px; font-weight: 400; color: var(--text); }
.welcome-sub { font-size: 15px; color: var(--text2); text-align: center; max-width: 380px; line-height: 1.5; }

.suggestion-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 8px; max-width: 520px; width: 100%; }
.suggestion {
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: var(--radius-sm); padding: 14px 16px;
  cursor: pointer; font-size: 13px; color: var(--text2);
  transition: all .14s; text-align: left; font-family: var(--sans);
  line-height: 1.45;
}
.suggestion:hover { background: var(--bg3); border-color: var(--border2); color: var(--text); }
.suggestion b { display: block; font-weight: 500; color: var(--text); margin-bottom: 3px; font-size: 13px; }

/* Messages */
.msgs { padding: 0 0 20px; display: flex; flex-direction: column; gap: 0; }

.msg-group { padding: 18px 0; }
.msg-group:not(:last-child) { border-bottom: 0px; }

.msg-inner { max-width: 720px; margin: 0 auto; padding: 0 28px; }

.msg-user .msg-inner { display: flex; justify-content: flex-end; }
.msg-user .bubble {
  background: var(--bg3); border: 1px solid var(--border);
  border-radius: 20px 20px 4px 20px;
  padding: 12px 18px; max-width: 75%;
  font-size: 15px; line-height: 1.65; white-space: pre-wrap; word-break: break-word;
}

.msg-ai .msg-inner { display: flex; gap: 14px; align-items: flex-start; }
.ai-avatar {
  width: 28px; height: 28px; background: var(--text); border-radius: 7px;
  flex-shrink: 0; display: flex; align-items: center; justify-content: center;
  font-family: var(--mono); font-size: 10px; font-weight: 500; color: #fff;
  margin-top: 3px;
}
.msg-ai .content {
  flex: 1; font-size: 15px; line-height: 1.72;
  white-space: pre-wrap; word-break: break-word;
  color: var(--text);
}
.thinking-block {
  background: #fffbf0; border: 1px solid #f0d060;
  border-radius: var(--radius-xs); padding: 10px 14px; margin-bottom: 12px;
  font-size: 13px; color: #7a6000; font-family: var(--mono); line-height: 1.6;
}
.thinking-label { font-size: 11px; font-weight: 500; margin-bottom: 5px; display: flex; align-items: center; gap: 4px; opacity: .8; }

/* Media output */
.media-wrap { max-width: 720px; margin: 0 auto; padding: 0 28px 16px; }
.media-card {
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: var(--radius-sm); overflow: hidden;
}
.media-card img { width: 100%; max-height: 540px; object-fit: contain; display: block; }
.media-foot { display: flex; align-items: center; justify-content: space-between; padding: 10px 14px; border-top: 1px solid var(--border); background: var(--bg2); }
.media-name { font-size: 12px; font-family: var(--mono); color: var(--text3); }
.dl-btn {
  font-size: 12px; font-weight: 500; padding: 5px 12px;
  border-radius: 20px; border: 1px solid var(--border2);
  background: var(--bg); color: var(--text2); text-decoration: none;
  transition: all .13s; cursor: pointer; font-family: var(--sans);
}
.dl-btn:hover { background: var(--bg3); color: var(--text); }

.audio-wrap { padding: 14px 16px; display: flex; align-items: center; gap: 12px; }
.audio-wrap audio { flex: 1; height: 36px; accent-color: var(--accent); }

.threed-card { padding: 32px; text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }

/* Progress bar */
.progress-wrap {
  margin: 10px 0 6px;
  display: flex; flex-direction: column; gap: 5px;
}
.progress-bar-outer {
  width: 100%; height: 6px; background: var(--border);
  border-radius: 3px; overflow: hidden;
}
.progress-bar-inner {
  height: 100%; background: var(--accent);
  border-radius: 3px;
  transition: width .4s cubic-bezier(.4,0,.2,1);
}
.progress-label {
  font-size: 12px; color: var(--text2);
  display: flex; justify-content: space-between;
}
.progress-msg { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 75%; }

/* Spinner */
.dot-pulse { display: flex; gap: 4px; align-items: center; padding: 6px 0; }
.dot-pulse span { width: 7px; height: 7px; border-radius: 50%; background: var(--text3); animation: dp .9s ease-in-out infinite; }
.dot-pulse span:nth-child(2) { animation-delay: .2s; }
.dot-pulse span:nth-child(3) { animation-delay: .4s; }
@keyframes dp { 0%,60%,100%{opacity:.25;transform:scale(.8)} 30%{opacity:1;transform:scale(1)} }

/* ── INPUT AREA ──────────────────────────────── */
.input-zone {
  padding: 12px 20px 20px;
  background: var(--bg);
  position: relative;
}
.input-zone::before {
  content: ''; position: absolute; top: -24px; left: 0; right: 0; height: 24px;
  background: linear-gradient(transparent, var(--bg));
  pointer-events: none;
}

.input-box {
  max-width: 720px; margin: 0 auto;
  background: var(--bg); border: 1.5px solid var(--border2);
  border-radius: 18px;
  box-shadow: 0 2px 16px rgba(0,0,0,.08);
  transition: border-color .15s, box-shadow .15s;
  position: relative;
}
.input-box.drag-over {
  border-color: var(--accent) !important;
  background: var(--accent-light) !important;
}
.input-box:focus-within {
  border-color: #c0c0c0;
  box-shadow: 0 2px 20px rgba(0,0,0,.12);
}

/* Attachments row inside box */
.attach-row {
  display: flex; flex-wrap: wrap; gap: 6px;
  padding: 10px 14px 0;
}
.attach-chip {
  display: flex; align-items: center; gap: 6px;
  background: var(--bg3); border: 1px solid var(--border);
  border-radius: 20px; padding: 4px 10px 4px 6px;
  font-size: 12px; font-family: var(--mono); color: var(--text2);
  max-width: 200px; animation: chipIn .12s ease;
}
@keyframes chipIn { from { opacity: 0; scale: .9; } to { opacity: 1; scale: 1; } }
.attach-chip img { width: 24px; height: 24px; border-radius: 4px; object-fit: cover; }
.attach-chip-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 130px; }
.attach-chip-rm { cursor: pointer; color: var(--text3); font-size: 14px; line-height: 1; padding: 0 2px; border-radius: 50%; transition: all .1s; }
.attach-chip-rm:hover { background: var(--border); color: var(--text); }

/* Text row */
.text-row {
  display: flex; align-items: flex-end; padding: 10px 12px 10px 16px; gap: 8px;
}
textarea.inp {
  flex: 1; background: transparent; border: none; outline: none;
  color: var(--text); font-size: 15px; font-family: var(--sans);
  resize: none; line-height: 1.6; min-height: 24px; max-height: 200px;
  padding: 2px 0;
}
textarea.inp::placeholder { color: var(--text3); }

.send-btn {
  width: 36px; height: 36px; border-radius: 50%; flex-shrink: 0;
  background: var(--text); border: none; cursor: pointer; color: #fff;
  display: flex; align-items: center; justify-content: center;
  transition: all .15s; align-self: flex-end;
}
.send-btn:hover { background: #333; transform: scale(1.06); }
.send-btn:active { transform: scale(.96); }
.send-btn:disabled { background: var(--bg3); color: var(--text3); cursor: not-allowed; transform: none; }

/* Controls bar below textarea */
.controls-row {
  display: flex; align-items: center; gap: 6px;
  padding: 0 12px 10px 12px; flex-wrap: wrap;
}
.ctrl-btn {
  display: flex; align-items: center; gap: 5px;
  padding: 5px 12px; border-radius: 20px; font-size: 13px;
  border: 1px solid var(--border); background: var(--bg);
  cursor: pointer; color: var(--text2); font-family: var(--sans);
  transition: all .13s; white-space: nowrap; position: relative;
}
.ctrl-btn:hover { background: var(--bg3); border-color: var(--border2); color: var(--text); }
.ctrl-btn.active { background: var(--accent-light); border-color: var(--accent); color: var(--accent); }
.ctrl-btn svg { width: 14px; height: 14px; flex-shrink: 0; }

/* Model/type dropdowns */
.dd-wrap { position: relative; }
.dd-menu {
  position: fixed;
  background: var(--bg); border: 1px solid var(--border2);
  border-radius: var(--radius-sm); box-shadow: 0 8px 32px rgba(0,0,0,.15);
  z-index: 1000; min-width: 220px; overflow: hidden;
  opacity: 0; transform: translateY(6px) scale(.98); pointer-events: none;
  transition: opacity .14s, transform .14s cubic-bezier(.4,0,.2,1);
}
.dd-menu.open { opacity: 1; transform: translateY(0) scale(1); pointer-events: all; }
.dd-item {
  display: flex; align-items: center; gap: 10px;
  padding: 9px 14px; cursor: pointer; font-size: 13px;
  color: var(--text); transition: background .1s;
}
.dd-item:hover { background: var(--bg3); }
.dd-item.sel { background: var(--accent-light); color: var(--accent); }
.dd-item-tag { font-family: var(--mono); font-size: 11px; color: var(--text3); margin-left: auto; }
.dd-sep { height: 1px; background: var(--border); margin: 4px 0; }
.dd-header { padding: 6px 14px 4px; font-size: 11px; font-weight: 500; color: var(--text3); text-transform: uppercase; letter-spacing: .05em; }

/* Attach input */
.attach-input-btn { 
  width: 32px; height: 32px; border-radius: 50%;
  border: 1px solid var(--border); background: transparent;
  cursor: pointer; color: var(--text3);
  display: flex; align-items: center; justify-content: center;
  transition: all .13s;
}
.attach-input-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }

/* ── PANELS ──────────────────────────────────── */
.panel { display: none; }
.panel.on { display: flex; flex-direction: column; flex: 1; overflow: hidden; }
.panel-inner { flex: 1; overflow-y: auto; padding: 32px 28px; max-width: 720px; margin: 0 auto; width: 100%; }
.panel-title { font-size: 20px; font-weight: 500; margin-bottom: 24px; }

/* Models panel */
.model-table { width: 100%; }
.mtrow {
  display: flex; align-items: center; gap: 12px;
  padding: 11px 0; border-bottom: 1px solid var(--border);
  font-size: 14px;
}
.mtrow:last-child { border-bottom: none; }
.type-pill {
  font-size: 11px; font-weight: 500; padding: 2px 9px;
  border-radius: 20px; font-family: var(--mono); flex-shrink: 0;
}
.tp-llm   { background: #dbeafe; color: #1d4ed8; }
.tp-image { background: #ede9fe; color: #6d28d9; }
.tp-audio { background: #ffedd5; color: #c2410c; }
.tp-video { background: #d1fae5; color: #065f46; }
.tp-3d    { background: #fce7f3; color: #9d174d; }

/* Registry */
.reg-filters { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 16px; }
.rf-btn {
  padding: 5px 14px; border-radius: 20px; font-size: 13px;
  border: 1px solid var(--border); background: var(--bg);
  cursor: pointer; color: var(--text2); font-family: var(--sans);
  transition: all .13s;
}
.rf-btn:hover { background: var(--bg3); }
.rf-btn.on { background: var(--text); color: #fff; border-color: var(--text); }

.reg-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px,1fr)); gap: 10px; }
.rcard {
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: var(--radius-sm); padding: 16px;
  display: flex; flex-direction: column; gap: 6px;
  transition: border-color .13s, box-shadow .13s;
}
.rcard:hover { border-color: var(--border2); box-shadow: 0 2px 10px rgba(0,0,0,.06); }
.rcard-name { font-size: 14px; font-weight: 500; }
.rcard-desc { font-size: 12px; color: var(--text2); line-height: 1.5; flex: 1; }
.rcard-foot { display: flex; align-items: center; justify-content: space-between; margin-top: 4px; }
.rcard-sz { font-size: 11px; font-family: var(--mono); color: var(--text3); }

.btn {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 6px 14px; border-radius: 20px; font-size: 13px;
  border: 1px solid var(--border2); background: var(--bg);
  cursor: pointer; color: var(--text2); font-family: var(--sans);
  transition: all .13s;
}
.btn:hover { background: var(--bg3); color: var(--text); }
.btn.primary { background: var(--text); color: #fff; border-color: var(--text); }
.btn.primary:hover { background: #333; }
.btn.danger { color: #e04040; border-color: rgba(224,64,64,.3); }
.btn.danger:hover { background: rgba(224,64,64,.06); }

.search-box {
  width: 100%; padding: 9px 14px; font-size: 14px;
  border: 1px solid var(--border); border-radius: var(--radius-xs);
  background: var(--bg2); color: var(--text); outline: none;
  font-family: var(--sans); margin-bottom: 14px;
  transition: border-color .15s;
}
.search-box:focus { border-color: var(--border2); }

/* Toast */
.toast {
  position: fixed; bottom: 24px; left: 50%;
  transform: translateX(-50%) translateY(80px);
  background: var(--text); color: #fff; border-radius: 24px;
  padding: 9px 20px; font-size: 13px; z-index: 500;
  pointer-events: none; box-shadow: 0 4px 20px rgba(0,0,0,.2);
  transition: transform .22s cubic-bezier(.4,0,.2,1);
}
.toast.show { transform: translateX(-50%) translateY(0); }
.toast.ok { background: #16a34a; }
.toast.err { background: #dc2626; }

/* Dialog */
.overlay {
  display: none; position: fixed; inset: 0;
  background: rgba(0,0,0,.3); backdrop-filter: blur(4px);
  z-index: 300; align-items: center; justify-content: center;
}
.overlay.show { display: flex; }
.dialog {
  background: var(--bg); border: 1px solid var(--border2);
  border-radius: var(--radius); padding: 24px; min-width: 320px; max-width: 400px;
  box-shadow: 0 20px 60px rgba(0,0,0,.15);
  animation: dlgIn .18s ease;
}
@keyframes dlgIn { from { opacity: 0; transform: scale(.95) translateY(8px); } to { opacity: 1; transform: none; } }
.dlg-title { font-weight: 500; font-size: 16px; margin-bottom: 8px; }
.dlg-body { font-size: 14px; color: var(--text2); margin-bottom: 20px; line-height: 1.5; }
.dlg-btns { display: flex; gap: 8px; justify-content: flex-end; }

/* Animations */
@keyframes msgIn { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
.msg-group { animation: msgIn .2s ease; }

/* ── Dark theme ─────────────────────────────────────── */
.dark {
  --bg: #0d0d0d; --bg2: #141414; --bg3: #1e1e1e;
  --border: #2a2a2a; --border2: #383838;
  --text: #f0ede8; --text2: #b0b0b0; --text3: #666;
  --accent: #10a37f; --accent-light: rgba(16,163,127,.15);
}
/* Ensure inputs/selects use CSS vars in dark mode */
.dark input, .dark textarea, .dark select {
  background: var(--bg2) !important;
  color: var(--text) !important;
  border-color: var(--border2) !important;
}
.dark input::placeholder, .dark textarea::placeholder {
  color: var(--text3) !important;
}
.dark .dialog {
  background: var(--bg2) !important;
  color: var(--text) !important;
  border: 1px solid var(--border2);
}
.dark .dlg-title { color: var(--text) !important; }
.dark .dlg-body  { color: var(--text2) !important; }
.dark .overlay   { background: rgba(0,0,0,.6) !important; }
.dark .panel-inner { background: var(--bg) !important; }
.dark .rcard {
  background: var(--bg2) !important;
  border-color: var(--border) !important;
}
.dark .rcard-name, .dark .rcard-desc { color: var(--text) !important; }
.dark .rcard-sz   { color: var(--text3) !important; }
.dark .mtrow      { border-color: var(--border) !important; }
.dark .sidebar    { background: var(--bg2) !important; border-color: var(--border) !important; }
.dark .sidebar-logo { border-color: var(--border) !important; }
.dark .logo-name, .dark .logo-ver { color: var(--text2) !important; }
.dark .logo-mark  { background: var(--accent) !important; }
.dark .hist-item  { color: var(--text2) !important; }
.dark .hist-item:hover { background: var(--bg3) !important; }
.dark .hist-item.active { background: var(--accent-light) !important; color: var(--accent) !important; }
.dark .new-chat-btn { color: var(--text2) !important; border-color: var(--border2) !important; }
.dark .new-chat-btn:hover { background: var(--bg3) !important; }
.dark .main-topbar { background: var(--bg) !important; border-color: var(--border) !important; }
.dark .topbar-title { color: var(--text2) !important; }
.dark .topbar-btn  { background: var(--bg) !important; color: var(--text2) !important; border-color: var(--border2) !important; }
.dark .topbar-btn:hover { background: var(--bg3) !important; }
.dark .input-zone  { background: var(--bg) !important; border-color: var(--border) !important; }
.dark .input-box   { background: var(--bg2) !important; border-color: var(--border2) !important; }
.dark .ctrl-btn    { background: var(--bg) !important; color: var(--text2) !important; border-color: var(--border2) !important; }
.dark .ctrl-btn:hover { background: var(--bg3) !important; }
.dark .dd-menu     { background: var(--bg2) !important; border-color: var(--border2) !important; }
.dark .dd-item:hover { background: var(--bg3) !important; }
.dark .dd-item.sel { background: var(--accent-light) !important; }
.dark .hw-split    { border-color: var(--border2) !important; }
.dark .hw-half     { background: var(--bg) !important; color: var(--text3) !important; }
.dark .hw-half.active { background: var(--text) !important; color: var(--bg) !important; }
.dark .msg-ai .bubble  { background: var(--bg2) !important; color: var(--text) !important; }
.dark .msg-user .bubble { background: var(--accent) !important; color: #fff !important; }
.dark .settings-panel  { background: var(--bg) !important; border-color: var(--border2) !important; }
.dark .settings-header { border-color: var(--border) !important; }
.dark .settings-header-title { color: var(--text) !important; }
.dark .settings-section-title { color: var(--text3) !important; border-color: var(--border) !important; }
.dark .settings-row-label { color: var(--text) !important; }
.dark .settings-row-sub   { color: var(--text3) !important; }
.dark .theme-toggle { border-color: var(--border2) !important; }
.dark .theme-btn    { background: var(--bg) !important; color: var(--text3) !important; }
.dark .theme-btn.active { background: var(--accent) !important; color: #fff !important; }
.dark .ctx-preset   { background: var(--bg) !important; color: var(--text2) !important; border-color: var(--border2) !important; }
.dark .info-row    { border-color: var(--border) !important; }
.dark .info-label  { color: var(--text2) !important; }
.dark .info-value  { color: var(--text3) !important; }
.dark .btn         { background: var(--bg2) !important; color: var(--text) !important; border-color: var(--border2) !important; }
.dark .btn:hover   { background: var(--bg3) !important; }
.dark .btn.primary { background: var(--accent) !important; color: #fff !important; border-color: transparent !important; }
.dark .btn.danger  { background: transparent !important; color: #f87171 !important; border-color: #f87171 !important; }
.dark .panel-title { color: var(--text) !important; }
.dark .welcome-title { color: var(--text) !important; }
.dark .welcome-sub   { color: var(--text3) !important; }
.dark .suggestion { background: var(--bg2) !important; color: var(--text2) !important; border-color: var(--border) !important; }
.dark .suggestion:hover { background: var(--bg3) !important; }
.dark .search-box  { background: var(--bg2) !important; color: var(--text) !important; border-color: var(--border2) !important; }
.dark .reg-filters .btn { background: var(--bg2) !important; }
.dark .toast-ok    { background: #052e16 !important; color: #86efac !important; }
.dark .toast-err   { background: #450a0a !important; color: #fca5a5 !important; }
.dark .toast       { background: var(--bg3) !important; color: var(--text) !important; }
.dark .type-pill   { opacity: .9; }
.dark .progress-bar { background: var(--border2) !important; }
.dark .ctx-slider  { background: var(--border2) !important; }

/* ── Main topbar ──────────────────────────────────────── */
.main-topbar {
  height: 44px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 18px; border-bottom: 1px solid var(--border);
  background: var(--bg);
}
.topbar-left { display: flex; align-items: center; gap: 8px; }
.topbar-right { display: flex; align-items: center; gap: 8px; }
.topbar-title { font-size: 13px; font-weight: 500; color: var(--text2); }
.topbar-btn {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 12px; border-radius: 7px;
  border: 1px solid var(--border2); background: var(--bg);
  cursor: pointer; font-size: 12px; font-weight: 500; color: var(--text2);
  font-family: var(--sans); transition: all .14s; white-space: nowrap;
}
.topbar-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }

/* ── CPU/GPU split button ─────────────────────────────── */
.hw-split {
  display: flex; border: 1px solid var(--border2);
  border-radius: 7px; overflow: hidden; flex-shrink: 0;
}
.hw-half {
  display: flex; align-items: center; gap: 5px;
  padding: 5px 12px; font-size: 12px; font-weight: 500;
  border: none; cursor: pointer; font-family: var(--sans);
  background: var(--bg); color: var(--text3); transition: all .14s;
  user-select: none;
}
.hw-half:first-child { border-right: 1px solid var(--border2); }
.hw-half.active { background: var(--text); color: #fff; }
.hw-half:hover:not(.active) { background: var(--bg3); color: var(--text); }

/* ── Settings panel ───────────────────────────────────── */
.settings-overlay {
  display: none; position: fixed; inset: 0; z-index: 199;
  background: rgba(0,0,0,.35); backdrop-filter: blur(2px);
}
.settings-overlay.open { display: block; }
.settings-panel {
  position: fixed; top: 0; right: -360px; width: 340px; height: 100vh;
  background: var(--bg); border-left: 1px solid var(--border2);
  z-index: 200; display: flex; flex-direction: column; overflow: hidden;
  box-shadow: -6px 0 28px rgba(0,0,0,.14);
  transition: right .22s cubic-bezier(.4,0,.2,1);
}
.settings-panel.open { right: 0; }
.settings-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.settings-header-title { font-size: 15px; font-weight: 600; color: var(--text); }
.settings-close {
  width: 28px; height: 28px; border-radius: 6px; border: none;
  background: transparent; cursor: pointer; color: var(--text2);
  font-size: 20px; line-height: 1; display: flex; align-items: center;
  justify-content: center; transition: background .12s;
}
.settings-close:hover { background: var(--bg3); color: var(--text); }
.settings-body { flex: 1; overflow-y: auto; padding: 20px 20px; }
.settings-section { margin-bottom: 28px; }
.settings-section-title {
  font-size: 10px; font-weight: 600; color: var(--text3);
  text-transform: uppercase; letter-spacing: .08em; margin-bottom: 14px;
  border-bottom: 1px solid var(--border); padding-bottom: 6px;
}
.settings-row {
  display: flex; align-items: flex-start; justify-content: space-between;
  padding: 10px 0; gap: 16px;
}
.settings-row-info { flex: 1; }
.settings-row-label { font-size: 14px; font-weight: 500; color: var(--text); margin-bottom: 3px; }
.settings-row-sub { font-size: 12px; color: var(--text3); line-height: 1.5; }
.settings-row-ctrl { flex-shrink: 0; }

/* Theme toggle */
.theme-toggle {
  display: flex; border: 1px solid var(--border2);
  border-radius: 8px; overflow: hidden;
}
.theme-btn {
  padding: 6px 14px; font-size: 12px; font-weight: 500;
  border: none; cursor: pointer; font-family: var(--sans);
  background: var(--bg); color: var(--text3); transition: all .15s;
}
.theme-btn:first-child { border-right: 1px solid var(--border2); }
.theme-btn.active { background: var(--accent); color: #fff; }
.theme-btn:hover:not(.active) { background: var(--bg3); color: var(--text); }

/* Context slider */
.ctx-wrap { margin-top: 12px; }
.ctx-slider {
  -webkit-appearance: none; appearance: none;
  width: 100%; height: 4px; border-radius: 2px;
  background: var(--border2); outline: none; cursor: pointer; margin: 8px 0 4px;
}
.ctx-slider::-webkit-slider-thumb {
  -webkit-appearance: none; width: 16px; height: 16px;
  border-radius: 50%; background: var(--accent); cursor: pointer;
  box-shadow: 0 1px 4px rgba(0,0,0,.2);
}
.ctx-slider::-moz-range-thumb {
  width: 16px; height: 16px; border-radius: 50%;
  background: var(--accent); cursor: pointer; border: none;
}
.ctx-value {
  font-family: var(--mono); font-size: 13px; font-weight: 600;
  color: var(--accent); text-align: center; margin: 6px 0 10px;
}
.ctx-presets {
  display: flex; gap: 5px; flex-wrap: wrap; margin-top: 4px;
}
.ctx-preset {
  padding: 3px 10px; font-size: 11px; font-weight: 500;
  border: 1px solid var(--border2); border-radius: 20px;
  background: var(--bg); color: var(--text2); cursor: pointer;
  font-family: var(--mono); transition: all .13s;
}
.ctx-preset:hover { border-color: var(--accent); color: var(--accent); }

/* Info rows */
.info-row {
  display: flex; justify-content: space-between; align-items: baseline;
  padding: 8px 0; border-bottom: 1px solid var(--border);
}
.info-row:last-child { border-bottom: none; }
.info-label { font-size: 13px; color: var(--text2); }
.info-value { font-size: 12px; font-family: var(--mono); color: var(--text3); max-width: 200px; text-align: right; word-break: break-all; }

/* Settings gear icon in topbar */
.gear-icon { width: 15px; height: 15px; opacity: .8; }
</style>
</head>
<body>
<div class="app">

<!-- ── SIDEBAR ── -->
<div class="sidebar">
  <div class="sidebar-logo">
    <div class="logo-mark">PA</div>
    <span class="logo-name">PullAI</span>
    <span class="logo-ver" id="ver">v…</span>
  </div>

  <button class="new-chat-btn" onclick="newChat()">
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><line x1="7" y1="1" x2="7" y2="13"/><line x1="1" y1="7" x2="13" y2="7"/></svg>
    <span id="new-chat-lbl">Nuova chat</span>
  </button>

  <div class="history-list" id="hist-list">
    <!-- populated by JS -->
  </div>

  <div style="margin-top:auto">
    <div style="padding:6px 10px;display:flex;gap:8px;align-items:center;border-top:1px solid var(--border);padding-top:12px">
      <div class="hw-dot off" id="hw-dot"></div>
      <span style="font-size:12px;color:var(--text3)" id="hw-lbl">—</span>
    </div>
  </div>
</div>

<!-- ── MAIN ── -->
<div class="main" id="main">

  <!-- CHAT PANEL -->
  <div class="panel on" id="ws-chat">
    <!-- Topbar with settings button -->
    <div class="main-topbar">
      <div class="topbar-left">
        <span class="topbar-title" id="topbar-title">Chat</span>
      </div>
      <div class="topbar-right">
        <button class="topbar-btn" id="save-chat-btn" onclick="openSaveChat()" title="Salva chat" style="display:none">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M2 2h8l2 2v8a1 1 0 01-1 1H3a1 1 0 01-1-1V2z"/><rect x="4" y="9" width="6" height="4"/><rect x="4" y="2" width="5" height="3"/></svg>
          Salva
        </button>
        <button class="topbar-btn" onclick="openSettings()" title="Impostazioni">
          <svg class="gear-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round">
            <circle cx="8" cy="8" r="2.5"/>
            <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06"/>
          </svg>
          Impostazioni
        </button>
      </div>
    </div>
    <div class="canvas" id="canvas" ondragover="inputDragOver(event)" ondragleave="inputDragLeave(event)" ondrop="handleDrop(event)">
      <div class="welcome" id="welcome">
        <div class="welcome-logo">PA</div>
        <div class="welcome-title" id="welcome-title">Cosa vuoi creare?</div>
        <div class="welcome-sub" id="welcome-sub">Scegli un tipo di output in basso, seleziona il modello e scrivi il prompt.</div>
        <div class="suggestion-grid" id="suggestions">
          <button class="suggestion" onclick="fillPrompt(this)"><b id="sugg-text">💬 Testo</b><span id="sugg-text-p">Spiega come funziona un motore a reazione</span></button>
          <button class="suggestion" onclick="fillPrompt(this)"><b id="sugg-image">🎨 Immagine</b><span id="sugg-image-p">Un tramonto su Marte, stile artistico</span></button>
          <button class="suggestion" onclick="fillPrompt(this)"><b id="sugg-audio">🔊 Audio</b><span id="sugg-audio-p">Leggi questo testo ad alta voce</span></button>
          <button class="suggestion" onclick="fillPrompt(this)"><b id="sugg-3d">🧊 3D</b><span id="sugg-3d-p">Una sedia di design minimalista</span></button>
        </div>
      </div>
      <div class="msgs" id="msgs" style="display:none"></div>
    </div>

    <!-- INPUT ZONE -->
    <div class="input-zone">
      <div class="input-box" id="input-box" ondragover="inputDragOver(event)" ondragleave="inputDragLeave(event)" ondrop="handleDrop(event)">
        <div class="attach-row" id="attach-row" style="display:none"></div>
        <div class="text-row">
          <button class="attach-input-btn" onclick="document.getElementById('fi').click()" title="Allega file">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 8.5L8 15a5 5 0 01-7.07-7.07L8 1a3.33 3.33 0 014.71 4.71L5.64 13a1.67 1.67 0 01-2.36-2.36L10 4"/></svg>
          </button>
          <input type="file" id="fi" style="display:none" multiple onchange="handleFiles(this.files)">
          <textarea class="inp" id="inp" rows="1" placeholder="Scrivi un messaggio..."
          oninput="autoSize(this)" onkeydown="onKey(event)"
          ondragover="event.preventDefault()" ondrop="handleDrop(event)"></textarea>
          <button class="send-btn" id="send-btn" onclick="doSend()">
            <svg id="send-icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M1.5 2L14.5 8L1.5 14L1.5 9.5L10 8L1.5 6.5Z"/></svg>
            <svg id="stop-icon" width="14" height="14" viewBox="0 0 14 14" fill="currentColor" style="display:none"><rect x="2" y="2" width="10" height="10" rx="1.5"/></svg>
          </button>
        </div>
        <!-- Controls row -->
        <div class="controls-row">
          <!-- Type selector -->
          <div class="dd-wrap" id="type-wrap">
            <button class="ctrl-btn" id="type-btn" onclick="toggleDD('type')">
              <span id="type-icon">💬</span>
              <span id="type-lbl">LLM</span>
              <svg viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M1 1l4 4 4-4"/></svg>
            </button>
            <div class="dd-menu" id="type-dd">
              <div class="dd-header">Tipo di output</div>
              <div class="dd-item sel" onclick="setType('chat','💬','LLM',this)"><span>💬</span> LLM — Testo</div>
              <div class="dd-item" onclick="setType('image','🎨','Immagini',this)"><span>🎨</span> Immagini</div>
              <div class="dd-item" onclick="setType('audio','🔊','Audio',this)"><span>🔊</span> Audio</div>
              <div class="dd-item" onclick="setType('video','🎬','Video',this)"><span>🎬</span> Video</div>
              <div class="dd-item" onclick="setType('threed','🧊','3D',this)"><span>🧊</span> 3D</div>
            </div>
          </div>

          <!-- Model selector -->
          <div class="dd-wrap" id="model-wrap">
            <button class="ctrl-btn" id="model-btn" onclick="toggleDD('model')">
              <svg viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><rect x="1" y="1" width="5" height="5" rx="1.5"/><rect x="8" y="1" width="5" height="5" rx="1.5"/><rect x="1" y="8" width="5" height="5" rx="1.5"/><rect x="8" y="8" width="5" height="5" rx="1.5"/></svg>
              <span id="model-lbl">Seleziona modello</span>
              <svg viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M1 1l4 4 4-4"/></svg>
            </button>
            <div class="dd-menu" id="model-dd"></div>
          </div>

          <!-- Think toggle (only LLM) -->
          <button class="ctrl-btn" id="think-btn" onclick="toggleThink()" style="display:none">
            <svg viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7 1a5 5 0 014 8l-1 1H4L3 9A5 5 0 017 1z"/><path d="M5 13h4M7 11v2"/></svg>
            <span id="think-lbl">Thinking</span>
          </button>

          <!-- CPU/GPU split button -->
          <div class="hw-split" title="Seleziona acceleratore">
            <button class="hw-half active" id="hw-gpu-btn" onclick="setHW(false)">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><rect x="2" y="2" width="10" height="10" rx="2"/><rect x="4" y="4" width="6" height="6" rx="1"/><path d="M0 5h2M0 9h2M12 5h2M12 9h2"/></svg>
              GPU
            </button>
            <button class="hw-half" id="hw-cpu-btn" onclick="setHW(true)">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><rect x="1" y="3" width="12" height="8" rx="1.5"/><path d="M4 3V1m6 2V1M4 11v2m6-2v2"/></svg>
              CPU
            </button>
          </div>

          <div style="margin-left:auto;font-size:12px;color:var(--text3)" id="mode-hint">
            <span id="mode-hint-text">Premi ↵ per inviare, Shift+↵ per andare a capo</span>
          </div>
        </div>
      </div>
      <div style="text-align:center;font-size:11px;color:var(--text3);margin-top:8px;max-width:720px;margin-left:auto;margin-right:auto">
        <span id="footer-note">PullAI esegue modelli localmente sul tuo dispositivo. Nessun dato inviato al cloud.</span>
      </div>
    </div>
  </div>

  <!-- MODELS PANEL -->
  <div class="panel" id="ws-models">
    <div class="panel-inner">
      <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:24px">
        <div class="panel-title" style="margin:0" id="panel-models-title">Modelli installati</div>
        <button class="btn primary" onclick="showPanel('registry')"><span id="download-more-lbl">⬇ Scarica altri</span></button>
      </div>
      <div id="mlist"></div>
    </div>
  </div>

  <!-- REGISTRY PANEL -->
  <div class="panel" id="ws-registry">
    <div class="panel-inner">
      <div style="display:flex;align-items:center;gap:10px;margin-bottom:24px">
        <button class="btn" onclick="showPanel('models')"><span id="back-btn-lbl">← Indietro</span></button>
        <div class="panel-title" style="margin:0" id="panel-registry-title">Scarica modelli</div>
      </div>
      <div style="display:flex;gap:8px;margin-bottom:12px">
        <input class="search-box" style="margin:0;flex:1" placeholder="Cerca nel catalogo..." oninput="filterReg(this.value)">
        <button class="btn primary" style="flex-shrink:0;white-space:nowrap" onclick="showCustomInstall()">+ URL HuggingFace</button>
      </div>
      <div id="custom-install" style="display:none;background:var(--bg2);border:1px solid var(--border);border-radius:var(--radius-sm);padding:14px;margin-bottom:14px">
        <div style="font-size:13px;font-weight:500;margin-bottom:8px">Installa da URL HuggingFace</div>
        <div style="font-size:12px;color:var(--text2);margin-bottom:10px">Es: https://huggingface.co/owner/repo oppure owner/repo:file.gguf</div>
        <div style="display:flex;gap:6px;margin-bottom:8px">
          <select id="custom-type" style="padding:7px 10px;border:1px solid var(--border);border-radius:var(--radius-xs);background:var(--bg);color:var(--text);font-size:13px;font-family:var(--sans)">
            <option value="llm">💬 LLM</option>
            <option value="image">🎨 Immagini</option>
            <option value="audio">🔊 Audio</option>
            <option value="video">🎬 Video</option>
            <option value="3d">🧊 3D</option>
          </select>
          <input id="custom-url" style="flex:1;padding:7px 10px;border:1px solid var(--border);border-radius:var(--radius-xs);background:var(--bg);color:var(--text);font-size:13px;font-family:var(--sans);outline:none"
            placeholder="https://huggingface.co/owner/repo o owner/repo:file.gguf"
            onkeydown="if(event.key==='Enter')installCustom()">
          <button class="btn primary" onclick="installCustom()" id="custom-btn">Installa</button>
        </div>
        <!-- Progress bar for custom URL download -->
        <div id="custom-dl-progress" style="display:none;margin-top:10px">
          <div style="display:flex;justify-content:space-between;font-size:11px;color:var(--text3);margin-bottom:5px">
            <span id="custom-dl-lbl">Connessione…</span>
            <span id="custom-dl-pct">0%</span>
          </div>
          <div style="background:var(--border);border-radius:3px;height:5px;overflow:hidden;margin-bottom:8px">
            <div id="custom-dl-bar" style="background:var(--accent);height:100%;width:0%;transition:width .25s ease;border-radius:3px"></div>
          </div>
          <button onclick="cancelCustomDl()"
            style="width:100%;padding:6px;font-size:12px;border:1px solid #dc2626;color:#dc2626;background:transparent;border-radius:6px;cursor:pointer;font-family:var(--sans);font-weight:500">
            ⏹ Annulla download
          </button>
        </div>
      </div>
      <div class="reg-filters" id="reg-filters">
        <button class="rf-btn on" onclick="filterType('all',this)">Tutti</button>
        <button class="rf-btn" onclick="filterType('llm',this)">💬 LLM</button>
        <button class="rf-btn" onclick="filterType('image',this)">🎨 Immagini</button>
        <button class="rf-btn" onclick="filterType('audio',this)">🔊 Audio</button>
        <button class="rf-btn" onclick="filterType('video',this)">🎬 Video</button>
        <button class="rf-btn" onclick="filterType('3d',this)">🧊 3D</button>
      </div>
      <div class="reg-grid" id="reg-grid"></div>
    </div>
  </div>

</div><!-- /main -->
<!-- SETTINGS OVERLAY -->
<div class="settings-overlay" id="settings-overlay" onclick="closeSettings()"></div>

<!-- SETTINGS PANEL -->
<div class="settings-panel" id="settings-panel">
  <div class="settings-header">
    <span class="settings-header-title">⚙️ Impostazioni</span>
    <button class="settings-close" onclick="closeSettings()" title="Chiudi">×</button>
  </div>
  <div class="settings-body">

    <!-- Aspetto -->
    <div class="settings-section">
      <div class="settings-section-title">Aspetto</div>
      <div class="settings-row">
        <div class="settings-row-info">
          <div class="settings-row-label">Tema</div>
          <div class="settings-row-sub">Chiaro o scuro</div>
        </div>
        <div class="settings-row-ctrl">
          <div class="theme-toggle">
            <button class="theme-btn active" id="theme-light" onclick="setTheme('light')">☀️ Chiaro</button>
            <button class="theme-btn" id="theme-dark" onclick="setTheme('dark')">🌙 Scuro</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Language -->
    <div class="settings-section">
      <div class="settings-section-title" data-i18n="language">Lingua</div>
      <div class="settings-row">
        <div class="settings-row-info">
          <div class="settings-row-label" data-i18n="language">Lingua</div>
          <div class="settings-row-sub" data-i18n="languageDesc">Lingua dell'interfaccia</div>
        </div>
        <div class="settings-row-ctrl">
          <div class="theme-toggle">
            <button class="theme-btn active" id="lang-it" onclick="setLang('it')">🇮🇹 IT</button>
            <button class="theme-btn" id="lang-en" onclick="setLang('en')">🇬🇧 EN</button>
          </div>
        </div>
      </div>
    </div>

    <!-- LLM -->
    <div class="settings-section">
      <div class="settings-section-title">Modelli LLM</div>
      <div class="settings-row-info">
        <div class="settings-row-label">Context window</div>
        <div class="settings-row-sub">Token di contesto per l'LLM. Più token = più memoria ma più lento e richiede più VRAM.</div>
      </div>
      <div class="ctx-wrap">
        <input type="range" class="ctx-slider" id="ctx-slider"
          min="512" max="131072" step="512" value="4096"
          oninput="updateCtxLabel(this.value)">
        <div class="ctx-value" id="ctx-value">4 096 token</div>
        <div class="ctx-presets">
          <button class="ctx-preset" onclick="setCtx(2048)">2K</button>
          <button class="ctx-preset" onclick="setCtx(4096)">4K</button>
          <button class="ctx-preset" onclick="setCtx(8192)">8K</button>
          <button class="ctx-preset" onclick="setCtx(16384)">16K</button>
          <button class="ctx-preset" onclick="setCtx(32768)">32K</button>
          <button class="ctx-preset" onclick="setCtx(65536)">64K</button>
          <button class="ctx-preset" onclick="setCtx(131072)">128K</button>
        </div>
      </div>
    </div>

    <!-- Default per tipo -->
    <div class="settings-section">
      <div class="settings-section-title">Modello predefinito per tipo</div>
      <div style="font-size:12px;color:var(--text3);margin-bottom:12px;line-height:1.5">
        Seleziona un modello predefinito per ogni tipo. Verrà pre-selezionato all'apertura. Clicca su ⭐ nella lista modelli installati.
      </div>
      <div id="settings-defaults-list" style="display:flex;flex-direction:column;gap:6px"></div>
    </div>

    <!-- Info -->
    <div class="settings-section">
      <div class="settings-section-title">Informazioni</div>
      <div class="info-row">
        <span class="info-label">Versione</span>
        <span class="info-value" id="settings-ver">—</span>
      </div>
      <div class="info-row">
        <span class="info-label">Hardware</span>
        <span class="info-value" id="settings-hw">—</span>
      </div>
      <div class="info-row">
        <span class="info-label">Modelli installati</span>
        <span class="info-value" id="settings-models">—</span>
      </div>
    </div>

  </div>
</div>

</div><!-- /app -->

<!-- Sidebar nav buttons -->
<div style="position:fixed;bottom:60px;left:0;width:260px;padding:0 8px;display:flex;flex-direction:column;gap:2px;z-index:10">
  <button class="new-chat-btn" onclick="showPanel('models')" style="font-size:13px">
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 10V4a2 2 0 012-2h6a2 2 0 012 2v4"/><path d="M1 12l2-2 2 2 2-2 2 2 2-2 2 2"/></svg>
    Modelli installati
  </button>
</div>

<!-- Save chat dialog -->
<div class="overlay" id="save-dlg">
  <div class="dialog">
    <div class="dlg-title">💾 Salva chat</div>
    <div class="dlg-body" style="padding:8px 0">
      <input id="save-name-input" type="text" placeholder="Nome della chat…"
        style="width:100%;padding:9px 12px;border:1px solid var(--border2);border-radius:8px;font-size:14px;font-family:var(--sans);background:var(--bg);color:var(--text);outline:none;box-sizing:border-box"
        onkeydown="if(event.key==='Enter')doSaveChat()">
    </div>
    <div class="dlg-btns">
      <button class="btn" onclick="document.getElementById('save-dlg').classList.remove('show')">Annulla</button>
      <button class="btn primary" onclick="doSaveChat()">Salva</button>
    </div>
  </div>
</div>

<!-- Delete dialog -->
<div class="overlay" id="dlg">
  <div class="dialog">
    <div class="dlg-title">Rimuovi modello</div>
    <div class="dlg-body" id="dlg-body">Confermi?</div>
    <div class="dlg-btns">
      <button class="btn" onclick="closeDlg()">Annulla</button>
      <button class="btn danger" id="dlg-ok">Rimuovi</button>
    </div>
  </div>
</div>

<!-- Rename dialog -->
<div class="overlay" id="rename-dlg">
  <div class="dialog">
    <div class="dlg-title">✏️ Rinomina modello</div>
    <div class="dlg-body">
      <div style="font-size:12px;color:var(--text3);margin-bottom:8px" id="rename-ref-lbl"></div>
      <input id="rename-input" type="text" placeholder="Nome personalizzato (vuoto = originale)"
        style="width:100%;padding:8px 12px;border:1.5px solid var(--border2);border-radius:8px;font-size:14px;font-family:var(--sans);outline:none;background:var(--bg);color:var(--text)"
        onkeydown="if(event.key==='Enter')doRename()">
    </div>
    <div class="dlg-btns">
      <button class="btn" onclick="document.getElementById('rename-dlg').classList.remove('show');pendingRename=null">Annulla</button>
      <button class="btn primary" onclick="doRename()">Salva</button>
    </div>
  </div>
</div>

<!-- Info dialog -->
<div class="overlay" id="info-dlg">
  <div class="dialog" style="min-width:340px;max-width:500px">
    <div class="dlg-title">ℹ️ Info modello</div>
    <div id="info-body" style="max-height:360px;overflow-y:auto;margin-bottom:20px"></div>
    <div class="dlg-btns">
      <button class="btn primary" onclick="document.getElementById('info-dlg').classList.remove('show')">Chiudi</button>
    </div>
  </div>
</div>

<div class="toast" id="toast"></div>

<script>
// ── Internazionalizzazione (i18n) ─────────────────────────────────────────

const LANGS = {
  it: {
    // UI labels
    newChat: '+ Nuova chat',
    installedModels: 'Modelli installati',
    settings: '⚙️ Impostazioni',
    save: '💾 Salva',
    send: 'Invia',
    stop: 'Ferma',
    thinking: 'Ragionamento',
    downloadMore: '⬇ Scarica altri',
    download: '↓ Scarica',
    cancel: 'Annulla',
    cancelDownload: '⏹ Annulla download',
    installing: 'Installazione…',
    installed: '✅ Installato',
    info: 'ℹ️ Info',
    rename: '✏️ Rinomina',
    setDefault: '☆ Default',
    isDefault: '⭐ Predefinito',
    remove: '🗑 Rimuovi',
    removeConfirm: 'Rimuovere',
    // Settings panel
    settingsTitle: '⚙️ Impostazioni',
    appearance: 'Aspetto',
    theme: 'Tema',
    themeDesc: 'Chiaro o scuro',
    light: '☀️ Chiaro',
    dark: '🌙 Scuro',
    language: 'Lingua',
    languageDesc: "Lingua dell'interfaccia",
    llmSettings: 'Modelli LLM',
    contextWindow: 'Context window',
    contextDesc: "Token di contesto per l'LLM. Più token = più memoria ma più lento.",
    defaultModels: 'Modelli predefiniti',
    defaultModelsDesc: "Seleziona il modello predefinito per ogni tipo.",
    noDefault: '— Nessun predefinito —',
    version: 'Versione',
    hardware: 'Hardware',
    installedCount: 'Modelli installati',
    info2: 'Informazioni',
    // Registry
    searchCatalog: 'Cerca nel catalogo...',
    urlInstall: '+ URL HuggingFace',
    installFromUrl: 'Installa da URL HuggingFace',
    urlExample: 'Es: https://huggingface.co/owner/repo oppure owner/repo:file.gguf',
    install: 'Installa',
    all: 'Tutti',
    // Chat
    placeholder: 'Scrivi un messaggio...',
    sendHint: '<span id="mode-hint-text">Premi ↵ per inviare, Shift+↵ per andare a capo</span>',
    noModels: 'Nessun modello installato.',
    downloadNow: '↓ Scarica subito',
    saveChatTitle: '💾 Salva chat',
    chatName: 'Nome della chat…',
    renameTitle: 'Rinomina modello',
    newName: 'Nuovo nome…',
    deleteTitle: 'Rimuovi modello',
    // Toasts
    downloaded: '✅ Scaricato: ',
    removed: '✅ Rimosso: ',
    renamed: '✅ Rinominato',
    chatSaved: '💾 Chat salvata: ',
    chatLoaded: 'Chat ripresa: ',
    chatDeleted: 'Chat eliminata',
    invalidRef: '❌ Ref modello non valido: ',
    offline: '❌ Server offline',
    connecting: 'Connessione a HuggingFace...',
    // Welcome screen
    welcomeTitle: 'Cosa vuoi creare?',
    welcomeSub: 'Scegli un tipo di output in basso, seleziona il modello e scrivi il prompt.',
    suggText: '💬 Testo',
    suggTextPrompt: 'Spiega come funziona un motore a reazione',
    suggImage: '🎨 Immagine',
    suggImagePrompt: 'Un tramonto su Marte, stile artistico',
    suggAudio: '🔊 Audio',
    suggAudioPrompt: 'Leggi questo testo ad alta voce',
    sugg3D: '🧊 3D',
    sugg3DPrompt: 'Una sedia di design minimalista',
    // Sidebar
    newChat: '+ Nuova chat',
    noChats: 'Nessuna chat salvata',
    installedBtn: 'Modelli installati',
    // Panels
    installedModelsTitle: 'Modelli installati',
    downloadMore: '⬇ Scarica altri',
    downloadModels: 'Scarica modelli',
    back: '← Indietro',
    searchPlaceholder: 'Cerca nel catalogo...',
    urlInstallTitle: 'Installa da URL HuggingFace',
    // Footer
    footerNote: '<span id="footer-note">PullAI esegue modelli localmente sul tuo dispositivo. Nessun dato inviato al cloud.</span>',
    // Controls
    sendHint: '<span id="mode-hint-text">Premi ↵ per inviare, Shift+↵ per andare a capo</span>',
    // Topbar
    chatTitle: 'Chat',
  },
  en: {
    newChat: '+ New chat',
    installedModels: 'Installed models',
    settings: '⚙️ Settings',
    save: '💾 Save',
    send: 'Send',
    stop: 'Stop',
    thinking: 'Thinking',
    downloadMore: '⬇ Download more',
    download: '↓ Download',
    cancel: 'Cancel',
    cancelDownload: '⏹ Cancel download',
    installing: 'Installing…',
    installed: '✅ Installed',
    info: 'ℹ️ Info',
    rename: '✏️ Rename',
    setDefault: '☆ Default',
    isDefault: '⭐ Default',
    remove: '🗑 Remove',
    removeConfirm: 'Remove',
    settingsTitle: '⚙️ Settings',
    appearance: 'Appearance',
    theme: 'Theme',
    themeDesc: 'Light or dark',
    light: '☀️ Light',
    dark: '🌙 Dark',
    language: 'Language',
    languageDesc: 'Interface language',
    llmSettings: 'LLM Models',
    contextWindow: 'Context window',
    contextDesc: 'Context tokens for LLM. More = more memory but slower.',
    defaultModels: 'Default models',
    defaultModelsDesc: 'Set default model for each type.',
    noDefault: '— No default —',
    version: 'Version',
    hardware: 'Hardware',
    installedCount: 'Installed models',
    info2: 'Information',
    searchCatalog: 'Search catalog...',
    urlInstall: '+ HuggingFace URL',
    installFromUrl: 'Install from HuggingFace URL',
    urlExample: 'E.g.: https://huggingface.co/owner/repo or owner/repo:file.gguf',
    install: 'Install',
    all: 'All',
    placeholder: 'Write a message...',
    sendHint: 'Press ↵ to send, Shift+↵ for newline',
    noModels: 'No models installed.',
    downloadNow: '↓ Download now',
    saveChatTitle: '💾 Save chat',
    chatName: 'Chat name…',
    renameTitle: 'Rename model',
    newName: 'New name…',
    deleteTitle: 'Remove model',
    downloaded: '✅ Downloaded: ',
    removed: '✅ Removed: ',
    renamed: '✅ Renamed',
    chatSaved: '💾 Chat saved: ',
    chatLoaded: 'Chat resumed: ',
    chatDeleted: 'Chat deleted',
    invalidRef: '❌ Invalid model ref: ',
    offline: '❌ Server offline',
    connecting: 'Connecting to HuggingFace...',
    // Welcome screen
    welcomeTitle: 'What do you want to create?',
    welcomeSub: 'Choose an output type below, select a model and write your prompt.',
    suggText: '💬 Text',
    suggTextPrompt: 'Explain how a jet engine works',
    suggImage: '🎨 Image',
    suggImagePrompt: 'A sunset on Mars, artistic style',
    suggAudio: '🔊 Audio',
    suggAudioPrompt: 'Read this text aloud',
    sugg3D: '🧊 3D',
    sugg3DPrompt: 'A minimalist design chair',
    // Sidebar
    newChat: '+ New chat',
    noChats: 'No saved chats',
    installedBtn: 'Installed models',
    // Panels
    installedModelsTitle: 'Installed models',
    downloadMore: '⬇ Download more',
    downloadModels: 'Download models',
    back: '← Back',
    searchPlaceholder: 'Search catalog...',
    urlInstallTitle: 'Install from HuggingFace URL',
    // Footer
    footerNote: 'PullAI runs models locally on your device. No data sent to the cloud.',
    // Controls
    sendHint: 'Press ↵ to send, Shift+↵ for newline',
    // Topbar
    chatTitle: 'Chat',
  }
};

let currentLang = localStorage.getItem('pullai_lang') || 'it';
function t(key) { return (LANGS[currentLang] || LANGS.it)[key] || key; }

function setLang(lang) {
  if (!LANGS[lang]) return;
  currentLang = lang;
  localStorage.setItem('pullai_lang', lang);
  applyLang();
}

function applyLang() {
  const s = (id, key) => { const el = document.getElementById(id); if (el) el.textContent = t(key); };
  const p = (id, key) => { const el = document.getElementById(id); if (el) el.placeholder = t(key); };

  // Welcome screen
  s('welcome-title', 'welcomeTitle');
  s('welcome-sub', 'welcomeSub');
  s('sugg-text', 'suggText');
  s('sugg-text-p', 'suggTextPrompt');
  s('sugg-image', 'suggImage');
  s('sugg-image-p', 'suggImagePrompt');
  s('sugg-audio', 'suggAudio');
  s('sugg-audio-p', 'suggAudioPrompt');
  s('sugg-3d', 'sugg3D');
  s('sugg-3d-p', 'sugg3DPrompt');

  // Sidebar
  s('new-chat-lbl', 'newChat');
  s('installed-btn-lbl', 'installedBtn');

  // Panels
  s('panel-models-title', 'installedModelsTitle');
  s('panel-registry-title', 'downloadModels');
  s('download-more-lbl', 'downloadMore');
  s('back-btn-lbl', 'back');
  s('footer-note', 'footerNote');

  // Input area
  p('msg-input', 'placeholder');
  const hint = document.getElementById('mode-hint-text') || document.getElementById('mode-hint');
  if (hint) hint.textContent = t('sendHint');
  const topbar = document.getElementById('topbar-title');
  if (topbar && (topbar.textContent === 'Chat' || topbar.textContent === 'Chat')) {
    topbar.textContent = t('chatTitle');
  }

  // Settings
  const st = document.querySelector('.settings-header-title');
  if (st) st.textContent = t('settingsTitle');
  p('save-name-input', 'chatName');

  // Search
  const sb = document.querySelector('.search-box');
  if (sb) sb.placeholder = t('searchPlaceholder');

  // Registry custom install title
  const rit = document.querySelector('#custom-install > div:first-child');
  if (rit) rit.textContent = t('urlInstallTitle');

  // data-i18n fallback
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    const val = t(key);
    if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
      el.placeholder = val;
    } else {
      el.textContent = val;
    }
  });

  // Lang toggle buttons
  document.getElementById('lang-it')?.classList.toggle('active', currentLang === 'it');
  document.getElementById('lang-en')?.classList.toggle('active', currentLang === 'en');

  // Refresh model list if visible (to re-render translated buttons)
  if (document.getElementById('ws-models')?.classList.contains('on')) loadModels();
}

// ── Settings ─────────────────────────────────────────
let currentTheme = localStorage.getItem('pullai_theme') || 'light';
let contextSize = parseInt(localStorage.getItem('pullai_ctx') || '4096');

function openSettings() {
  document.getElementById('settings-panel').classList.add('open');
  document.getElementById('settings-overlay').classList.add('open');
  const sl = document.getElementById('ctx-slider');
  if (sl) { sl.value = contextSize; updateCtxLabel(contextSize); }
  const verEl = document.getElementById('ver');
  document.getElementById('settings-ver').textContent = verEl ? verEl.textContent : '—';
  const hwEl = document.getElementById('hw-lbl');
  document.getElementById('settings-hw').textContent = hwEl ? hwEl.textContent : '—';
  document.getElementById('settings-models').textContent = installedModels.length + ' modelli';
  document.getElementById('theme-light').classList.toggle('active', currentTheme==='light');
  document.getElementById('theme-dark').classList.toggle('active', currentTheme==='dark');

  // Render per-type defaults
  const types = [...new Set(installedModels.map(m => m.type))];
  const defList = document.getElementById('settings-defaults-list');
  if (defList) {
    if (!types.length) {
      defList.innerHTML = '<div style="font-size:12px;color:var(--text3)">Nessun modello installato</div>';
    } else {
      const typeIcons = {llm:'💬',image:'🎨',audio:'🔊',video:'🎬','3d':'🧊'};
      defList.innerHTML = types.map(t => {
        const models = installedModels.filter(m => m.type === t);
        const curDef = defaultModels[t] || '';
        return `<div style="padding:8px 0;border-bottom:1px solid var(--border)">
          <div style="font-size:12px;font-weight:600;color:var(--text2);margin-bottom:6px">
            ${typeIcons[t]||'●'} ${t.toUpperCase()}
          </div>
          <select onchange="setDefaultModelDirect('${t}', this.value)"
            style="width:100%;padding:6px 10px;border:1px solid var(--border2);border-radius:6px;background:var(--bg);color:var(--text);font-size:12px;font-family:var(--sans)">
            <option value="">— Nessun predefinito —</option>
            ${models.map(m => {
              const ref = m.type+'/'+m.name+':'+m.tag;
              const label = m.display_name || (m.name+':'+m.tag);
              return `<option value="${ref}" ${curDef===ref?'selected':''}>${esc(label)}</option>`;
            }).join('')}
          </select>
        </div>`;
      }).join('');
    }
  }
}

function setDefaultModelDirect(type, ref) {
  if (!ref) {
    delete defaultModels[type];
    toast('Predefinito rimosso per ' + type.toUpperCase());
  } else {
    defaultModels[type] = ref;
    toast('⭐ Predefinito ' + type.toUpperCase() + ': ' + ref.split('/').slice(1).join('/'));
  }
  localStorage.setItem('pullai_default_models', JSON.stringify(defaultModels));
  loadModels();
}

function closeSettings() {
  document.getElementById('settings-panel').classList.remove('open');
  document.getElementById('settings-overlay').classList.remove('open');
}

function setTheme(theme) {
  currentTheme = theme;
  localStorage.setItem('pullai_theme', theme);
  applyTheme(theme);
  document.getElementById('theme-light').classList.toggle('active', theme==='light');
  document.getElementById('theme-dark').classList.toggle('active', theme==='dark');
}

function applyTheme(theme) {
  if (theme === 'dark') {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }
}

function updateCtxLabel(val) {
  const n = parseInt(val);
  contextSize = n;
  localStorage.setItem('pullai_ctx', n);
  const el = document.getElementById('ctx-value');
  if (el) {
    const k = n >= 1024 ? (n/1024).toFixed(0)+'K' : n;
    el.textContent = n + ' token (' + k + ')';
  }
}

function setCtx(val) {
  contextSize = val;
  localStorage.setItem('pullai_ctx', val);
  const sl = document.getElementById('ctx-slider');
  if (sl) { sl.value = val; }
  updateCtxLabel(val);
  toast('Context: ' + val + ' token');
}

// ── State ─────────────────────────────────────────
let domain = 'chat';
let thinkOn = false;
let useCPU = false;
let attachments = [];
let selectedModel = '';
let chatHistory = [];
let sessions = [];
let currentSession = null;
let currentAbortController = null;
let savedChats = JSON.parse(localStorage.getItem('pullai_chats') || '[]');
let currentChatId = null;
// Per-type default models: { llm: 'llm/name:tag', image: '...', ... }
let defaultModels = JSON.parse(localStorage.getItem('pullai_default_models') || '{}');

function getDefaultModel(type) { return defaultModels[type] || ''; }
function setDefaultModel(ref) {
  const type = ref.split('/')[0];
  // Toggle: if already default, clear it
  if (defaultModels[type] === ref) {
    delete defaultModels[type];
    toast('✅ Default rimosso per ' + type.toUpperCase());
  } else {
    defaultModels[type] = ref;
    toast('⭐ Predefinito per ' + type.toUpperCase() + ': ' + ref.split('/').slice(1).join('/'));
  }
  localStorage.setItem('pullai_default_models', JSON.stringify(defaultModels));
  loadModels();
}


// ── Domain config — no hardcoded models, loaded from API ─────────────────
const DC = {
  chat:   {icon:'💬', label:'LLM',      ph:'Scrivi un messaggio...',                              think:true,  apiType:'llm'},
  image:  {icon:'🎨', label:'Immagini', ph:'Descrivi l\'immagine che vuoi generare...',           think:false, apiType:'image'},
  audio:  {icon:'🔊', label:'Audio',    ph:'Testo per TTS, oppure allega audio per STT...',        think:false, apiType:'audio'},
  video:  {icon:'🎬', label:'Video',    ph:'Descrivi il video da generare...',                     think:false, apiType:'video'},
  threed: {icon:'🧊', label:'3D',       ph:'Descrivi il modello 3D o allega un\'immagine...',     think:false, apiType:'3d'},
};

// Installed models cache — filled by API
let installedModels = [];

// ── Init ──────────────────────────────────────────
function init() {
  applyTheme(currentTheme);
  applyLang();
  loadSavedChats();
  setType('chat','💬','LLM', document.querySelector('#type-dd .dd-item'));
  loadStatus();
  setInterval(loadStatus, 15000);
  renderReg(REG);
}

// ── Status ────────────────────────────────────────
async function loadStatus() {
  try {
    const r = await fetch('/api/status', {signal: AbortSignal.timeout(3000)});
    const d = await r.json();
    document.getElementById('hw-dot').classList.remove('off');
    document.getElementById('hw-lbl').textContent = d.hardware || 'CPU';
    document.getElementById('ver').textContent = d.version ? ('v' + d.version) : 'v?';
    await refreshModels();
  } catch {
    document.getElementById('hw-dot').classList.add('off');
    document.getElementById('hw-lbl').textContent = 'Offline';
  }
}

async function refreshModels() {
  try {
    const r = await fetch('/api/models');
    const d = await r.json();
    installedModels = (d.models || []);
    updateModelDDForDomain();
  } catch {
    installedModels = [];
  }
}

function updateModelDDForDomain() {
  const cfg = DC[domain];
  if (!cfg) return;
  const apiType = cfg.apiType;
  const mine = installedModels.filter(m => m.type === apiType);
  const models = mine.map(m => ({
    v: `${m.type}/${m.name}:${m.tag}`,
    l: `${m.name}:${m.tag}`,
    tag: m.size_human || '',
  }));
  buildModelDD(models);
  // Auto-select first if current selection no longer exists
  if (!models.find(m => m.v === selectedModel)) {
    selectedModel = models[0]?.v || '';
    document.getElementById('model-lbl').textContent = models[0]?.l || 'Nessun modello';
  }
}

// ── Type & Model ──────────────────────────────────
function setType(d, icon, label, el) {
  domain = d;
  document.getElementById('type-icon').textContent = icon;
  document.getElementById('type-lbl').textContent = label;
  document.querySelectorAll('#type-dd .dd-item').forEach(x => x.classList.remove('sel'));
  if (el) el.classList.add('sel');
  closeAllDD();

  const cfg = DC[d];
  if (cfg) {
    document.getElementById('inp').placeholder = cfg.ph;
    // Show only installed models for this domain
    updateModelDDForDomain();
  }
  document.getElementById('think-btn').style.display = cfg?.think ? 'flex' : 'none';
  if (!cfg?.think) thinkOn = false;

  updateWelcomeSuggestions();
}

function buildModelDD(models) {
  const dd = document.getElementById('model-dd');
  const cfg = DC[domain] || {};
  if (!models.length) {
    dd.innerHTML = `<div class="dd-header">Nessun modello installato</div>
      <div class="dd-item" style="color:var(--text3);font-size:12px;cursor:default">
        Usa <strong>Scarica modelli</strong> per installarne uno
      </div>
      <div class="dd-sep"></div>
      <div class="dd-item" onclick="showPanel('registry');closeAllDD()">
        → Vai al catalogo modelli
      </div>`;
    document.getElementById('model-lbl').textContent = 'Nessun modello';
    selectedModel = '';
    return;
  }
  dd.innerHTML = '<div class="dd-header">Modelli installati</div>' + models.map(m =>
    `<div class="dd-item${m.v===selectedModel?' sel':''}" onclick="selectModel('${esc(m.v)}','${esc(m.l)}')">
      <span style="font-size:13px">${esc(m.l)}</span>
      <span class="dd-item-tag">${esc(m.tag||'')}</span>
    </div>`
  ).join('');
}

function selectModel(v, l) {
  selectedModel = v;
  document.getElementById('model-lbl').textContent = l;
  document.querySelectorAll('#model-dd .dd-item').forEach(x => {
    x.classList.toggle('sel', x.textContent.trim().startsWith(l));
  });
  closeAllDD();
}

function toggleThink() {
  thinkOn = !thinkOn;
  const btn = document.getElementById('think-btn');
  btn.classList.toggle('active', thinkOn);
  document.getElementById('think-lbl').textContent = thinkOn ? 'Thinking ON' : 'Thinking';
}

// ── Dropdowns ─────────────────────────────────────
function toggleDD(name) {
  const dd = document.getElementById(name+'-dd');
  const btn = document.getElementById(name+'-btn');
  const isOpen = dd.classList.contains('open');
  closeAllDD();
  if (!isOpen) {
    const r = btn.getBoundingClientRect();
    // Position above the button
    dd.style.left = r.left + 'px';
    dd.style.bottom = (window.innerHeight - r.top + 6) + 'px';
    dd.style.top = 'auto';
    // If not enough space above, open below
    if (r.top < 300) {
      dd.style.top = (r.bottom + 6) + 'px';
      dd.style.bottom = 'auto';
    }
    dd.classList.add('open');
  }
}
function closeAllDD() {
  document.querySelectorAll('.dd-menu').forEach(d => d.classList.remove('open'));
}
document.addEventListener('click', e => {
  if (!e.target.closest('.dd-wrap')) closeAllDD();
});

// ── Files ─────────────────────────────────────────
function handleFiles(files) {
  Array.from(files).forEach(f => {
    const id = Date.now() + Math.random();
    const isImg = f.type.startsWith('image/');
    const reader = new FileReader();
    reader.onload = e => {
      attachments.push({id, file: f, dataUrl: e.target.result, isImg});
      renderAttachRow();
    };
    reader.readAsDataURL(f);
  });
  document.getElementById('fi').value = '';
}

function renderAttachRow() {
  const row = document.getElementById('attach-row');
  if (!attachments.length) { row.style.display = 'none'; return; }
  row.style.display = 'flex';
  row.innerHTML = attachments.map(a =>
    `<div class="attach-chip">
      ${a.isImg ? `<img src="${a.dataUrl}">` : `<span style="font-size:14px">📄</span>`}
      <span class="attach-chip-name">${esc(a.file.name)}</span>
      <span class="attach-chip-rm" onclick="rmAttach(${a.id})">×</span>
    </div>`
  ).join('');
}
function rmAttach(id) { attachments = attachments.filter(a => a.id !== id); renderAttachRow(); }

// ── Input ─────────────────────────────────────────
function autoSize(el) {
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 200) + 'px';
}
function onKey(e) {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); doSend(); }
}

function fillPrompt(btn) {
  const b = btn.querySelector('b');
  const fullText = btn.textContent;
  const prompt = fullText.replace(b.textContent, '').trim();
  const iconLabel = b.textContent.trim();
  const typeMap = {'💬 Testo':'chat','🎨 Immagine':'image','🔊 Audio':'audio','🧊 3D':'threed'};
  for (const [k,v] of Object.entries(typeMap)) {
    if (iconLabel.startsWith(k.split(' ')[0])) {
      const cfg = DC[v];
      const domainEl = document.querySelector(`#type-dd [onclick*="'${v}'"]`);
      setType(v, cfg.icon, cfg.label, domainEl);
      break;
    }
  }
  document.getElementById('inp').value = prompt;
  autoSize(document.getElementById('inp'));
  document.getElementById('inp').focus();
}

function newChat() {
  chatHistory = [];
  attachments = [];
  renderAttachRow();
  document.getElementById('inp').value = '';
  document.getElementById('msgs').innerHTML = '';
  document.getElementById('msgs').style.display = 'none';
  document.getElementById('welcome').style.display = 'flex';
  showPanel('chat');
}

function updateWelcomeSuggestions() {}

// ── Send ──────────────────────────────────────────
async function doSend() {
  const field = document.getElementById('inp');
  const text = field.value.trim();
  if (!text && !attachments.length) return;
  if (!selectedModel) { toast('Seleziona un modello','err'); return; }

  field.value = ''; autoSize(field);
  document.getElementById('welcome').style.display = 'none';
  document.getElementById('msgs').style.display = 'block';
  const _scb = document.getElementById('save-chat-btn');
  if (_scb) _scb.style.display = 'flex';

  setGenerating(true);

  currentAbortController = new AbortController();
  const signal = currentAbortController.signal;

  const localAttach = [...attachments];
  attachments = []; renderAttachRow();

  const uploaded = [];
  for (const a of localAttach) {
    try {
      const fd = new FormData();
      fd.append('file', a.file);
      const up = await fetch('/api/upload', {method:'POST', body:fd, signal});
      const ud = await up.json();
      if (ud.path) uploaded.push({...a, serverPath: ud.path});
    } catch {}
  }

  // Auto-detect file type and warn if unsupported for this domain
  if (uploaded.length > 0) {
    const f = uploaded[0];
    const ext = f.file.name.split('.').pop().toLowerCase();
    const isAudio = /^(mp3|wav|flac|ogg|m4a|aac|opus|wv|aiff)$/.test(ext);
    const isImage = /^(png|jpg|jpeg|webp|gif|bmp)$/.test(ext);
    const isVideo = /^(mp4|avi|mov|mkv|webm)$/.test(ext);
    const supportedAudio = ['audio', 'chat'];
    const supportedImage = ['image', 'threed', 'chat'];
    const supportedVideo = ['video', 'chat'];
    let unsupported = false;
    if (isAudio && !supportedAudio.includes(domain)) {
      toast('⚠️ File audio: passa alla modalità Audio (STT)', 'err'); unsupported = true;
    } else if (isImage && !supportedImage.includes(domain)) {
      toast('⚠️ File immagine: usa modalità Immagini o 3D', 'err'); unsupported = true;
    } else if (isVideo && !supportedVideo.includes(domain)) {
      toast('⚠️ File video: non supportato in questa modalità', 'err'); unsupported = true;
    } else if (!isAudio && !isImage && !isVideo && domain !== 'chat') {
      toast('⚠️ Formato file non supportato per questa modalità', 'err'); unsupported = true;
    }
    // Auto-switch domain to match file type
    if (isAudio && domain === 'chat') {
      const audioModels = installedModels.filter(m => m.type === 'audio');
      if (audioModels.length) {
        setType('audio', '🔊', 'Audio', document.querySelector('#type-dd .dd-item[onclick*=audio]'));
        toast('🔊 Passato in modalità Audio per il file allegato');
      }
    }
  }

  if (domain === 'chat') await doChat(text, uploaded, signal);
  else await doGenerate(text, uploaded, signal);

  setGenerating(false);
  currentAbortController = null;
}

// ── Chat ──────────────────────────────────────────
async function doChat(text, files, signal) {
  const msgs = document.getElementById('msgs');
  const canvas = document.getElementById('canvas');

  addUserMsg(text, files);
  chatHistory.push({role:'user', content:text});

  // AI msg
  const grp = document.createElement('div');
  grp.className = 'msg-group msg-ai';
  const inner = document.createElement('div');
  inner.className = 'msg-inner';
  const avatar = document.createElement('div');
  avatar.className = 'ai-avatar'; avatar.textContent = 'PA';
  const content = document.createElement('div');
  content.className = 'content';

  if (thinkOn) {
    const tb = document.createElement('div');
    tb.className = 'thinking-block';
    tb.innerHTML = '<div class="thinking-label">💡 Deep Thinking</div><div class="dot-pulse"><span></span><span></span><span></span></div>';
    content.appendChild(tb);
  }

  const pulse = document.createElement('div');
  pulse.className = 'dot-pulse';
  pulse.innerHTML = '<span></span><span></span><span></span>';
  content.appendChild(pulse);

  inner.appendChild(avatar); inner.appendChild(content);
  grp.appendChild(inner); msgs.appendChild(grp);
  canvas.scrollTop = 9999;

  try {
    // Send full history so the LLM has conversation context
    const payload = {
      model: selectedModel, prompt: text, stream: true,
      force_cpu: useCPU, context_size: contextSize,
      think: thinkOn, // enable chain-of-thought for reasoning models
      messages: chatHistory.slice(0, -1)
    };
    if (files.length) payload.input_file = files[0].serverPath;
    const r = await fetch('/api/generate', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(payload), signal});

    pulse.remove();
    const textNode = document.createElement('span');
    content.appendChild(textNode);

    if (!r.ok) {
      const d = await r.json().catch(()=>({}));
      textNode.textContent = '⚠️ ' + (d.error || 'Errore');
      return;
    }

    const reader = r.body.getReader(), dec = new TextDecoder();
    let buf = '', full = '';
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      buf += dec.decode(value, {stream:true});
      const lines = buf.split('\n'); buf = lines.pop();
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        const data = line.slice(6);
        if (data === '[DONE]') break;
        if (data.startsWith('[ERROR]')) { textNode.textContent += '\n⚠️' + data.slice(7); break; }
        // Handle <think>...</think> for reasoning models
        if (!window._pullaiThinkBuf) { window._pullaiThinkBuf = ''; window._pullaiInThink = false; }
        window._pullaiThinkBuf += data;
        let toDisplay = '';
        while (window._pullaiThinkBuf.length > 0) {
          if (window._pullaiInThink) {
            const eIdx = window._pullaiThinkBuf.indexOf('</think>');
            if (eIdx >= 0) {
              const thinkText = window._pullaiThinkBuf.slice(0, eIdx);
              window._pullaiThinkBuf = window._pullaiThinkBuf.slice(eIdx + 8);
              window._pullaiInThink = false;
              // Update thinking block
              let tb = grp.querySelector('.thinking-text');
              if (!tb) {
                let tblock = grp.querySelector('.thinking-block');
                if (!tblock) {
                  tblock = document.createElement('div');
                  tblock.className = 'thinking-block';
                  tblock.innerHTML = '<div class="thinking-label">💡 Ragionamento</div><div class="thinking-text"></div>';
                  content.insertBefore(tblock, textNode.parentNode || content.firstChild);
                }
                tb = tblock.querySelector('.thinking-text');
              }
              if (tb) tb.textContent += thinkText;
            } else { break; }
          } else {
            const sIdx = window._pullaiThinkBuf.indexOf('<think>');
            if (sIdx >= 0) {
              toDisplay += window._pullaiThinkBuf.slice(0, sIdx);
              window._pullaiThinkBuf = window._pullaiThinkBuf.slice(sIdx + 7);
              window._pullaiInThink = true;
            } else {
              toDisplay += window._pullaiThinkBuf;
              window._pullaiThinkBuf = '';
            }
          }
        }
        if (toDisplay) { textNode.textContent += toDisplay; full += toDisplay; }
        else { full += data; }
        canvas.scrollTop = 9999;
      }
    }
    chatHistory.push({role:'assistant', content:full});
  } catch {
    pulse.remove();
    content.innerHTML += '<span style="color:#e04040">⚠️ Server non raggiungibile</span>';
  }
}

// ── Generate ──────────────────────────────────────
async function doGenerate(text, files, signal) {
  const msgs = document.getElementById('msgs');
  const canvas = document.getElementById('canvas');
  const outExt = {image:'output.png', audio:'output.wav', video:'output.mp4', threed:'output.obj'}[domain] || 'output.bin';

  addUserMsg(text, files);

  const grp = document.createElement('div');
  grp.className = 'msg-group msg-ai';
  grp.innerHTML = `<div class="msg-inner"><div class="ai-avatar">PA</div><div class="content">
    <div class="progress-wrap" id="prog-wrap">
      <div class="progress-label">
        <span class="progress-msg" id="prog-msg">Avvio generazione...</span>
        <span id="prog-pct">0%</span>
      </div>
      <div class="progress-bar-outer"><div class="progress-bar-inner" id="prog-bar" style="width:0%"></div></div>
    </div>
  </div></div>`;
  msgs.appendChild(grp); canvas.scrollTop = 9999;

  const contentEl = grp.querySelector('.content');

  function updateProgress(pct, msg) {
    const bar = document.getElementById('prog-bar');
    const pctEl = document.getElementById('prog-pct');
    const msgEl = document.getElementById('prog-msg');
    if (bar) bar.style.width = pct + '%';
    if (pctEl) pctEl.textContent = pct + '%';
    if (msgEl && msg) msgEl.textContent = msg;
    canvas.scrollTop = 9999;
  }

  try {
    const inputPath = files.length ? files[0].serverPath : '';
    const r = await fetch('/api/generate', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({model:selectedModel, prompt:text, input_file:inputPath, force_cpu:useCPU}),
      signal
    });

    if (!r.ok) {
      const d = await r.json().catch(()=>({}));
      contentEl.innerHTML = `<span style="color:var(--red)">❌ ${esc(d.error||'Errore')}</span>`;
      return;
    }

    // Parse SSE stream — handles both named events (event: X\ndata: Y)
    // and old-style (data: {"type":"X",...})
    const reader = r.body.getReader();
    const dec = new TextDecoder();
    let buf = '', resultData = null, errorData = null;

    outer: while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      buf += dec.decode(value, {stream: true});
      const chunks = buf.split('\n\n');
      buf = chunks.pop();
      for (const chunk of chunks) {
        let etype = '', dline = '';
        for (const line of chunk.split('\n')) {
          if (line.startsWith('event: ')) etype = line.slice(7).trim();
          else if (line.startsWith('data: ')) dline = line.slice(6).trim();
        }
        if (!dline) continue;
        try {
          const ev = JSON.parse(dline);
          // Named event format (v0.3.19 server)
          if (etype === 'progress' || ev.type === 'progress') {
            const pct = ev.pct ?? ev.Percent ?? 0;
            const msg = ev.msg ?? ev.Message ?? '';
            updateProgress(pct, msg);
          } else if (etype === 'result') {
            resultData = ev; updateProgress(100, 'Completato!'); break outer;
          } else if (etype === 'error') {
            errorData = ev; break outer;
          } else if (ev.type === 'done') {
            // old format compat
            resultData = ev; updateProgress(100, 'Completato!'); break outer;
          } else if (ev.type === 'error') {
            errorData = {error: ev.msg || 'Errore'}; break outer;
          }
        } catch {}
      }
    }

    if (errorData) {
      contentEl.innerHTML = `<span style="color:var(--red)">❌ ${esc(errorData.error||'Errore')}</span>`;
      return;
    }
    if (!resultData) {
      contentEl.innerHTML = `<span style="color:var(--red)">❌ Nessun risultato ricevuto</span>`;
      return;
    }

    const d = resultData;
    contentEl.innerHTML = '';

    if (!r.ok) {
      contentEl.innerHTML = `<span style="color:#e04040">❌ ${esc(d.error||'Errore')}</span>`;
      return;
    }

    if (d.data) {
      const src = `data:${d.mime};base64,${d.data}`;
      const isImg = d.mime?.startsWith('image/');
      const isAudio = d.mime?.startsWith('audio/');
      const is3D = d.mime === 'model/obj' || domain === 'threed';
      const isVideo = d.mime?.startsWith('video/');
      const savedTo = d.saved_to ? `<div style="font-size:11px;color:var(--text3);margin-top:4px">💾 Salvato: ${esc(d.saved_to)}</div>` : '';

      const mw = document.createElement('div');
      if (isImg) {
        mw.innerHTML = `<div class="media-card">
          <img src="${src}" alt="${esc(text)}" style="max-height:480px;width:100%;object-fit:contain;display:block;background:#f0f0f0">
          <div class="media-foot">
            <div><span class="media-name">${outExt}</span>${savedTo}</div>
            <a class="dl-btn" href="${src}" download="pullai-image.png">↓ Scarica</a>
          </div>
        </div>`;
      } else if (isAudio) {
        mw.innerHTML = `<div class="media-card">
          <div class="audio-wrap">
            <audio controls style="width:100%"><source src="${src}" type="${d.mime}"></audio>
          </div>
          <div class="media-foot">
            <div><span class="media-name">Audio generato</span>${savedTo}</div>
            <a class="dl-btn" href="${src}" download="pullai-audio.wav">↓ Scarica</a>
          </div>
        </div>`;
      } else if (isVideo) {
        mw.innerHTML = `<div class="media-card">
          <video controls style="width:100%;max-height:400px;display:block;background:#000">
            <source src="${src}" type="${d.mime}">
          </video>
          <div class="media-foot">
            <div><span class="media-name">Video generato</span>${savedTo}</div>
            <a class="dl-btn" href="${src}" download="pullai-video.mp4">↓ Scarica</a>
          </div>
        </div>`;
      } else if (is3D) {
        mw.innerHTML = `<div class="media-card">
          <div style="padding:28px;text-align:center;display:flex;flex-direction:column;align-items:center;gap:10px">
            <div style="font-size:52px">🧊</div>
            <div style="font-weight:500">Modello 3D generato</div>
            ${savedTo}
            <a class="btn primary" href="${src}" download="pullai-model.obj" style="margin-top:4px">↓ Scarica OBJ</a>
          </div>
        </div>`;
      } else {
        mw.innerHTML = `<a class="btn" href="${src}" download="${outExt}">↓ Scarica ${outExt}</a>`;
      }
      contentEl.appendChild(mw);
    } else {
      contentEl.innerHTML = `<span style="color:#16a34a">✅ ${d.saved_to ? 'Salvato in: ' + esc(d.saved_to) : 'Completato'}</span>`;
    }
    toast('✅ Completato');
  } catch {
    contentEl.innerHTML = '<span style="color:#e04040">⚠️ Server non raggiungibile</span>';
  }
  canvas.scrollTop = 9999;
}

function addUserMsg(text, files) {
  const msgs = document.getElementById('msgs');
  const grp = document.createElement('div');
  grp.className = 'msg-group msg-user';
  const imgHtml = files.filter(f=>f.isImg).map(f=>`<img src="${f.dataUrl}" style="max-width:180px;border-radius:8px;margin-bottom:6px;display:block">`).join('');
  grp.innerHTML = `<div class="msg-inner"><div class="bubble">${imgHtml}${esc(text)||'(allegato)'}</div></div>`;
  msgs.appendChild(grp);
  document.getElementById('canvas').scrollTop = 9999;
}

// ── Panels ────────────────────────────────────────
function showPanel(name) {
  document.querySelectorAll('.panel').forEach(p => p.classList.remove('on'));
  const target = document.getElementById('ws-' + name);
  if (target) target.classList.add('on');
  if (name === 'models')   loadModels();
  if (name === 'registry') renderReg(REG);
  // Update topbar context
  const titles = {chat:'Chat', models:'Modelli installati', registry:'Scarica modelli'};
  const ttEl = document.getElementById('topbar-title');
  if (ttEl) ttEl.textContent = titles[name] || 'PullAI';
  // Save button only visible in chat panel when there are messages
  const scb = document.getElementById('save-chat-btn');
  if (scb) {
    const hasMsgs = document.getElementById('msgs')?.children?.length > 0;
    scb.style.display = (name === 'chat' && hasMsgs) ? 'flex' : 'none';
  }
}

// ── Models ────────────────────────────────────────
async function loadModels() {
  const el = document.getElementById('mlist');
  el.innerHTML = '<div style="color:var(--text3);padding:12px 0">Caricamento…</div>';
  try {
    const r = await fetch('/api/models');
    const d = await r.json();
    const models = d.models || [];
    if (!models.length) {
      el.innerHTML = '<div style="color:var(--text3);font-size:14px;padding:20px 0">Nessun modello installato. <button class="btn primary" onclick="showPanel(\'registry\')" style="margin-left:8px">↓ Scarica subito</button></div>';
      return;
    }
    el.innerHTML = models.map(m => {
      const ref = `${m.type}/${m.name}:${m.tag}`;
      const displayName = m.display_name ? `<strong>${esc(m.display_name)}</strong> <span style="font-size:11px;color:var(--text3)">(${esc(m.name)}:${esc(m.tag)})</span>` : `${esc(m.name)}:${esc(m.tag)}`;
      const caps = m.capabilities && m.capabilities.length
        ? m.capabilities.join(' · ')
        : ({llm:'testo → testo', image:'testo → immagine', audio:'testo ↔ audio', video:'testo → video', '3d':'immagine/testo → 3D'}[m.type] || '');
      return `<div class="mtrow">
        <span class="type-pill tp-${m.type === '3d' ? '3d' : m.type}">${m.type.toUpperCase()}</span>
        <div style="flex:1;min-width:0">
          <div style="font-weight:500;font-size:14px">${displayName}</div>
          ${caps ? `<div style="font-size:11px;color:var(--text3);margin-top:1px">${caps}</div>` : ''}
        </div>
        <span style="font-family:var(--mono);font-size:12px;color:var(--text3);flex-shrink:0">${m.size_human || ''}</span>
        <div style="display:flex;gap:4px;flex-shrink:0;flex-wrap:wrap">
          <button class="btn" style="padding:4px 9px;font-size:11px" onclick="openInfo('${esc(ref)}')">ℹ️ Info</button>
          <button class="btn" style="padding:4px 9px;font-size:11px" onclick="openRename('${esc(ref)}','${esc(m.display_name||'')}')">✏️ Rinomina</button>
          <button class="btn ${(defaultModels[m.type]===ref)?'primary':''}" style="padding:4px 9px;font-size:11px" onclick="setDefaultModel('${esc(ref)}')" title="Predefinito per questo tipo (clic per impostare/rimuovere)">
            ${(defaultModels[m.type]===ref)?'⭐ Predefinito':'☆ Default'}
          </button>
          <button class="btn danger" style="padding:4px 9px;font-size:11px" onclick="confirmDel('${esc(ref)}')">🗑 Rimuovi</button>
        </div>
      </div>`;
    }).join('');
  } catch { el.innerHTML = '<div style="color:var(--err,#dc2626);padding:12px 0">Errore caricamento modelli</div>'; }
}

let pendingDel = null;
let pendingRename = null;

function confirmDel(ref) {
  // Validate ref has type prefix before opening dialog
  if (!ref || !ref.includes('/')) {
    toast('❌ Ref modello non valido: ' + JSON.stringify(ref), 'err');
    console.error('confirmDel: invalid ref', ref);
    return;
  }
  pendingDel = ref;
  document.getElementById('dlg-body').innerHTML =
    `Rimuovere "<strong>${esc(ref)}</strong>"?<br><small style="color:var(--text3)">${esc(ref)}</small>`;
  document.getElementById('dlg').classList.add('show');
  document.getElementById('dlg-ok').onclick = doDel;
}
function closeDlg() { document.getElementById('dlg').classList.remove('show'); pendingDel = null; }

// ── Info modello ────────────────────────────────────────
async function openInfo(ref) {
  try {
    const r = await fetch('/api/models/info', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({model: ref})
    });
    if (!r.ok) { toast('Errore info', 'err'); return; }
    const m = await r.json();
    const lines = [
      ['Tipo', m.type], ['Nome', m.name], ['Tag', m.tag],
      ['Formato', m.format], ['Parametri', m.parameters || '—'],
      ['Dimensione', m.size_human || m.SizeHuman || '—'],
      ['Licenza', m.license || '—'],
      ['Fonte', m.source || '—'],
      ['Scaricato', m.downloaded_at ? new Date(m.downloaded_at).toLocaleString('it-IT') : '—'],
    ];
    document.getElementById('info-body').innerHTML = lines.map(([k,v]) =>
      `<div class="info-row"><span class="info-label">${k}</span><span class="info-value">${esc(String(v||'—'))}</span></div>`
    ).join('');
    document.getElementById('info-dlg').classList.add('show');
  } catch(e) { toast('❌ ' + e.message, 'err'); }
}

// ── Rinomina modello ─────────────────────────────────────
function openRename(ref, currentName) {
  pendingRename = ref;
  document.getElementById('rename-ref-lbl').textContent = ref;
  const inp = document.getElementById('rename-input');
  inp.value = currentName || '';
  document.getElementById('rename-dlg').classList.add('show');
  setTimeout(() => { inp.focus(); inp.select(); }, 60);
}

async function doRename() {
  if (!pendingRename) return;
  const inp = document.getElementById('rename-input');
  const name = inp ? inp.value.trim() : '';
  document.getElementById('rename-dlg').classList.remove('show');
  try {
    const r = await fetch('/api/models/rename', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({model: pendingRename, display_name: name})
    });
    const d = await r.json().catch(()=>({}));
    if (r.ok) {
      toast('✅ Rinominato', 'ok');
      pendingRename = null;
      loadModels();
    } else {
      toast('❌ ' + (d.error || 'Errore rinomina'), 'err');
    }
  } catch(e) { toast('❌ ' + e.message, 'err'); }
}
async function doDel() {
  if (!pendingDel) return;
  const ref = pendingDel; // capture BEFORE closeDlg nullifies pendingDel
  closeDlg();
  try {
    const r = await fetch('/api/models/remove', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({model: ref})
    });
    const d = await r.json().catch(()=>({}));
    if (r.ok) {
      toast('✅ Rimosso: ' + ref.split('/').slice(1).join('/'), 'ok');
      await refreshModels();
      loadModels();
    } else {
      toast('❌ ' + (d.error || 'Errore rimozione'), 'err');
    }
  } catch(e) { toast('❌ ' + e.message, 'err'); }
}

// ── Registry ──────────────────────────────────────
const REG = [
  {t:'llm',  n:'mistral:7b',        d:'Chat e coding, bilanciato',          s:'4.1 GB', l:'Apache-2.0'},
  {t:'llm',  n:'llama3:8b',         d:'Meta Llama 3, ottima qualità',        s:'4.7 GB', l:'Meta'},
  {t:'llm',  n:'phi3:mini',         d:'Veloce su CPU, 3.8B param',           s:'2.2 GB', l:'MIT'},
  {t:'llm',  n:'qwen:0.5b',         d:'Ultra-leggero, funziona ovunque',     s:'0.4 GB', l:'Apache-2.0'},
  {t:'image',n:'openjourney:latest',d:'Stile MidJourney, ideale 4GB VRAM',   s:'4.2 GB', l:'CreativeML'},
  {t:'image',n:'dreamshaper:latest',d:'Realistico e artistico, SD1.5',       s:'2.1 GB', l:'CreativeML'},
  {t:'image',n:'sdxl:latest',       d:'Stable Diffusion XL 1024px',          s:'6.9 GB', l:'CreativeML'},
  {t:'image',n:'flux:schnell',      d:'FLUX.1 schnell, alta qualità',        s:'23 GB',  l:'Apache-2.0'},
  {t:'audio',n:'whisper:large',     d:'STT 99 lingue, alta precisione',      s:'3.1 GB', l:'Apache-2.0'},
  {t:'audio',n:'whisper:base',      d:'STT leggero e veloce',                s:'290 MB', l:'Apache-2.0'},
  {t:'audio',n:'kokoro:latest',     d:'TTS naturale, 82M parametri',         s:'340 MB', l:'Apache-2.0'},
  {t:'audio',n:'bark:latest',       d:'TTS emotivo multilingua',             s:'5.5 GB', l:'MIT'},
  {t:'video',n:'wan:1.3b',          d:'Wan 2.1 T2V — leggero, 4GB VRAM',    s:'0.9 GB', l:'Apache-2.0'},
  {t:'video',n:'wan:14b',           d:'Wan 2.1 T2V — alta qualità',          s:'8.5 GB', l:'Apache-2.0'},
  {t:'video',n:'cogvideo:5b',       d:'CogVideoX 5B, testo→video HD',        s:'18 GB',  l:'CogVideoX'},
  {t:'video',n:'animatediff:v3',    d:'Video animati stilizzati',            s:'4.2 GB', l:'Apache-2.0'},
  {t:'3d',   n:'triposr:latest',    d:'Immagine → mesh 3D in secondi',       s:'~2 GB',  l:'MIT'},
  {t:'3d',   n:'shap-e:latest',     d:'Testo → mesh 3D (OpenAI)',            s:'~5 GB',  l:'MIT'},
];
const tIcon = {llm:'💬',image:'🎨',audio:'🔊',video:'🎬','3d':'🧊'};
const tClass = {llm:'tp-llm',image:'tp-image',audio:'tp-audio',video:'tp-video','3d':'tp-3d'};
let regFilter = 'all';

function safeId(s) { return String(s).replace(/[^a-z0-9]/gi,'_'); }

function renderReg(items) {
  const grid = document.getElementById('reg-grid');
  if (!grid) return;
  const filtered = items.filter(m => regFilter === 'all' || m.t === regFilter);
  grid.innerHTML = filtered.map(m => {
    const sid = safeId(m.t+'/'+m.n);
    return `
    <div class="rcard">
      <div style="display:flex;align-items:center;gap:7px;margin-bottom:6px">
        <span class="type-pill ${tClass[m.t]}">${tIcon[m.t]} ${m.t.toUpperCase()}</span>
        ${m.gpu ? `<span style="font-size:10px;color:var(--text3)">${m.gpu}</span>` : ''}
      </div>
      <div class="rcard-name">${m.n}</div>
      <div class="rcard-desc">${m.d}</div>
      <div id="dl-prog-${sid}" style="display:none;margin:8px 0 4px">
        <div style="display:flex;justify-content:space-between;font-size:11px;color:var(--text3);margin-bottom:4px">
          <span id="dl-lbl-${sid}">Connessione…</span>
          <span id="dl-pct-${sid}">0%</span>
        </div>
        <div style="background:var(--border);border-radius:3px;height:4px;overflow:hidden">
          <div id="dl-bar-${sid}" style="background:var(--accent);height:100%;width:0%;transition:width .25s ease;border-radius:3px"></div>
        </div>
        <button onclick="cancelDl('${m.t}/${m.n}')"
          style="margin-top:6px;width:100%;padding:4px;font-size:11px;border:1px solid var(--err-color,#dc2626);color:#dc2626;background:transparent;border-radius:4px;cursor:pointer;font-family:var(--sans)">
          ⏹ Annulla download
        </button>
      </div>
      <div class="rcard-foot" id="dl-foot-${sid}">
        <span class="rcard-sz">${m.s} · ${m.l}</span>
        <button class="btn primary" id="dl-btn-${sid}" style="font-size:12px;padding:5px 14px"
          onclick="pullModel('${m.t}/${m.n}',this)">↓ Scarica</button>
      </div>
    </div>`;
  }).join('');
}

function filterType(t, btn) {
  regFilter = t;
  document.querySelectorAll('.rf-btn').forEach(b => b.classList.remove('on'));
  btn.classList.add('on');
  renderReg(REG);
}

function filterReg(q) {
  const items = REG.filter(m =>
    (regFilter === 'all' || m.t === regFilter) &&
    (m.n.toLowerCase().includes(q.toLowerCase()) || m.d.toLowerCase().includes(q.toLowerCase()))
  );
  document.getElementById('reg-grid').innerHTML = items.map(m => `
    <div class="rcard">
      <span class="type-pill ${tClass[m.t]}">${tIcon[m.t]} ${m.t.toUpperCase()}</span>
      <div class="rcard-name">${m.n}</div>
      <div class="rcard-desc">${m.d}</div>
      <div class="rcard-foot">
        <span class="rcard-sz">${m.s}</span>
        <button class="btn" style="font-size:12px;padding:5px 12px" onclick="pullModel('${m.t}/${m.n}',this)">↓ Scarica</button>
      </div>
    </div>`).join('');
}

function showCustomInstall() {
  const el = document.getElementById('custom-install');
  el.style.display = el.style.display === 'none' ? 'block' : 'none';
  if (el.style.display !== 'none') {
    setTimeout(() => document.getElementById('custom-url').focus(), 50);
  }
}

async function installCustom() {
  const urlInput = document.getElementById('custom-url').value.trim();
  const type = document.getElementById('custom-type').value;
  const btn = document.getElementById('custom-btn');
  if (!urlInput) { toast('Inserisci URL o nome repo HuggingFace', 'err'); return; }

  let ref;
  const inp = urlInput.trim();
  if (inp.startsWith('https://') || inp.startsWith('http://')) {
    // Full URL: type/https://...
    ref = `${type}/${inp}`;
  } else if (inp.startsWith('hf.co/')) {
    // Already hf.co/owner/repo:file → add type prefix only
    ref = `${type}/${inp}`;
  } else if (inp.includes('/') && !inp.startsWith(type + '/')) {
    // owner/repo or owner/repo:file → add type/hf.co/ prefix
    ref = `${type}/hf.co/${inp}`;
  } else if (inp.startsWith(type + '/')) {
    // Already type/name:tag
    ref = inp;
  } else if (inp.includes(':') && !inp.includes('/')) {
    // name:tag shorthand → type/name:tag
    ref = `${type}/${inp}`;
  } else {
    ref = inp;
  }

  document.getElementById('custom-url').dataset.ref = ref;
  btn.textContent = 'Connessione…'; btn.disabled = true;
  const progEl = document.getElementById('custom-dl-progress');
  const barEl  = document.getElementById('custom-dl-bar');
  const lblEl  = document.getElementById('custom-dl-lbl');
  const pctEl  = document.getElementById('custom-dl-pct');
  if (progEl) progEl.style.display = 'block';
  if (barEl) barEl.style.width = '0%';

  customDlAC = new AbortController();
  try {
    const r = await fetch('/api/pull', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body:JSON.stringify({model:ref}), signal:customDlAC.signal
    });
    if (!r.ok) { const d=await r.json().catch(()=>({})); throw new Error(d.error||'Errore server'); }
    const reader = r.body.getReader(), dec = new TextDecoder();
    let buf = '';
    outer: while (true) {
      const {done, value} = await reader.read(); if (done) break;
      buf += dec.decode(value, {stream:true});
      const chunks = buf.split('\n\n'); buf = chunks.pop();
      for (const chunk of chunks) {
        let etype='', dline='';
        for (const line of chunk.split('\n')) {
          if (line.startsWith('event: ')) etype = line.slice(7).trim();
          else if (line.startsWith('data: ')) dline = line.slice(6).trim();
        }
        if (!dline) continue;
        try {
          const ev = JSON.parse(dline);
          if (etype === 'progress') {
            const p = Math.min(100, ev.pct||0);
            if (barEl) barEl.style.width = p + '%';
            if (lblEl) lblEl.textContent = ev.msg || 'Download…';
            if (pctEl) pctEl.textContent = p + '%';
            btn.textContent = p + '%';
          } else if (etype === 'done') {
            if (progEl) progEl.style.display = 'none';
            document.getElementById('custom-url').value = '';
            document.getElementById('custom-install').style.display = 'none';
            await refreshModels();
            toast('✅ Scaricato: ' + ref, 'ok');
            break outer;
          } else if (etype === 'error') { throw new Error(ev.error||'Errore download'); }
        } catch(pe) { if (pe.name==='SyntaxError') continue; throw pe; }
      }
    }
  } catch(e) {
    if (progEl) progEl.style.display = 'none';
    if (e.name !== 'AbortError') toast('❌ ' + (e.message||'Server offline'), 'err');
  }
  customDlAC = null;
  btn.textContent = 'Installa'; btn.disabled = false;
}

// Active download controllers { ref: AbortController }
const activeDls = {};

function cancelDl(ref) {
  if (activeDls[ref]) { activeDls[ref].abort(); delete activeDls[ref]; }
  fetch('/api/pull/cancel', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({model:ref})}).catch(()=>{});
  const sid = safeId(ref);
  const progEl = document.getElementById('dl-prog-'+sid);
  const footEl = document.getElementById('dl-foot-'+sid);
  const btnEl  = document.getElementById('dl-btn-'+sid);
  if (progEl) progEl.style.display = 'none';
  if (footEl) footEl.style.display = 'flex';
  if (btnEl)  { btnEl.textContent = '↓ Scarica'; btnEl.disabled = false; btnEl.style.color = ''; }
  toast('⏹ Download annullato');
}

async function pullModel(ref, btn) {
  const sid = safeId(ref);
  const progEl = document.getElementById('dl-prog-'+sid);
  const footEl = document.getElementById('dl-foot-'+sid);
  const barEl  = document.getElementById('dl-bar-'+sid);
  const lblEl  = document.getElementById('dl-lbl-'+sid);
  const pctEl  = document.getElementById('dl-pct-'+sid);

  // Show progress bar, hide footer
  if (progEl) progEl.style.display = 'block';
  if (btn) { btn.disabled = true; btn.textContent = '…'; }

  const ac = new AbortController();
  activeDls[ref] = ac;

  try {
    const r = await fetch('/api/pull', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body:JSON.stringify({model:ref}), signal:ac.signal
    });
    if (!r.ok) {
      const d = await r.json().catch(()=>({}));
      const msg = d.error || ('HTTP ' + r.status);
      console.error('pullModel error:', msg, 'ref:', ref);
      throw new Error(msg);
    }

    const reader = r.body.getReader(), dec = new TextDecoder();
    let buf = '';
    outer: while (true) {
      const {done, value} = await reader.read(); if (done) break;
      buf += dec.decode(value, {stream:true});
      const chunks = buf.split('\n\n'); buf = chunks.pop();
      for (const chunk of chunks) {
        let etype='', dline='';
        for (const line of chunk.split('\n')) {
          if (line.startsWith('event: ')) etype = line.slice(7).trim();
          else if (line.startsWith('data: ')) dline = line.slice(6).trim();
        }
        if (!dline) continue;
        try {
          const ev = JSON.parse(dline);
          if (etype === 'progress') {
            if (barEl) barEl.style.width = Math.min(100,ev.pct) + '%';
            if (lblEl) lblEl.textContent = ev.msg || 'Download…';
            if (pctEl) pctEl.textContent = ev.pct + '%';
          } else if (etype === 'done') {
            if (progEl) progEl.style.display = 'none';
            if (footEl) footEl.style.display = 'flex';
            if (btn) { btn.textContent = '✅ Installato'; btn.style.color='#16a34a'; btn.disabled=false; }
            delete activeDls[ref];
            toast('✅ Scaricato: ' + ref, 'ok');
            await refreshModels();
            break outer;
          } else if (etype === 'error') { throw new Error(ev.error||'Errore'); }
        } catch(pe){ if(pe.name!=='SyntaxError') throw pe; }
      }
    }
  } catch(e) {
    delete activeDls[ref];
    if (progEl) progEl.style.display = 'none';
    if (footEl) footEl.style.display = 'flex';
    if (btn) { btn.textContent='↓ Scarica'; btn.disabled=false; btn.style.color=''; }
    if (e.name !== 'AbortError') toast('❌ ' + (e.message||'Offline'), 'err');
  }
}


// ── Stop generation ───────────────────────────────────
function setGenerating(on) {
  const btn = document.getElementById('send-btn');
  const sendIcon = document.getElementById('send-icon');
  const stopIcon = document.getElementById('stop-icon');
  if (on) {
    btn.onclick = stopGeneration;
    btn.style.background = '#dc2626';
    sendIcon.style.display = 'none';
    stopIcon.style.display = 'block';
  } else {
    btn.onclick = doSend;
    btn.style.background = '';
    sendIcon.style.display = 'block';
    stopIcon.style.display = 'none';
    btn.disabled = false;
  }
}

function stopGeneration() {
  if (currentAbortController) {
    currentAbortController.abort();
    currentAbortController = null;
  }
  setGenerating(false);
  toast('⏹ Generazione annullata');
}

// ── Drag & Drop ──────────────────────────────────────
function inputDragOver(e) {
  e.preventDefault();
  e.stopPropagation();
  document.getElementById('input-box').classList.add('drag-over');
}
function inputDragLeave(e) {
  if (!e.currentTarget.contains(e.relatedTarget)) {
    document.getElementById('input-box').classList.remove('drag-over');
  }
}
function handleDrop(e) {
  e.preventDefault();
  e.stopPropagation();
  document.getElementById('input-box').classList.remove('drag-over');
  const files = e.dataTransfer?.files;
  if (files && files.length > 0) {
    handleFiles(files);
    // If text was dropped, put it in the input
  } else {
    const text = e.dataTransfer?.getData('text');
    if (text) {
      const inp = document.getElementById('inp');
      inp.value = (inp.value + ' ' + text).trim();
      autoSize(inp);
    }
  }
}

// ── Utils ─────────────────────────────────────────
function toast(msg, type='') {
  const el = document.getElementById('toast');
  el.textContent = msg; el.className = 'toast show' + (type ? ' '+type : '');
  setTimeout(() => el.className = 'toast', 3000);
}
function esc(s) {
  return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// ── Save/Load Chats ─────────────────────────────────────

function openSaveChat() {
  const inp = document.getElementById('save-name-input');
  if (inp) {
    const existing = savedChats.find(c => c.id === currentChatId);
    inp.value = existing ? existing.name : ('Chat ' + new Date().toLocaleTimeString('it-IT', {hour:'2-digit', minute:'2-digit'}));
    setTimeout(() => { inp.focus(); inp.select(); }, 60);
  }
  document.getElementById('save-dlg').classList.add('show');
}

function doSaveChat() {
  const nameEl = document.getElementById('save-name-input');
  const name = nameEl ? nameEl.value.trim() : '';
  if (!name) return;
  document.getElementById('save-dlg').classList.remove('show');
  const msgsHtml = document.getElementById('msgs').innerHTML;
  const id = currentChatId || Date.now().toString();
  currentChatId = id;
  const chatData = { id, name, messages: chatHistory.slice(), msgsHtml, domain, model: selectedModel, savedAt: new Date().toISOString() };
  const idx = savedChats.findIndex(c => c.id === id);
  if (idx >= 0) savedChats[idx] = chatData; else savedChats.unshift(chatData);
  if (savedChats.length > 50) savedChats = savedChats.slice(0, 50);
  try { localStorage.setItem('pullai_chats', JSON.stringify(savedChats)); } catch(e) {}
  renderChatHistory();
  toast('💾 Chat salvata: ' + name, 'ok');
}

function loadSavedChats() {
  try { savedChats = JSON.parse(localStorage.getItem('pullai_chats') || '[]'); } catch(e) { savedChats = []; }
  renderChatHistory();
}

function renderChatHistory() {
  const el = document.getElementById('hist-list');
  if (!el) return;
  if (!savedChats.length) {
    el.innerHTML = `<div style="font-size:12px;color:var(--text3);padding:6px 10px">${t('noChats')}</div>`;
    return;
  }
  el.innerHTML = savedChats.map((c, i) => `
    <div class="hist-item ${c.id===currentChatId?'active':''}" title="${esc(c.name)}">
      <span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" onclick="loadChat(${i})">${esc(c.name)}</span>
      <span style="cursor:pointer;opacity:.4;padding:0 4px;font-size:14px;flex-shrink:0" onclick="deleteChat(${i})" title="Elimina">×</span>
    </div>`).join('');
}

function loadChat(idx) {
  const c = savedChats[idx]; if (!c) return;
  // Restore full chatHistory so LLM gets full context on next message
  chatHistory = (c.messages || []).map(m => ({...m}));
  currentChatId = c.id;
  if (c.model) {
    const found = installedModels.find(m => `${m.type}/${m.name}:${m.tag}` === c.model);
    if (found) {
      selectedModel = c.model;
      const lbl = found.display_name || (found.name + ':' + found.tag);
      document.getElementById('model-lbl').textContent = lbl;
    }
  }
  document.getElementById('welcome').style.display = 'none';
  document.getElementById('msgs').style.display = 'block';
  document.getElementById('msgs').innerHTML = c.msgsHtml || '';
  const sb = document.getElementById('save-chat-btn');
  if (sb) sb.style.display = 'flex';
  showPanel('chat');
  document.getElementById('canvas').scrollTop = 9999;
  toast('💬 Chat ripresa: ' + c.name + ' (' + chatHistory.filter(m=>m.role==="user").length + ' messaggi)');
}

function deleteChat(idx) {
  savedChats.splice(idx, 1);
  try { localStorage.setItem('pullai_chats', JSON.stringify(savedChats)); } catch(e) {}
  renderChatHistory();
  toast('Chat eliminata');
}

// ── Default Model ────────────────────────────────────────

function setDefaultModel(ref) {
  const type = ref.split('/')[0];
  if (defaultModels[type] === ref) {
    // Already default → unset
    delete defaultModels[type];
    toast('☆ Predefinito rimosso per ' + type.toUpperCase());
  } else {
    defaultModels[type] = ref;
    toast('⭐ Predefinito per ' + type.toUpperCase() + ': ' + ref.split('/').slice(1).join('/'));
  }
  localStorage.setItem('pullai_default_models', JSON.stringify(defaultModels));
  loadModels();
}

function applyDefaultModel() {
  const typeMap = {chat:'llm', image:'image', audio:'audio', video:'video', threed:'3d'};
  const t = typeMap[domain] || 'llm';
  const def = defaultModels[t];
  if (!def) return;
  const found = installedModels.find(m => `${m.type}/${m.name}:${m.tag}` === def);
  if (found) {
    selectedModel = def;
    document.getElementById('model-lbl').textContent = found.display_name || (found.name + ':' + found.tag);
  }
}


// ── CPU/GPU Toggle ────────────────────────────────────────
function setHW(cpu) {
  useCPU = cpu;
  const gpuBtn = document.getElementById('hw-gpu-btn');
  const cpuBtn = document.getElementById('hw-cpu-btn');
  if (gpuBtn) gpuBtn.classList.toggle('active', !cpu);
  if (cpuBtn) cpuBtn.classList.toggle('active', cpu);
}

// ── Cancel custom download ────────────────────────────────
let customDlAC = null;
function cancelCustomDl() {
  if (customDlAC) { customDlAC.abort(); customDlAC = null; }
  const ref = document.getElementById('custom-url')?.dataset?.ref || '';
  if (ref) fetch('/api/pull/cancel', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({model:ref})}).catch(()=>{});
  const progEl = document.getElementById('custom-dl-progress');
  if (progEl) progEl.style.display = 'none';
  const btn = document.getElementById('custom-btn');
  if (btn) { btn.textContent = 'Installa'; btn.disabled = false; }
  toast('⏹ Download annullato');
}


// Spinning keyframe for pull button
const style = document.createElement('style');
style.textContent = '@keyframes spinning{to{transform:rotate(360deg)}}';
document.head.appendChild(style);

init();
</script>
</body>
</html>
