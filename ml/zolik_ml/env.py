"""Client for the Go game environment (server/cmd/gameenv).

The environment speaks JSON lines on stdin/stdout (server/internal/learn/env.go):

    {"op":"reset","game":..,"variation":..,"seed":..,"budget":..,"tables":[{"plan":[...]}]}
    {"op":"step","choices":[...]}
    {"op":"info","game":..}

and every reset/step reply is ``{"tables":[{seat, obs, cands, events, matches,
illegal, stalls, lastStall}]}``. Every table always comes back with a pending
learner decision; ``events`` credit rewards to learner seats since the last reply.

This module never interprets a game: it moves numbers between the process and
numpy, and pads the variable-length candidate lists into a dense batch with a
mask.
"""

from __future__ import annotations

import os
import subprocess
import threading
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np
import orjson

ML_DIR = Path(__file__).resolve().parent.parent
REPO_DIR = ML_DIR.parent
SERVER_DIR = REPO_DIR / "server"
BIN_DIR = ML_DIR / ".bin"

# Training seeds stay below this; eval.py plays from it upwards.
HELD_OUT_SEED = 1_000_000

_build_lock = threading.Lock()
_built: set[str] = set()


def binary(name: str) -> Path:
    """Build server/cmd/<name> into ml/.bin once per process and return its path.

    ``go build`` is incremental, so this is cheap when nothing changed, and it
    means a trainer never runs a stale environment after a Go edit.
    """
    path = BIN_DIR / name
    with _build_lock:
        if name in _built and path.exists():
            return path
        BIN_DIR.mkdir(parents=True, exist_ok=True)
        subprocess.run(
            ["go", "build", "-o", str(path), f"./cmd/{name}"],
            cwd=SERVER_DIR,
            check=True,
        )
        _built.add(name)
    return path


def _go_env(procs: int | None, gogc: int | None) -> dict | None:
    """The process environment for one gameenv.

    procs caps the Go scheduler's threads, so several processes share the
    machine's cores instead of each assuming it has them all. gogc trades
    memory for speed: the engines are JSON in and out, and at gameenv's own
    default (400) a Hold'em table with several bots still spends a quarter of
    its time growing and returning heap; 1000 is about 35% faster for a few
    hundred MB per process.
    """
    if not procs and not gogc:
        return None
    env = dict(os.environ)
    if procs:
        env["GOMAXPROCS"] = str(procs)
    if gogc:
        env["GOGC"] = str(gogc)
    return env


class EnvError(RuntimeError):
    pass


@dataclass
class Event:
    seat: str
    reward: float
    done: bool
    base: float | None = None  # the game's own reward, when a match reward replaced it


@dataclass
class TableObs:
    seat: str
    obs: np.ndarray  # [StateDim] float32
    cands: np.ndarray  # [n, CandDim] float32
    events: list[Event] = field(default_factory=list)
    matches: int = 0
    illegal: int = 0
    stalls: int = 0
    last_stall: str = ""
    # The privileged observation (what the seat cannot see), only when the
    # env was asked for it: the critic's input, never the policy's.
    priv: np.ndarray | None = None


def _parse_table(t: dict) -> TableObs:
    priv = t.get("priv")
    return TableObs(
        seat=t["seat"],
        obs=np.asarray(t["obs"], dtype=np.float32),
        cands=np.asarray(t["cands"], dtype=np.float32),
        events=[
            Event(e["seat"], float(e.get("reward", 0.0)), bool(e.get("done", False)), e.get("base"))
            for e in t.get("events") or []
        ],
        matches=t.get("matches", 0),
        illegal=t.get("illegal", 0),
        stalls=t.get("stalls", 0),
        last_stall=t.get("lastStall", ""),
        priv=np.asarray(priv, dtype=np.float32) if priv is not None else None,
    )


