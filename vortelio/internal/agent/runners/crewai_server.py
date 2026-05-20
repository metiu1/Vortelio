#!/usr/bin/env python3
"""Vortelio CrewAI Studio Bridge — full crewAI Studio API, self-contained."""

from __future__ import annotations

import argparse
import asyncio
import importlib
import inspect
import json
import os
import pkgutil
import queue
import sys
import threading
import uuid
from collections.abc import AsyncGenerator
from datetime import datetime
from pathlib import Path
from typing import Any, Literal

parser = argparse.ArgumentParser()
parser.add_argument("--port", type=int, default=8500)
parser.add_argument("--vortelio-url", default="http://localhost:11500")
parser.add_argument("--home", default=os.path.join(os.path.expanduser("~"), ".vortelio"))
args = parser.parse_args()

# ── Deps ──────────────────────────────────────────────────────────────────────

try:
    from fastapi import APIRouter, FastAPI, HTTPException, Request
    from fastapi.middleware.cors import CORSMiddleware
    from fastapi.responses import StreamingResponse
    import uvicorn
except ImportError:
    print("ERROR: pip install fastapi uvicorn", flush=True)
    sys.exit(1)

try:
    from sse_starlette.sse import EventSourceResponse
except ImportError:
    print("ERROR: pip install sse-starlette", flush=True)
    sys.exit(1)

try:
    import tomli_w
except ImportError:
    print("ERROR: pip install tomli-w", flush=True)
    sys.exit(1)

if sys.version_info >= (3, 11):
    import tomllib
else:
    try:
        import tomllib  # type: ignore[no-redef]
    except ImportError:
        try:
            import tomli as tomllib  # type: ignore[no-redef]
        except ImportError:
            print("ERROR: pip install tomli  (Python < 3.11)", flush=True)
            sys.exit(1)

try:
    from pydantic import BaseModel, Field
except ImportError:
    print("ERROR: pip install pydantic", flush=True)
    sys.exit(1)

try:
    from crewai import Agent, Crew, Process, Task
    from crewai.llm import LLM
    _CREWAI_OK = True
except ImportError:
    print("ERROR: pip install crewai", flush=True)
    sys.exit(1)

# ── Config path ───────────────────────────────────────────────────────────────

HOME_DIR = Path(args.home)
HOME_DIR.mkdir(parents=True, exist_ok=True)
CONFIG_PATH = HOME_DIR / "studio.toml"

# ── Pydantic models ───────────────────────────────────────────────────────────

PROVIDER_TYPES = Literal[
    "openai", "ollama", "anthropic", "google", "azure", "bedrock",
    "groq", "mistral", "cohere", "deepseek", "litellm", "other",
]

PROVIDER_DEFAULTS: dict[str, dict] = {
    "openai":    {"base_url": "https://api.openai.com/v1",       "models": ["gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"]},
    "ollama":    {"base_url": "http://localhost:11434",            "models": ["llama3.2", "mistral", "codellama", "phi3"]},
    "anthropic": {"base_url": "",                                  "models": ["claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"]},
    "google":    {"base_url": "",                                  "models": ["gemini/gemini-1.5-pro", "gemini/gemini-2.0-flash"]},
    "azure":     {"base_url": "",                                  "models": ["azure/gpt-4o", "azure/gpt-4o-mini"]},
    "groq":      {"base_url": "https://api.groq.com/openai/v1",   "models": ["groq/llama-3.1-70b-versatile", "groq/mixtral-8x7b-32768"]},
    "mistral":   {"base_url": "https://api.mistral.ai/v1",        "models": ["mistral/mistral-large-latest", "mistral/mistral-small-latest"]},
    "deepseek":  {"base_url": "https://api.deepseek.com/v1",      "models": ["deepseek/deepseek-chat", "deepseek/deepseek-coder"]},
}

