# Vortelio v0.3.48

> Esegui modelli AI in locale su Windows — LLM, immagini, audio, video, 3D. Interfaccia web inclusa.

---

## Struttura del progetto

```
pullai-0.3.38/
├── pullai/                        # Codice sorgente Go (il binario si chiama "vortelio")
│   ├── cmd/pullai/main.go         # Entry point — avvia CLI o server
│   ├── go.mod / go.sum            # Dipendenze Go
│   └── internal/
│       ├── version/version.go     # Versione corrente ("0.3.46")
│       ├── server/
│       │   ├── server.go          # Server HTTP Go — tutte le route API
│       │   └── ui.html            # Intera GUI web (HTML + CSS + JS, file singolo)
│       ├── cli/
│       │   ├── root.go            # Help, routing comandi, branding CLI
│       │   └── commands/
│       │       ├── misc.go        # Comandi: gui, serve, list, info, cleanup
│       │       ├── pull.go        # Comando: pull (scarica modelli)
│       │       ├── run.go         # Comando: run (esegui modelli)
│       │       ├── remove.go      # Comando: rm / remove
│       │       ├── quantize.go    # Comando: quantize
│       │       ├── setup.go       # Comando: setup (installa dipendenze)
│       │       ├── service_windows.go  # Avvio server in background su Windows
│       │       └── service_other.go    # Stub per Linux/macOS
│       ├── runtime/
│       │   ├── runtime.go         # DetectHardware, RunOptions, interfacce Runner
│       │   ├── llm_runner.go      # LLM via llama.cpp — chat testuale
│       │   ├── image_runner.go    # Immagini via diffusers Python
│       │   ├── audio_runner.go    # Audio: Whisper (STT) + Kokoro/Bark (TTS)
│       │   ├── video_runner.go    # Video: WAN 2.1, AnimateDiff, CogVideo
│       │   ├── threed_runner.go   # 3D: TripoSR, Shap-E, LGM, TRELLIS
│       │   ├── python.go          # FindPython, InstallPythonPackage, CheckPythonPackage
│       │   ├── backends.go        # Gestione llama.cpp binary
│       │   ├── output_dir.go      # ResolveOutputPath (default: Downloads)
│       │   ├── proc_windows.go    # HideWindow per subprocess invisibili su Windows
│       │   └── proc_other.go      # Stub Linux/macOS
│       ├── hub/
│       │   ├── hub.go             # Download modelli da HuggingFace
│       │   └── store.go           # ModelStore — lista/salva/rimuovi modelli locali
│       ├── agent/
│       │   └── agent.go           # Agenti AI: OpenClaw, Open Code (install/start/stop)
│       └── llama/
│           └── llama.go           # Wrapper llama.cpp per LLM locali
├── pullai-installer/
│   ├── pullai-installer.nsi       # Script NSIS per creare il .exe installer
│   ├── build/
│   │   ├── pullai.exe             # ← Binario Windows copiato qui prima di NSIS
│   │   └── pullai-server.exe      # ← Stessa copia (avviato come server in background)
│   └── assets/
│       └── pullai.ico             # Icona app
└── README.md                      # Questo file
```

---

## Come compilare il .exe

### Prerequisiti

| Tool | Versione | Dove scaricarlo |
|------|----------|-----------------|
| **Go** | ≥ 1.21 | https://go.dev/dl/ |
| **NSIS** | ≥ 3.09 | https://nsis.sourceforge.io |
| **Git** | qualsiasi | https://git-scm.com |

Su Windows con Chocolatey:
```powershell
choco install golang nsis git
```

### Build step-by-step

**1. Verifica la versione** in `pullai/internal/version/version.go`:
```go
const Version = "0.3.46"
```

**2. Compila il binario Windows** (cross-compile da Linux/Mac, o build nativo su Windows):
```bash
cd pullai
GOOS=windows GOARCH=amd64 go build -o dist/vortelio-windows-amd64.exe ./cmd/pullai/
```
Su Windows nativamente:
```cmd
cd pullai
go build -o dist\vortelio-windows-amd64.exe .\cmd\pullai\
```

**3. Copia i binari nella cartella build dell'installer**:
```bash
cp pullai/dist/vortelio-windows-amd64.exe pullai-installer/build/pullai.exe
cp pullai/dist/vortelio-windows-amd64.exe pullai-installer/build/pullai-server.exe
```
> Nota: NSIS rinomina `pullai.exe` → `vortelio.exe` e `pullai-server.exe` → `vortelio-server.exe`
> durante l'installazione usando la direttiva `/oname=`.

**4. Crea l'installer**:
```bash
cd pullai-installer
makensis pullai-installer.nsi
```
Output: `pullai-installer/Vortelio-Setup-0.3.46.exe`

### Build rapido (script unico)
```bash
cd pullai
go build ./...  # verifica che compili
GOOS=windows GOARCH=amd64 go build -o dist/vortelio-windows-amd64.exe ./cmd/pullai/
cp dist/vortelio-windows-amd64.exe ../pullai-installer/build/pullai.exe
cp dist/vortelio-windows-amd64.exe ../pullai-installer/build/pullai-server.exe
cd ../pullai-installer
makensis pullai-installer.nsi
```

