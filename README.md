# Votelio — AI Locale, Zero Compomessi

**Votelio** è uno strumento CLI open-source che ti permette di eseguire modelli AI interamente sul tuo PC — senza cloud, senza abbonamenti, senza alcuna dipendenza da servizi esterni.

Scarica e usa modelli linguistici (LLM), generatori di immagini, audio, video e persino oggetti 3D, tutto in locale sulla tua hardware.

---

## Funzionalita Principali

### Modelli Linguistici (LLM)
Esegui conversazioni, rispondi a domande, scrivi e analizza testo — con il tuo modello preferito, interamente sulla tua macchina.

- **GGUF** con llama.cpp (veloce, efficiente, basso utilizzo RAM)
- **Safetensors** con HuggingFace Transformers + PyTorch
- Supporto nativo per modelli come Mistral 7B, Llama 3, Phi-3, Qwen, Gemma
- Streaming token in tempo reale (SSE)
- Storico conversazioni e modalita chain-of-thought ("think")

### Generazione Immagini
Crea immagini da descrizioni testuali (prompt) con modelli Stable Diffusion in vari formati.

- SD1.5, SDXL, FLUX, Illustrious, Pony Diffusion
- Backend GGUF (velocissimo) o Safetensors
- Modelli built-in: OpenJourney, DreamShaper, SDXL, FLUX Schnell

### Audio
- **Whisper** — trascrizione vocale in testo (speech-to-text)
- **Bark** e **Kokoro** — sintesi vocale (text-to-speech)

### Video
Genera video corti da prompt testuali usando modelli state-of-the-art.

- CogVideoX-5B, Wan 2.1 (1.3B / 14B), AnimateDiff v3
- HunyuanVideo, LTX-Video, Mochi (via Diffusers)

### 3D
Crea mesh e oggetti tridimensionali da prompt.

- TripoSR, Shap-E

---

## Cosa Puoi Fare

- **Scaricare modelli** da HuggingFace Hub con un solo comando
- **Eseguire modelli** in locale via CLI o API REST
- **Avviare un server API** locale con interfaccia web integrata
- **Gestire modelli** (lista, rimuovere, info, quantizzare)
- **Proxy verso provider AI remoti** (Anthropic, OpenAI-compatible) per unificare l'accesso
- **Plugin agent** — estendi le funzionalita con agent personalizzati (Node.js)
- **Multipiattaforma** — Windows, macOS (Apple Silicon), Linux, GPU NVIDIA/AMD/CPU

---

## Cosa NON Puoi Fare (ancora)

- **Addestrare o fine-tunare** modelli — Votelio è solo inferenza
- **Eseguire piu modelli in pipeline orchestrate** — un modello alla volta
- **Rilevare automaticamente la GPU** in tutti gli ambienti (VM/container potrebbero richiedere configurazione manuale)
- **Funzionare senza Python** per contenuti media (immagini/audio/video/3D) e modelli safetensors
- **Quantizzare modelli autonomamente** — richiede strumenti esterni

---

## Requisiti di Sistema

| Componente | Minimo |
|---|---|
| OS | Windows 10+, macOS 12+, Linux |
| RAM | 8 GB (16 GB consigliati per LLM) |
| GPU | NVIDIA CUDA, Apple Metal, AMD ROCm, o CPU-only |
| Python | 3.10+ (per modelli media/safetensors) |
| Go | 1.22+ (per compilazione da sorgente) |

---

## Installazione Rapida

Scarica il programma di installazione per Windows:

```
Votelio-Setup-0.3.38.exe
```

Oppure installa da sorgente:

```bash
git clone https://github.com/pullai/pullai
cd pullai
go build -o votelio ./pullai/cmd/pullai
./votelio setup
```

---

## Guida Introduttiva

### 1. Configura le dipendenze

```bash
votelio setup
```

### 2. Scarica un modello

```bash
# LLM (ex. Mistral 7B)
votelio pull mistral:7b

# Immagini (ex. SDXL)
votelio pull sdxl

# Whisper (trascrizione audio)
votelio pull whisper:base
```

### 3. Esegui un modello

```bash
# Chat interattiva
votelio run mistral:7b

# Prompt singolo
votelio run mistral:7b "Spiegami cosa è il machine learning in parole semplici"
```

### 4. Avvia il server API (con interfaccia web)

```bash
votelio serve --port 11500
```

Apri il browser su `http://localhost:11500` per accedere all'interfaccia grafica.

---

## Comandi CLI

| Comando | Descrizione |
|---|---|
| `votelio pull <modello>` | Scarica un modello da HuggingFace |
| `votelio run <modello> [prompt]` | Esegui un modello (interattivo o singolo prompt) |
| `votelio list` | Elenca tutti i modelli scaricati |
| `votelio rm <modello>` | Rimuovi un modello |
| `votelio info <modello>` | Mostra dettagli di un modello |
| `votelio serve [--port N]` | Avvia server API + UI web |
| `votelio gui` | Apri la GUI desktop |
| `votelio setup` | Installa dipendenze runtime |
| `votelio cleanup` | Analizza/rimuovi file cache orfani |
| `votelio quantize <modello>` | Quantizza un modello |

---

## API REST

Il server locale espone API REST per l'integrazione con altri strumenti:

```
GET  /api/models              — Lista modelli scaricati
POST /api/generate            — Esegui inferenza (streaming SSE per LLM)
POST /api/pull                — Scarica un modello (con eventi progresso)
GET  /api/agents/catalog      — Catalogo plugin agent disponibili
POST /api/agents/install     — Installa un plugin agent
POST /api/agents/start        — Avvia un agent
```

Server predefinito: `http://localhost:11500`

---

## Stack Tecnologico

| Layer | Tecnologia |
|---|---|
| CLI / Core | Go 1.22 |
| Python Runner | Python 3 (subprocess) |
| LLM GGUF | llama.cpp (`llama-server`) |
| LLM Safetensors | Transformers + PyTorch |
| Immagini | stable-diffusion-cpp-python / Diffusers |
| Audio | Whisper, Bark, Kokoro |
| Video | Diffusers (CogVideoX, Wan2.1, AnimateDiff, HunyuanVideo) |
| 3D | TripoSR, Shap-E |
| GPU | CUDA, Metal, ROCm, CPU |
| API Server | Go `net/http` + UI HTML integrata |

---

## Struttura Dati

I modelli vengono salvati in `~/.pullai/models/`:

```
~/.pullai/
├── models/
│   ├── llm/<nome>/<tag>/
│   ├── image/<nome>/<tag>/
│   ├── audio/<nome>/<tag>/
│   ├── video/<nome>/<tag>/
│   └── 3d/<nome>/<tag>/
└── bin/          (binari llama.cpp)
```

---

## Limitazioni Note

1. **llama.cpp necessario** per modelli LLM GGUF — `setup` lo installa automaticamente, ma su alcuni sistemi potrebbe servire configurazione manuale.
2. **Python obbligatorio** per generazione media e modelli safetensors.
3. **Rilevamento GPU** automatico (NVIDIA/AMD/Apple), ma potrebbe fallire in VM/container.
4. **Nessun training** — solo inferenza.
5. **Inferenza singola** — un modello alla volta, nessuna pipeline multi-modello.

---

## Licenza

Progetto open-source. Consulta il repository per i dettagli sulla licenza.

---

**Votelio** — L'AI che gira sul tuo PC, non sui nostri server.