_PROVIDER_ENV_MAP: dict[str, tuple[str, str]] = {
    "openai":    ("OPENAI_API_KEY",    "OPENAI_BASE_URL"),
    "anthropic": ("ANTHROPIC_API_KEY", ""),
    "ollama":    ("OLLAMA_API_KEY",    "OLLAMA_HOST"),
    "google":    ("GOOGLE_API_KEY",    ""),
    "azure":     ("AZURE_API_KEY",     "AZURE_API_BASE"),
    "groq":      ("GROQ_API_KEY",      ""),
    "mistral":   ("MISTRAL_API_KEY",   ""),
    "deepseek":  ("DEEPSEEK_API_KEY",  "DEEPSEEK_BASE_URL"),
}

_PROVIDER_LLM_PREFIX: dict[str, str] = {
    "ollama":   "ollama/",
    "google":   "gemini/",
    "groq":     "groq/",
    "mistral":  "mistral/",
    "deepseek": "deepseek/",
    "azure":    "azure/",
}


class ProviderConfig(BaseModel):
    name: str
    type: str = "other"
    api_key: str = ""
    base_url: str = ""
    models: list[str] = Field(default_factory=list)
    env_var: str = ""


class AgentConfig(BaseModel):
    name: str
    role: str
    goal: str
    backstory: str
    llm: str = "gpt-4o"
    cache: bool = True
    verbose: bool = False
    max_iter: int = 25
    max_rpm: int | None = None
    allow_delegation: bool = False
    tools: list[str] = Field(default_factory=list)
    max_tokens: int | None = None
    function_calling_llm: str | None = None
    use_system_prompt: bool = True
    reasoning: bool = False
    max_reasoning_attempts: int | None = None
    allow_code_execution: bool = False
    code_execution_mode: Literal["safe", "unsafe"] = "safe"
    planning: bool = False
    guardrail: str | None = None
    guardrail_max_retries: int = 3
    knowledge_sources: list[str] = Field(default_factory=list)


class TaskConfig(BaseModel):
    name: str
    description: str
    expected_output: str
    agent: str
    context: list[str] = Field(default_factory=list)
    tools: list[str] = Field(default_factory=list)
    async_execution: bool = False
    human_input: bool = False
    markdown: bool = False
    output_file: str | None = None
    guardrail: str | None = None
    guardrail_max_retries: int = 3


class CrewConfig(BaseModel):
    name: str
    process: Literal["sequential", "hierarchical"] = "sequential"
    verbose: bool = False
    memory: bool = False
    stream: bool = False
    cache: bool = True
    max_rpm: int | None = None
    agents: list[AgentConfig] = Field(default_factory=list)
    tasks: list[TaskConfig] = Field(default_factory=list)
    manager_llm: str | None = None
    manager_agent: str | None = None
    planning: bool = False
    planning_llm: str | None = None
    output_log_file: str | None = None


class StudioConfig(BaseModel):
    version: str = "1"
    crews: list[CrewConfig] = Field(default_factory=list)
    providers: list[ProviderConfig] = Field(default_factory=list)


# ── Config I/O ────────────────────────────────────────────────────────────────

def load_config() -> StudioConfig:
    if not CONFIG_PATH.exists():
        return StudioConfig()
    with CONFIG_PATH.open("rb") as f:
        data = tomllib.load(f)
    return StudioConfig.model_validate(data)


def save_config(cfg: StudioConfig) -> None:
    data = cfg.model_dump(exclude_none=True)
    with CONFIG_PATH.open("wb") as f:
        tomli_w.dump(data, f)


# ── EventBus bridge ───────────────────────────────────────────────────────────

_CORE_EVENT_TYPES: list[type] = []

try:
    from crewai.events.event_bus import crewai_event_bus
    from crewai.events.event_types import (
        AgentExecutionCompletedEvent,
        AgentExecutionErrorEvent,
        AgentExecutionStartedEvent,
        CrewKickoffCompletedEvent,
        CrewKickoffFailedEvent,
        CrewKickoffStartedEvent,
        LLMCallCompletedEvent,
        LLMCallFailedEvent,
        LLMCallStartedEvent,
        LLMStreamChunkEvent,
        TaskCompletedEvent,
        TaskFailedEvent,
        TaskStartedEvent,
        ToolUsageErrorEvent,
        ToolUsageFinishedEvent,
        ToolUsageStartedEvent,
    )
    _CORE_EVENT_TYPES = [
        CrewKickoffStartedEvent, CrewKickoffCompletedEvent, CrewKickoffFailedEvent,
        AgentExecutionStartedEvent, AgentExecutionCompletedEvent, AgentExecutionErrorEvent,
        TaskStartedEvent, TaskCompletedEvent, TaskFailedEvent,
        LLMCallStartedEvent, LLMCallCompletedEvent, LLMCallFailedEvent, LLMStreamChunkEvent,
        ToolUsageStartedEvent, ToolUsageFinishedEvent, ToolUsageErrorEvent,
    ]
    _BUS_AVAILABLE = True
