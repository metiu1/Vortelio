<div align="center">

<img src="vortelio-installer/assets/pullai.ico" alt="Vortelio" width="96" height="96" />

# Vortelio

**The local-first AI platform. Run LLMs, generate images, transcribe audio, create video and 3D — all on your own machine. One binary. Web UI included. No cloud. No subscriptions.**

*Open-source Ollama alternative for every AI modality — text, image, audio, video, 3D.*

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

## 🩹 Problems Vortelio solves

| Pain | Today | With Vortelio |
|------|-------|---------------|
| 💸 **Cloud AI bills explode** — $20/mo ChatGPT × N seats, OpenAI API tokens add up | Pay per call, forever | Run unlimited on your own GPU. Zero per-token cost. |
| 🔒 **Sensitive data leaves your machine** — medical, legal, financial, NDA | Sent to OpenAI/Anthropic/Google servers | 100% offline. Nothing leaves localhost. |
| 🧩 **5 different apps for 5 modalities** — Ollama for text, A1111 for images, Whisper script for audio, ComfyUI for video, separate tool for 3D | Juggle Python envs, ports, configs | One binary. One UI. One API. All modalities. |
| 🐍 **Python dependency hell** — CUDA / torch / diffusers / espeak conflicts | Hours of `pip install` debugging | `vortelio pull <model>` — deps auto-installed per model |
| 🌐 **No internet, no AI** — flights, secure facilities, rural areas | App breaks without API | Works fully offline once models are pulled |
| 🔁 **Locked into one provider** — rewriting code to switch from OpenAI to Claude | Vendor lock-in | OpenAI-compat + Ollama-compat API + 8 cloud providers proxied |
| 🖥️ **"It works on my Mac" syndrome** | Different stack per OS | Same binary on Windows / Linux / macOS / WSL |
| 🛡️ **Compliance / GDPR / HIPAA** | Cloud AI = data residency nightmare | Self-hosted, on-prem, air-gapped |
| 🚀 **Slow first-token latency from cloud** | 500-2000ms TTFT over network | Sub-100ms local TTFT |
| 🧪 **Hard to test multiple models** | Sign up + API key per provider | `vortelio pull`, run, compare in one UI |

## Why Vortelio

> **Ollama for everything.** Run LLMs, Stable Diffusion, Whisper, Kokoro, WAN 2.1, TripoSR — all from one CLI, one server, one web UI. No Docker. No Python juggling. No cloud lock-in.

### Who is this for

- 👩‍💻 **Indie devs** building AI features without burning runway on API bills
- 🏢 **Enterprises** with strict data residency / compliance requirements
- 🎨 **Creators** generating images / video / voice locally without watermarks or rate limits
- 🔬 **Researchers** running reproducible offline experiments
- 🕹️ **Hobbyists** with a gaming GPU sitting idle

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

## ❓ FAQ

**Is Vortelio really free?**
Yes. Apache 2.0. No paid tier, no telemetry, no account required.

**Does it work without a GPU?**
Yes for LLMs (CPU inference via llama.cpp) and Whisper. Image/video/3D need a GPU (NVIDIA CUDA, AMD ROCm, or Apple Metal) to be usable.

**How does it compare to Ollama?**
Ollama only runs LLMs. Vortelio runs LLMs **plus** images, audio (STT + TTS), video, 3D — and exposes Ollama-compatible API so existing Ollama clients work unchanged.

**Can I use Vortelio with LangChain / LlamaIndex / OpenAI SDK?**
Yes. Vortelio serves OpenAI-compatible `/v1/chat/completions`, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/*`. Point any OpenAI SDK at `http://localhost:11500`.

**Where are my data stored?**
`~/.pullai/models/` for models. Chats live in your browser's `localStorage`. Nothing is sent to a server unless you explicitly enable a Cloud provider.

**Can I run it offline / air-gapped?**
Yes. Pull models once on a connected machine, copy `~/.pullai/` over, run.

**Does it support Apple Silicon (M1/M2/M3/M4)?**
Yes — Metal acceleration via llama.cpp for LLMs, MPS for diffusers.

**Can I self-host for my team?**
Yes. `vortelio serve --port 11500 --host 0.0.0.0` and point teammates' OpenAI SDK at your box.

---

## ⭐ Star history

[![Star History Chart](https://api.star-history.com/svg?repos=metiu1/Vortelio&type=Date)](https://star-history.com/#metiu1/Vortelio&Date)

<div align="center">

**[Website](https://github.com/metiu1/Vortelio) • [Releases](https://github.com/metiu1/Vortelio/releases) • [Issues](https://github.com/metiu1/Vortelio/issues) • [Discussions](https://github.com/metiu1/Vortelio/discussions)**

Made with ⚡ by the Vortelio community.

</div>

---

<details>
<summary>🔍 Keywords / topics</summary>

`local ai` · `local llm` · `run llm locally` · `offline ai` · `self-hosted ai` · `private ai` · `ollama alternative` · `lm studio alternative` · `gpt4all alternative` · `jan ai alternative` · `localai alternative` · `open source chatgpt` · `chatgpt local` · `claude local` · `llama local` · `mistral local` · `qwen local` · `gemma local` · `phi local` · `llama.cpp` · `gguf` · `huggingface` · `stable diffusion local` · `flux local` · `sdxl local` · `text to image local` · `image generation offline` · `whisper local` · `speech to text offline` · `tts local` · `text to speech offline` · `kokoro tts` · `bark tts` · `wan 2.1 local` · `text to video local` · `animatediff` · `cogvideo` · `text to 3d` · `image to 3d` · `triposr` · `shap-e` · `lgm` · `trellis` · `openai api compatible` · `openai compatible server` · `ollama api compatible` · `drop-in openai replacement` · `langchain local` · `llamaindex local` · `local rag` · `local embeddings` · `local agent` · `ai agent local` · `openclaw` · `open code agent` · `go ai server` · `golang ai` · `single binary ai` · `windows ai` · `linux ai` · `macos ai` · `apple silicon ai` · `m1 m2 m3 m4 llm` · `cuda llm` · `rocm llm` · `metal llm` · `cpu llm` · `low vram llm` · `quantized llm` · `gguf quantization` · `ai chatbot self-hosted` · `private chatgpt` · `gdpr ai` · `hipaa ai` · `air-gapped ai` · `enterprise ai on-prem` · `data sovereignty ai` · `no api key ai` · `free chatgpt alternative` · `unlimited ai` · `no rate limit ai` · `ai studio` · `multimodal ai` · `omni ai` · `all-in-one ai` · `ai toolkit` · `ai playground` · `local inference` · `edge ai` · `vortelio`

</details>

<!--
SEO meta (rendered as HTML comment so it does not show but is indexed in raw):
keywords: vortelio, local ai, local llm, ollama alternative, lm studio alternative, run llm locally, offline ai, self-hosted ai, private ai, stable diffusion local, flux local, whisper offline, kokoro tts, wan 2.1, triposr, shap-e, openai api compatible, ollama api compatible, golang ai server, single binary ai platform, apple silicon llm, cuda llm, multimodal ai studio, gdpr compliant ai, air-gapped ai, free chatgpt alternative
description: Vortelio is the open-source, local-first AI platform. Run LLMs, generate images and video, transcribe audio, and create 3D — all on your own machine. One binary. OpenAI & Ollama API compatible. Apache 2.0.
-->