---

## Come funziona l'avvio su Windows

1. L'utente clicca il collegamento **Vortelio** sul Desktop
2. Viene eseguito `vortelio.exe gui`
3. `service_windows.go` cerca `vortelio-server.exe` nella stessa cartella (fallback: sé stesso)
4. Lancia `vortelio-server.exe serve --port 11500 --no-browser` come processo invisibile in background
5. Aspetta fino a 16 secondi che `/api/status` risponda
6. Apre Edge/Chrome in modalità app (`--app=http://localhost:11500`)
7. La GUI si connette automaticamente al server

---

## Route API del server (`server.go`)

| Endpoint | Metodo | Descrizione |
|----------|--------|-------------|
| `/api/status` | GET | Hardware, versione, conteggio modelli |
| `/api/models` | GET | Lista modelli installati |
| `/api/models/info` | POST | Dettagli singolo modello |
| `/api/models/rename` | POST | Rinomina display name |
| `/api/models/remove` | POST | Elimina modello |
| `/api/pull` | POST | Scarica modello (SSE progress) |
| `/api/pull/cancel` | POST | Annulla download |
| `/api/generate` | POST | Esegui modello (SSE progress + risultato) |
| `/api/agents/catalog` | GET | Lista agenti AI disponibili |
| `/api/agents/install` | POST | Installa agente via npm |
| `/api/agents/start` | POST | Avvia agente |
| `/api/agents/stop` | POST | Ferma agente |
| `/api/agents/uninstall` | POST | Rimuovi agente |
| `/api/agents/proxy` | POST | Proxy per chiamate API cloud (SSE passthrough) |
| `/api/ollama/models` | GET | Lista modelli Ollama locale |

---

## Dove vengono salvati i dati

| Dato | Percorso |
|------|----------|
| Modelli scaricati | `C:\Users\<utente>\.pullai\models\` |
| Config Vortelio | `C:\Users\<utente>\.pullai\` |
| Profilo browser (Edge/Chrome) | `C:\Users\<utente>\.vortelio\app-profile\` |
| Account utenti (localStorage) | Edge/Chrome profilo app — `vortelio_users` |
| Sessione corrente (localStorage) | Edge/Chrome profilo app — `vortelio_session` |
| Chat salvate (localStorage) | Edge/Chrome profilo app — `pullai_chats` |
| Chat cloud salvate (localStorage) | Edge/Chrome profilo app — `vortelio_cloud_chats` |
| Config provider cloud (localStorage) | Edge/Chrome profilo app — `vortelio_cloud_<id>` |

---

## Dipendenze Python necessarie

I modelli non-LLM richiedono Python 3.10-3.12 (Kokoro, Bark) o Python 3.14 (faster-whisper, kokoro-onnx).

```bash
# Trascrizione audio (Whisper) — compatibile Python 3.14
pip install faster-whisper

# TTS Kokoro — compatibile Python 3.14
pip install kokoro-onnx soundfile huggingface_hub

# TTS Kokoro — solo Python 3.10-3.12, richiede espeak-ng installato
pip install kokoro soundfile

# Immagini (Stable Diffusion, FLUX)
pip install diffusers transformers accelerate torch torchvision Pillow

# Video (WAN 2.1)
pip install diffusers transformers accelerate torch

# 3D Shap-E (testo → mesh)
pip install git+https://github.com/openai/shap-e.git

# 3D TripoSR (immagine → mesh, richiede git)
pip install git+https://github.com/VAST-AI-Research/TripoSR.git
```

---

## Aggiornare la versione

Prima di ogni build, aggiornare:
- `pullai/internal/version/version.go` → `const Version = "X.Y.Z"`
- `pullai-installer/pullai-installer.nsi` → `!define PRODUCT_VERSION "X.Y.Z"`

---

## Provider Cloud supportati

| Provider | Protocollo | Auth | Note |
|----------|-----------|------|------|
| Ollama Cloud | OpenAI-compat `/v1/chat/completions` | Bearer | ollama.com/settings/keys |
| Ollama locale | Ollama nativo `/api/chat` | nessuna | localhost:11434 |
| OpenAI | OpenAI `/v1/chat/completions` | Bearer sk-... | |
| Anthropic | `/v1/messages` + `x-api-key` + `anthropic-version` | x-api-key | SSE diverso da OpenAI |
| Google Gemini | `/v1beta/models/{model}:streamGenerateContent?alt=sse` | x-goog-api-key | Fetch diretto, no proxy |
| Groq | OpenAI-compat `/openai/v1/chat/completions` | Bearer gsk_... | |
| Mistral | OpenAI-compat `/v1/chat/completions` | Bearer | |
| OpenRouter | OpenAI-compat `/api/v1/chat/completions` | Bearer sk-or-... | |

---

## Regola di documentazione

> **Ad ogni modifica importante, aggiornare questo file README.md.**
>
> Ogni voce nel changelog deve includere: versione, data, cosa è stato cambiato e perché.

---

## Changelog

### v0.3.47 — 2026-04-10
- **[FIX] Codice su riga singola**: i nodi di testo AI erano `<span>` che collassano i whitespace; cambiati in `<div>` per rispettare `white-space:pre` dei blocchi codice.
- **[FIX] Menu account non appariva**: l'HTML del dropdown `#acct-menu` non veniva inserito nel DOM.
- **[ADD] README.md**: documentazione completa con struttura file, build instructions, API routes, percorsi dati, dipendenze Python.