except ImportError:
    _BUS_AVAILABLE = False


def _now() -> str:
    return datetime.utcnow().isoformat()


def _serialize_event(event_name: str, source: Any, event: Any) -> str:
    try:
        data = event.model_dump(mode="json", exclude_none=True)
    except Exception:
        data = {}
    source_id = getattr(source, "id", None) or getattr(source, "name", str(source))
    return json.dumps({"type": event_name, "source": str(source_id), "data": data, "ts": _now()})


class EventBusBridge:
    _instance: EventBusBridge | None = None
    _lock = threading.Lock()

    def __new__(cls) -> EventBusBridge:
        with cls._lock:
            if cls._instance is None:
                inst = super().__new__(cls)
                inst._queues: list[asyncio.Queue[str]] = []
                inst._loop: asyncio.AbstractEventLoop | None = None
                cls._instance = inst
                if _BUS_AVAILABLE:
                    inst._register_listeners()
        return cls._instance

    def set_loop(self, loop: asyncio.AbstractEventLoop) -> None:
        self._loop = loop

    def _register_listeners(self) -> None:
        for ev_type in _CORE_EVENT_TYPES:
            crewai_event_bus.on(ev_type)(self._make_relay(ev_type.__name__))

    def _make_relay(self, event_name: str):
        def handler(source: Any, event: Any) -> None:
            self._relay(event_name, source, event)
        return handler

    def _relay(self, event_name: str, source: Any, event: Any) -> None:
        if not self._queues or self._loop is None:
            return
        try:
            payload = _serialize_event(event_name, source, event)
        except Exception:
            return
        for q in list(self._queues):
            try:
                asyncio.run_coroutine_threadsafe(q.put(payload), self._loop)
            except Exception:
                pass

    async def subscribe(self) -> AsyncGenerator[str, None]:
        q: asyncio.Queue[str] = asyncio.Queue(maxsize=1000)
        self._queues.append(q)
        try:
            while True:
                yield await q.get()
        finally:
            try:
                self._queues.remove(q)
            except ValueError:
                pass

    def publish(self, event_name: str, data: dict[str, Any]) -> None:
        if not self._queues or self._loop is None:
            return
        payload = json.dumps({"type": event_name, "data": data, "ts": _now()})
        for q in list(self._queues):
            try:
                asyncio.run_coroutine_threadsafe(q.put(payload), self._loop)
            except Exception:
                pass


event_bridge = EventBusBridge()

# ── FastAPI ───────────────────────────────────────────────────────────────────

app = FastAPI(title="Vortelio CrewAI Studio", version="0.1.0", docs_url="/api/docs")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.on_event("startup")
async def _startup() -> None:
    event_bridge.set_loop(asyncio.get_event_loop())
    _ensure_vortelio_provider()


def _ensure_vortelio_provider() -> None:
    cfg = load_config()
    if any(p.name == "vortelio" for p in cfg.providers):
        return
    # Fetch available models from Vortelio's Ollama-compat /api/tags
    models: list[str] = []
    try:
        import urllib.request
        with urllib.request.urlopen(f"{args.vortelio_url}/api/tags", timeout=3) as r:
            body = json.loads(r.read())
            models = [m["name"] for m in body.get("models", [])]
    except Exception:
        pass
    if not models:
        models = ["llama3.2"]
    cfg.providers.insert(0, ProviderConfig(
        name="vortelio",
        type="openai",
        api_key="vortelio",
        base_url=f"{args.vortelio_url}/v1",
        models=models,
    ))
    save_config(cfg)