class GameEnv:
    """One gameenv process: a batch of tables, reset and stepped together.

    ``send_*``/``recv`` are split so a caller can keep several processes busy at
    once: send to all, then receive from each in turn.
    """

    def __init__(
        self,
        game: str,
        variation: str = "",
        budget: int = 50_000,
        exe: Path | None = None,
        procs: int | None = None,
        gogc: int | None = None,
        privileged: bool = False,
        match_reward: dict | None = None,
    ):
        """privileged asks for every decision's privileged observation (the
        critic's input); match_reward ({alpha, k, model}) mixes each deal's
        reward with the change in the modelled chance of winning the match.
        Both are training-only and off by default (learn.EnvOptions)."""
        self.game = game
        self.variation = variation
        self.budget = budget
        self.privileged = privileged
        self.match_reward = match_reward
        exe = exe or binary("gameenv")
        self.proc = subprocess.Popen(
            [str(exe)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            cwd=SERVER_DIR,
            env=_go_env(procs, gogc),
        )
        self.n_tables = 0
        self._pending = False

    # --- raw protocol -----------------------------------------------------------

    def _send(self, req: dict) -> None:
        if self._pending:
            raise EnvError("request already in flight")
        assert self.proc.stdin is not None
        self.proc.stdin.write(orjson.dumps(req) + b"\n")
        self.proc.stdin.flush()
        self._pending = True

    def _recv_raw(self) -> dict:
        assert self.proc.stdout is not None
        line = self.proc.stdout.readline()
        self._pending = False
        if not line:
            raise EnvError(f"gameenv exited (code {self.proc.poll()})")
        rep = orjson.loads(line)
        if rep.get("error"):
            raise EnvError(rep["error"])
        return rep

    def info(self) -> tuple[int, int]:
        rep = self.info_full()
        return rep["stateDim"], rep["candDim"]

    def info_full(self) -> dict:
        """stateDim, candDim and privDim (0 for a game without one)."""
        self._send({"op": "info", "game": self.game})
        rep = self._recv_raw()
        rep.setdefault("privDim", 0)
        return rep

    def send_reset(self, plans: list[list[str]], seed: int) -> None:
        self.n_tables = len(plans)
        req = {
            "op": "reset",
            "game": self.game,
            "variation": self.variation,
            "seed": int(seed),
            "budget": self.budget,
            "tables": [{"plan": p} for p in plans],
        }
        if self.privileged:
            req["privileged"] = True
        if self.match_reward:
            req["matchReward"] = self.match_reward
        self._send(req)

    def send_step(self, choices) -> None:
        self._send({"op": "step", "choices": [int(c) for c in choices]})

    def recv(self) -> list[TableObs]:
        return [_parse_table(t) for t in self._recv_raw()["tables"]]

    def reset(self, plans: list[list[str]], seed: int) -> list[TableObs]:
        self.send_reset(plans, seed)
        return self.recv()

    def step(self, choices) -> list[TableObs]:
        self.send_step(choices)
        return self.recv()

    def close(self) -> None:
        if self.proc.poll() is None:
            if self._pending:
                # Read the reply in flight: a process blocked writing a reply
                # larger than the pipe buffer would never see its stdin close.
                try:
                    assert self.proc.stdout is not None
                    self.proc.stdout.readline()
                except OSError:
                    pass
                self._pending = False
            try:
                assert self.proc.stdin is not None
                self.proc.stdin.close()
            except OSError:
                pass
            try:
                self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()


class VecEnv:
    """Several gameenv processes, each with its own tables and seeds.

    The processes run in parallel: ``step`` sends every process its choices
    before reading any reply. The trainer uses the finer-grained
    ``envs[i].send_step``/``recv`` to pipeline instead, so no process waits on
    Python while another's reply is being scored.
    """

    def __init__(
        self,
        n: int,
        game: str,
        variation: str = "",
        budget: int = 50_000,
        procs: int | None = None,
        gogc: int | None = None,
    ):
        exe = binary("gameenv")
        self.envs = [GameEnv(game, variation, budget, exe, procs, gogc) for _ in range(n)]

    def __len__(self) -> int:
        return len(self.envs)

    def reset(self, plans: list[list[list[str]]], seeds: list[int]) -> list[list[TableObs]]:
        for env, p, s in zip(self.envs, plans, seeds):
            env.send_reset(p, s)
        return [env.recv() for env in self.envs]

    def step(self, choices: list) -> list[list[TableObs]]:
        for env, c in zip(self.envs, choices):
            env.send_step(c)
        return [env.recv() for env in self.envs]

    def close(self) -> None:
        for env in self.envs:
            env.close()

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()


def pad_batch(obs: list[np.ndarray], cands: list[np.ndarray], cand_dim: int | None = None):
    """Stack observations and pad candidate lists to the longest one.

    Returns ``(obs [B,S], cands [B,K,C], mask [B,K] bool)`` as numpy arrays.
    """
    b = len(obs)
    k = max(len(c) for c in cands)
    c_dim = cand_dim if cand_dim is not None else cands[0].shape[1]
    out = np.zeros((b, k, c_dim), dtype=np.float32)
    mask = np.zeros((b, k), dtype=bool)
    for i, c in enumerate(cands):
        n = len(c)
        out[i, :n] = c
        mask[i, :n] = True
    return np.stack(obs).astype(np.float32, copy=False), out, mask