### v0.3.46 — 2026-04-10
- **[ADD] Menu account dropdown**: clic sul pulsante Account apre dropdown con Impostazioni, Cambia password, Disconnetti.
- **[FIX] Kokoro TTS Python 3.14**: aggiunto `kokoro-onnx` come backend primario (nessun `espeak-ng`), fallback a `kokoro` per Python ≤3.12.
- **[FIX] Codice block CSS**: sfondo scuro, full-width, copia in alto a destra.

### v0.3.45 — 2026-04-09
- **[FIX] Topbar layout**: Impostazioni + Salva a sinistra, Account a destra — non si sovrappongono più.
- **[FIX] Ollama Cloud latency**: fetch diretto a `ollama.com` senza proxy Go (elimina buffering).
- **[FIX] Audio SyntaxError**: `buildWhisperScript` riscritta con lista di stringhe invece di `fmt.Sprintf` per evitare conflitti con `%`.

### v0.3.44 — 2026-04-09
- **[FIX] GUI disconnessa — causa radice**: `init()` era chiamato prima che `#auth-overlay` fosse nel DOM → `authApplySession()` crashava silenziosamente → `startPolling()` non veniva mai chiamato → pallino sempre rosso. Risolto spostando l'HTML auth prima di `<script>` e wrappando `init()` in `DOMContentLoaded`.
- **[FIX] Hardware detection cachata**: `nvidia-smi` veniva eseguito ad ogni `/api/status` (2-5 secondi). Ora cachato con `sync.Once`.
- **[FIX] Server lookup**: `service_windows.go` cercava `pullai-server.exe` invece di `vortelio-server.exe`.

### v0.3.43 — 2026-04-09
- **[FIX] GUI pallino rosso**: timeout `/api/status` aumentato a 8s, retry ogni 2s finché non connesso.

### v0.3.42 — 2026-04-08
- **[FIX] Hardware detection**: `DetectHardware()` cachata, warmup in background all'avvio server.
- **[FIX] Profile directory**: Edge/Chrome usa `~/.vortelio/app-profile` invece di `~/.pullai/app-profile`.

### v0.3.41 — 2026-04-07
- **[FIX] Server exe name**: cerca `vortelio-server.exe` → `pullai-server.exe` → self (compatibilità).
- **[FIX] Branding CLI**: rimossi ultimi riferimenti a `pullai` nei messaggi di errore.

### v0.3.40 — 2026-04-06
- **[FIX] Ollama Cloud 401**: endpoint corretto da `/api/chat` a `/v1/chat/completions` (OpenAI-compat).
- **[FIX] Modelli Ollama cloud**: rimosso `qwen3-vl-235b-cloud` (non esistente), aggiornata lista.
- **[FIX] Traduzione IT↔EN**: aggiunte chiavi mancanti per pannelli cloud, agenti, auth.
- **[FIX] GUI retry**: polling ogni 2s finché non connesso invece di aspettare 15s.

### v0.3.39 — 2026-04-05
- **[FIX] Audio Python 3.14**: `faster-whisper` come backend primario (no ctypes crash).
- **[ADD] Gemini API**: aggiornato a `streamGenerateContent?alt=sse`, header `x-goog-api-key`.
- **[FIX] Installer**: `/oname=vortelio.exe` — il binario ora viene rinominato correttamente.

### v0.3.38 — 2026-03-30
- **[ADD] Rinomina in Vortelio**: da Votelio a Vortelio in tutto il codebase.
- **[ADD] Cloud API panel**: pannello dedicato con 8 provider (Ollama, OpenAI, Anthropic, Gemini, Groq, Mistral, OpenRouter).
- **[ADD] Agenti AI**: OpenClaw e Open Code con install/start/stop/web UI.
- **[ADD] Auth system**: login email+password (SHA-256+salt), Google OAuth, sessione localStorage.
- **[ADD] Code block rendering**: blocchi codice durante streaming con tema scuro e pulsante copia.
- **[FIX] NSIS installer**: `/oname=` per rinominare exe durante installazione.

### v0.3.49 — 2026-04-11
- **[FIX] Auth overlay non visibile**: mancava il CSS `#auth-overlay { position:fixed; inset:0; z-index:600 }` — l'overlay era nel DOM ma senza posizionamento fixed si nascondeva sotto la UI. Il pulsante Account sembrava non fare nulla perché l'overlay si apriva ma era invisibile.
- **[FIX] authShowOverlay()**: ora resetta il form (mostra login, nasconde registrazione, pulisce errori) e fa focus sull'email.
- **[FIX] authApplySession()**: mostra correttamente l'overlay dopo logout, resetta i campi.