# ── Health ────────────────────────────────────────────────────────────────────

@app.get("/v1/models")
def list_models_health():
    return {"object": "list", "data": [{"id": "crewai-studio", "object": "model", "owned_by": "vortelio"}]}


# ── Config: StudioConfig ──────────────────────────────────────────────────────

@app.get("/api/config", response_model=StudioConfig)
def get_config() -> StudioConfig:
    return load_config()


@app.post("/api/config", response_model=StudioConfig)
def set_config(cfg: StudioConfig) -> StudioConfig:
    save_config(cfg)
    return cfg


# ── Config: Crews ─────────────────────────────────────────────────────────────

@app.get("/api/config/crews", response_model=list[CrewConfig])
def list_crews() -> list[CrewConfig]:
    return load_config().crews


@app.post("/api/config/crews", response_model=CrewConfig, status_code=201)
def create_crew(crew: CrewConfig) -> CrewConfig:
    cfg = load_config()
    if any(c.name == crew.name for c in cfg.crews):
        raise HTTPException(400, f"Crew '{crew.name}' already exists")
    cfg.crews.append(crew)
    save_config(cfg)
    return crew


@app.get("/api/config/crews/{name}", response_model=CrewConfig)
def get_crew(name: str) -> CrewConfig:
    for c in load_config().crews:
        if c.name == name:
            return c
    raise HTTPException(404, f"Crew '{name}' not found")


@app.put("/api/config/crews/{name}", response_model=CrewConfig)
def update_crew(name: str, crew: CrewConfig) -> CrewConfig:
    cfg = load_config()
    for i, c in enumerate(cfg.crews):
        if c.name == name:
            cfg.crews[i] = crew
            save_config(cfg)
            return crew
    raise HTTPException(404, f"Crew '{name}' not found")


@app.delete("/api/config/crews/{name}", status_code=204)
def delete_crew(name: str) -> None:
    cfg = load_config()
    before = len(cfg.crews)
    cfg.crews = [c for c in cfg.crews if c.name != name]
    if len(cfg.crews) == before:
        raise HTTPException(404, f"Crew '{name}' not found")
    save_config(cfg)


# ── Config: Agents ────────────────────────────────────────────────────────────

@app.get("/api/config/crews/{crew_name}/agents", response_model=list[AgentConfig])
def list_agents(crew_name: str) -> list[AgentConfig]:
    return get_crew(crew_name).agents


@app.post("/api/config/crews/{crew_name}/agents", response_model=AgentConfig, status_code=201)
def create_agent(crew_name: str, agent: AgentConfig) -> AgentConfig:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    if any(a.name == agent.name for a in crew.agents):
        raise HTTPException(400, f"Agent '{agent.name}' already exists")
    crew.agents.append(agent)
    save_config(cfg)
    return agent


@app.put("/api/config/crews/{crew_name}/agents/{agent_name}", response_model=AgentConfig)
def update_agent(crew_name: str, agent_name: str, agent: AgentConfig) -> AgentConfig:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    for i, a in enumerate(crew.agents):
        if a.name == agent_name:
            crew.agents[i] = agent
            save_config(cfg)
            return agent
    raise HTTPException(404, f"Agent '{agent_name}' not found")


@app.delete("/api/config/crews/{crew_name}/agents/{agent_name}", status_code=204)
def delete_agent(crew_name: str, agent_name: str) -> None:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    before = len(crew.agents)
    crew.agents = [a for a in crew.agents if a.name != agent_name]
    if len(crew.agents) == before:
        raise HTTPException(404, f"Agent '{agent_name}' not found")
    save_config(cfg)


# ── Config: Tasks ─────────────────────────────────────────────────────────────

@app.get("/api/config/crews/{crew_name}/tasks", response_model=list[TaskConfig])
def list_tasks(crew_name: str) -> list[TaskConfig]:
    return get_crew(crew_name).tasks


