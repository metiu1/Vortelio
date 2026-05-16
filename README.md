<div align="center">

<img src="vortelio-installer/assets/pullai.ico" alt="Vortelio" width="96" height="96" />

# Vortelio

**Run any AI model locally. LLMs, images, audio, video, 3D — one binary, web UI included.**

[![Release](https://img.shields.io/github/v/release/metiu1/Vortelio?style=flat-square)](https://github.com/metiu1/Vortelio/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg?style=flat-square)](LICENSE)
[![PyPI](https://img.shields.io/pypi/v/vortelio?style=flat-square)](https://pypi.org/project/vortelio/)
[![Downloads](https://img.shields.io/github/downloads/metiu1/Vortelio/total?style=flat-square)](https://github.com/metiu1/Vortelio/releases)
[![Stars](https://img.shields.io/github/stars/metiu1/Vortelio?style=flat-square)](https://github.com/metiu1/Vortelio/stargazers)
[![Go Report](https://goreportcard.com/badge/github.com/metiu1/Vortelio?style=flat-square)](https://goreportcard.com/report/github.com/metiu1/Vortelio)
[![CI](https://img.shields.io/github/actions/workflow/status/metiu1/Vortelio/ci.yml?branch=main&style=flat-square)](https://github.com/metiu1/Vortelio/actions)

[Install](#-install) • [Quickstart](#-quickstart) • [Features](#-features) • [Models](#-supported-models) • [API](#-api) • [Contributing](#-contributing)

</div>

---

## Why Vortelio

> **Ollama for everything.** Run LLMs, Stable Diffusion, Whisper, Kokoro, WAN 2.1, TripoSR — all from one CLI, one server, one web UI. No Docker. No Python juggling. No cloud lock-in.

| | Vortelio | Ollama | LM Studio | ComfyUI |
|---|:---:|:---:|:---:|:---:|
| LLM chat | ✅ | ✅ | ✅ | ❌ |
| Image gen (SD/FLUX) | ✅ | ❌ | ❌ | ✅ |
| Audio (Whisper + TTS) | ✅ | ❌ | ❌ | ❌ |
| Video (WAN 2.1) | ✅ | ❌ | ❌ | ⚠️ |
| 3D (TripoSR, Shap-E) | ✅ | ❌ | ❌ | ❌ |
| Web UI included | ✅ | ❌ | ✅ | ✅ |
| OpenAI / Ollama API compat | ✅ | ✅ | ✅ | ❌ |
| Cloud provider proxy (8 providers) | ✅ | ❌ | ❌ | ❌ |
| Single binary install | ✅ | ✅ | ❌ | ❌ |

---

## 🚀 Install

### Via `uv` (cross-platform)

```bash
uv tool install vortelio
vortelio gui
```

### Via `pip`

```bash
pip install vortelio
```

### Windows installer

Download `Vortelio-Setup-x.y.z.exe` from [Releases](https://github.com/metiu1/Vortelio/releases) → double-click.

### Build from source

```bash
git clone https://github.com/metiu1/Vortelio
cd Vortelio/vortelio
go build -o vortelio ./cmd/vortelio
./vortelio gui
```

---

## ⚡ Quickstart

```bash
# Open the web UI (auto-starts background server)
vortelio gui

# Pull a model from HuggingFace
vortelio pull llama-3.2-3b-instruct

# Run inference from the CLI
vortelio run llama-3.2-3b-instruct "Explain quantum entanglement in one tweet"

# Serve OpenAI / Ollama compatible API
vortelio serve --port 11500

# Use it from any OpenAI SDK
curl http://localhost:11500/v1/chat/completions \
  -d '{"model":"llama-3.2-3b-instruct","messages":[{"role":"user","content":"hi"}]}'
```

---

## ✨ Features

- 🧠 **LLMs** — llama.cpp backend, GGUF, chat + completions, streaming
- 🎨 **Images** — Stable Diffusion, FLUX, SDXL via `diffusers`
- 🎤 **Audio** — Whisper STT (faster-whisper), Kokoro/Bark TTS
- 🎬 **Video** — WAN 2.1, AnimateDiff, CogVideo
- 🧊 **3D** — TripoSR, Shap-E, LGM, TRELLIS
- 🌐 **Web UI** — single-file HTML, dark theme, code blocks, streaming
- 🔌 **API compatibility** — drop-in for OpenAI `/v1/chat/completions` and Ollama `/api/chat`
- ☁️ **Cloud proxy** — OpenAI, Anthropic, Gemini, Groq, Mistral, OpenRouter, Ollama Cloud
- 🤖 **AI agents** — install/start/stop OpenClaw, Open Code from the UI
- 🔐 **Local auth** — email+password, Google OAuth, session in browser
- ⚙️ **Hardware aware** — auto-detects CUDA, ROCm, Metal, CPU
- 📦 **Zero-config** — one binary, models in `~/.pullai/models/`

---

## 🧩 Supported models

LLM: Llama, Qwen, Mistral, Gemma, Phi (any GGUF). Image: Stable Diffusion 1.5/2/XL, FLUX.1, Kandinsky. Audio: Whisper (all sizes), Kokoro, Bark. Video: WAN 2.1, AnimateDiff, CogVideo-X. 3D: TripoSR, Shap-E, LGM, TRELLIS.

Pull anything from HuggingFace:

```bash
vortelio pull <org>/<repo>
```

---

## 🌐 API

OpenAI-compatible:

```bash
POST /v1/chat/completions
POST /v1/embeddings
POST /v1/images/generations
POST /v1/audio/transcriptions
POST /v1/audio/speech
```

Ollama-compatible:

```bash
POST /api/chat
POST /api/generate
GET  /api/tags
```

Native:

```bash
GET  /api/status            # hardware, version, model count
POST /api/pull              # download model (SSE progress)
POST /api/generate          # run any model type (SSE)
POST /api/agents/install    # install AI agent
```

Full route list in [`vortelio/internal/server/server.go`](vortelio/internal/server/server.go).

---

## 🖥️ Screenshots

> Drop demo GIFs / screenshots in `docs/screenshots/` and reference here.

```
[ chat UI ]   [ image gen ]   [ model hub ]   [ cloud panel ]
```

---

## 🛠️ Tech stack

Go (server, CLI, llama.cpp wrapper) · Python (diffusers, faster-whisper, kokoro-onnx) · vanilla JS (UI, single HTML) · NSIS (Windows installer) · `uv` / hatchling (Python wrapper).

---

## 🤝 Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Report security issues per [SECURITY.md](SECURITY.md).

```bash
git clone https://github.com/metiu1/Vortelio
cd Vortelio/vortelio
go test ./...
```

---

## 📜 License

[Apache 2.0](LICENSE) © Vortelio Contributors

---

## ⭐ Star history

[![Star History Chart](https://api.star-history.com/svg?repos=metiu1/Vortelio&type=Date)](https://star-history.com/#metiu1/Vortelio&Date)

<div align="center">

**[Website](https://github.com/metiu1/Vortelio) • [Releases](https://github.com/metiu1/Vortelio/releases) • [Issues](https://github.com/metiu1/Vortelio/issues) • [Discussions](https://github.com/metiu1/Vortelio/discussions)**

Made with ⚡ by the Vortelio community.

</div>