@app.post("/api/config/crews/{crew_name}/tasks", response_model=TaskConfig, status_code=201)
def create_task(crew_name: str, task: TaskConfig) -> TaskConfig:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    if any(t.name == task.name for t in crew.tasks):
        raise HTTPException(400, f"Task '{task.name}' already exists")
    crew.tasks.append(task)
    save_config(cfg)
    return task


@app.put("/api/config/crews/{crew_name}/tasks/{task_name}", response_model=TaskConfig)
def update_task(crew_name: str, task_name: str, task: TaskConfig) -> TaskConfig:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    for i, t in enumerate(crew.tasks):
        if t.name == task_name:
            crew.tasks[i] = task
            save_config(cfg)
            return task
    raise HTTPException(404, f"Task '{task_name}' not found")


@app.delete("/api/config/crews/{crew_name}/tasks/{task_name}", status_code=204)
def delete_task(crew_name: str, task_name: str) -> None:
    cfg = load_config()
    crew = _find_crew(cfg, crew_name)
    before = len(crew.tasks)
    crew.tasks = [t for t in crew.tasks if t.name != task_name]
    if len(crew.tasks) == before:
        raise HTTPException(404, f"Task '{task_name}' not found")
    save_config(cfg)


def _find_crew(cfg: StudioConfig, name: str) -> CrewConfig:
    for c in cfg.crews:
        if c.name == name:
            return c
    raise HTTPException(404, f"Crew '{name}' not found")


# ── Providers ─────────────────────────────────────────────────────────────────

@app.get("/api/providers", response_model=list[ProviderConfig])
def list_providers() -> list[ProviderConfig]:
    return load_config().providers


@app.post("/api/providers", response_model=ProviderConfig, status_code=201)
def create_provider(p: ProviderConfig) -> ProviderConfig:
    cfg = load_config()
    if any(x.name == p.name for x in cfg.providers):
        raise HTTPException(400, f"Provider '{p.name}' already exists")
    defaults = PROVIDER_DEFAULTS.get(p.type, {})
    if not p.base_url and defaults.get("base_url"):
        p.base_url = defaults["base_url"]
    if not p.models and defaults.get("models"):
        p.models = defaults["models"]
    cfg.providers.append(p)
    save_config(cfg)
    return p


@app.put("/api/providers/{name}", response_model=ProviderConfig)
def update_provider(name: str, p: ProviderConfig) -> ProviderConfig:
    cfg = load_config()
    for i, x in enumerate(cfg.providers):
        if x.name == name:
            cfg.providers[i] = p
            save_config(cfg)
            return p
    raise HTTPException(404, f"Provider '{name}' not found")


@app.delete("/api/providers/{name}", status_code=204)
def delete_provider(name: str) -> None:
    cfg = load_config()
    before = len(cfg.providers)
    cfg.providers = [x for x in cfg.providers if x.name != name]
    if len(cfg.providers) == before:
        raise HTTPException(404, f"Provider '{name}' not found")
    save_config(cfg)


@app.get("/api/providers/models/all")
def all_provider_models() -> list[dict]:
    cfg = load_config()
    result = []
    for p in cfg.providers:
        for m in p.models:
            result.append({"label": f"{p.name} — {m}", "value": m, "provider": p.name, "type": p.type})
    return result


@app.get("/api/providers/defaults/{provider_type}")
def provider_defaults(provider_type: str) -> dict:
    return PROVIDER_DEFAULTS.get(provider_type, {"base_url": "", "models": []})


# ── Tools ─────────────────────────────────────────────────────────────────────

class ToolInfo(BaseModel):
    name: str
    description: str
    module: str


def _discover_tools() -> list[ToolInfo]:
    tools: list[ToolInfo] = []
    try:
        import crewai_tools
        for importer, modname, ispkg in pkgutil.walk_packages(
            path=crewai_tools.__path__,
            prefix=crewai_tools.__name__ + ".",
            onerror=lambda _: None,
        ):
            if ispkg:
                continue
            try:
                mod = importlib.import_module(modname)
            except Exception:
                continue
            for name, obj in inspect.getmembers(mod, inspect.isclass):
                try:
                    from crewai.tools import BaseTool
                    if issubclass(obj, BaseTool) and obj is not BaseTool and obj.__module__ == modname:
                        desc = (inspect.getdoc(obj) or "").split("\n")[0]
                        tools.append(ToolInfo(name=name, description=desc, module=modname))
                except Exception:
                    pass
    except ImportError:
        pass
    seen: set[str] = set()
    unique: list[ToolInfo] = []
    for t in tools:
        if t.name not in seen:
            seen.add(t.name)
            unique.append(t)
    return unique


@app.get("/api/tools", response_model=list[ToolInfo])
def list_tools() -> list[ToolInfo]:
    return _discover_tools()


# ── Events SSE ────────────────────────────────────────────────────────────────

@app.get("/api/events")
async def stream_events() -> EventSourceResponse:
    async def generator() -> AsyncGenerator[dict, None]:
        async for payload in event_bridge.subscribe():
            yield {"data": payload}
    return EventSourceResponse(generator())


# ── Flow graph ────────────────────────────────────────────────────────────────

@app.get("/api/flows/{flow_name}/graph")
def get_flow_graph(flow_name: str) -> dict:
    try:
        from crewai.flow.visualization import build_flow_structure
    except ImportError as e:
        raise HTTPException(500, f"crewai flow visualization not available: {e}") from e

    flow_cls = _find_flow_class(flow_name)
    if flow_cls is None:
        raise HTTPException(404, f"Flow class '{flow_name}' not found")
    try:
        return dict(build_flow_structure(flow_cls()))
    except Exception as exc:
        raise HTTPException(500, str(exc)) from exc


def _find_flow_class(name: str) -> type | None:
    try:
        from crewai.flow.flow import Flow
    except ImportError:
        return None
    for mod in list(sys.modules.values()):
        if mod is None:
            continue
        for attr_name, obj in inspect.getmembers(mod, inspect.isclass):
            if attr_name == name and issubclass(obj, Flow) and obj is not Flow:
                return obj
    return None


# ── Run ───────────────────────────────────────────────────────────────────────

_runs: dict[str, dict[str, Any]] = {}


class RunRequest(BaseModel):
    inputs: dict[str, Any] = {}


def _apply_provider_env(providers: list[ProviderConfig]) -> None:
    for p in providers:
        env_key, env_base = _PROVIDER_ENV_MAP.get(p.type, ("", ""))
        if env_key and p.api_key:
            os.environ.setdefault(env_key, p.api_key)
            if p.env_var and p.env_var != env_key:
                os.environ.setdefault(p.env_var, p.api_key)
        if env_base and p.base_url:
            os.environ.setdefault(env_base, p.base_url)


def _resolve_llm(llm: str, providers: list[ProviderConfig]) -> str:
    if "/" in llm:
        return llm
    for p in providers:
        matched = any(
            m == llm or m.startswith(llm) or llm.startswith(m.split(":")[0])
            for m in p.models
        )
        if matched:
            prefix = _PROVIDER_LLM_PREFIX.get(p.type, "")
            if prefix:
                exact = next((m for m in p.models if m == llm or m.startswith(llm)), llm)
                return f"{prefix}{exact}"
    return llm


def _build_crew(crew_cfg: CrewConfig, providers: list[ProviderConfig]) -> Any:
    agents_map: dict[str, Any] = {}
    for a_cfg in crew_cfg.agents:
        llm_str = _resolve_llm(a_cfg.llm, providers)
        agent = Agent(
            role=a_cfg.role,
            goal=a_cfg.goal,
            backstory=a_cfg.backstory,
            llm=llm_str,
            cache=a_cfg.cache,
            verbose=a_cfg.verbose,
            max_iter=a_cfg.max_iter,
            allow_delegation=a_cfg.allow_delegation,
        )
        agents_map[a_cfg.name] = agent

    tasks: list[Any] = []
    tasks_map: dict[str, Any] = {}
    for t_cfg in crew_cfg.tasks:
        agent = agents_map.get(t_cfg.agent)
        if agent is None:
            raise ValueError(f"Task '{t_cfg.name}' references unknown agent '{t_cfg.agent}'")
        task = Task(
            description=t_cfg.description,
            expected_output=t_cfg.expected_output,
            agent=agent,
            async_execution=t_cfg.async_execution,
            human_input=t_cfg.human_input,
        )
        tasks.append(task)
        tasks_map[t_cfg.name] = task

    for t_cfg, task in zip(crew_cfg.tasks, tasks):
        if t_cfg.context:
            task.context = [tasks_map[n] for n in t_cfg.context if n in tasks_map]

    process = Process.sequential if crew_cfg.process == "sequential" else Process.hierarchical

    return Crew(
        agents=list(agents_map.values()),
        tasks=tasks,
        process=process,
        verbose=crew_cfg.verbose,
        memory=crew_cfg.memory,
    )


@app.get("/api/run/status/{run_id}")
async def run_status(run_id: str):
    run = _runs.get(run_id)
    if run is None:
        raise HTTPException(404, f"Run '{run_id}' not found")
    return run


@app.post("/api/run/{crew_name}")
async def run_crew(crew_name: str, req: RunRequest) -> EventSourceResponse:
    cfg = load_config()
    crew_cfg = next((c for c in cfg.crews if c.name == crew_name), None)
    if crew_cfg is None:
        raise HTTPException(404, f"Crew '{crew_name}' not found")

    run_id = str(uuid.uuid4())
    _runs[run_id] = {"status": "running", "crew": crew_name, "output": None, "error": None, "ts": _now()}

    out_q: asyncio.Queue[str | None] = asyncio.Queue()
    loop = asyncio.get_event_loop()

    _apply_provider_env(cfg.providers)

    def run_in_thread() -> None:
        old_out, old_err = sys.stdout, sys.stderr

        class _Writer:
            def __init__(self, typ: str):
                self._typ = typ
            def write(self, s: str) -> None:
                s = s.rstrip()
                if s:
                    payload = json.dumps({"type": "log", "msg": s})
                    asyncio.run_coroutine_threadsafe(out_q.put(payload), loop)
            def flush(self) -> None:
                pass

        sys.stdout = _Writer("log")  # type: ignore[assignment]
        sys.stderr = _Writer("log")  # type: ignore[assignment]
        try:
            crew = _build_crew(crew_cfg, cfg.providers)
            result = crew.kickoff(inputs=req.inputs)
            output = str(result)
            _runs[run_id].update({"status": "completed", "output": output})
            payload = json.dumps({"type": "RunCompleted", "run_id": run_id, "output": output, "ts": _now()})
            asyncio.run_coroutine_threadsafe(out_q.put(payload), loop)
        except Exception as exc:
            error = str(exc)
            _runs[run_id].update({"status": "failed", "error": error})
            payload = json.dumps({"type": "RunFailed", "run_id": run_id, "error": error, "ts": _now()})
            asyncio.run_coroutine_threadsafe(out_q.put(payload), loop)
        finally:
            sys.stdout = old_out
            sys.stderr = old_err
            asyncio.run_coroutine_threadsafe(out_q.put(None), loop)

    threading.Thread(target=run_in_thread, daemon=True).start()

    async def generator() -> AsyncGenerator[dict, None]:
        yield {"data": json.dumps({"type": "RunStarted", "run_id": run_id, "crew": crew_name, "ts": _now()})}
        while True:
            item = await out_q.get()
            if item is None:
                break
            yield {"data": item}

    return EventSourceResponse(generator())


# ── Legacy crew JSON endpoints (backwards compat with old UI) ──────────────────

@app.get("/api/crews")
def legacy_crew_list():
    cfg = load_config()
    return {"crews": [c.model_dump() for c in cfg.crews]}


@app.post("/api/crews/{name}/run")
async def legacy_run(name: str, request: Request) -> EventSourceResponse:
    body = {}
    try:
        body = await request.json()
    except Exception:
        pass
    return await run_crew(name, RunRequest(inputs=body.get("inputs", {})))


# ── Entry point ───────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print(f"\n  Vortelio CrewAI Studio  →  http://localhost:{args.port}/api/docs\n", flush=True)
    uvicorn.run(app, host="127.0.0.1", port=args.port, log_level="warning")
