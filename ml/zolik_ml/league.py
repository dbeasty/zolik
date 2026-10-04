"""Opponent sampling: which seats a table deals, and who plays each.

A table plan is the gameenv's list of seat specs: ``learner``, a skill
(``easy``/``medium``/``hard``), a style the game supplies (Hold'em: ``maniac``,
``rock``, ``station``, ``riverbluffer``), or ``net:<path>@<temp>`` for a frozen
checkpoint of the learner itself.

Every table is one of three kinds, drawn with the config's blend:

* ``checkpoint`` — the learner against past snapshots, weighted to recent ones;
* ``heuristic``  — the learner against one hand-written opponent type;
* ``self``       — several learner seats at one table.

A checkpoint or heuristic table seats one learner unless ``league.learners``
({learner seats: weight}) says otherwise: ``{1: .5, 2: .2, 3: .3}`` at four
seats also deals two learners with two opponents and three with one — how a
table of several copies of the model and one human looks. It is capped at one
seat short of the table, and a table with more than one learner is labelled
``<opponent>/<k>L``.

Each table carries a label naming its opponent (``station``, ``hard``,
``ckpt``, ``self``...) so the trainer can report how it does against each.

Canasta at an even number of seats from four up is partnerships of two, seat i
with seat i + n/2 (four seats: p0+p2, p1+p3). The learner's partner is
sometimes another learner seat and sometimes a heuristic bot, so it learns to
play with a partner it does not control.

A variation can have its own seat counts (``variations: {samba: {2: .., 6: ..}}``),
which is how Canasta keeps Samba behind a flag: train.py gives some env
processes the Samba variation only when ``league.samba`` is set.

A game whose rulesets are all worth training on names them in
``league.variation_mix`` ({variation: weight}); ``assign_variations`` then
splits the env processes between them in proportion (Žolíky: mostly
``zolik_classic``, some ``zolik_classic+floor35``). Every table in one process
plays that process's variation.
"""

from __future__ import annotations

import random
from dataclasses import dataclass
from pathlib import Path

LEARNER = "learner"


def assign_variations(cfg: dict, n_envs: int) -> list[str]:
    """The variation each env process plays.

    ``league.variation_mix`` splits the processes in proportion to its weights
    (largest remainder, every named variation with weight > 0 gets at least one
    process when there are enough). Without it, every process plays the config's
    ``variation``, except that ``league.samba`` gives every other one Samba.
    """
    lg = cfg.get("league", {})
    mix = {k: float(v) for k, v in (lg.get("variation_mix") or {}).items() if float(v) > 0}
    if not mix:
        return ["samba" if lg.get("samba") and i % 2 == 1 else cfg.get("variation", "") for i in range(n_envs)]
    total = sum(mix.values())
    exact = {k: n_envs * w / total for k, w in mix.items()}
    counts = {k: int(x) for k, x in exact.items()}
    if n_envs >= len(mix):
        for k in counts:
            if counts[k] == 0:
                counts[k] = 1
    while sum(counts.values()) > n_envs:
        k = max(counts, key=lambda k: counts[k] - exact[k])
        counts[k] -= 1
    while sum(counts.values()) < n_envs:
        k = max(counts, key=lambda k: exact[k] - counts[k])
        counts[k] += 1
    out: list[str] = []
    for k, n in counts.items():
        out += [k] * n
    # Interleave so a prefix of the processes still sees every variation.
    order = sorted(range(n_envs), key=lambda i: (out[:i].count(out[i]) / max(counts[out[i]], 1), i))
    return [out[i] for i in order]


@dataclass
class Table:
    plan: list[str]
    label: str


def _weighted(rng: random.Random, table: dict):
    keys = list(table)
    return rng.choices(keys, weights=[float(table[k]) for k in keys])[0]


class League:
    def __init__(self, cfg: dict, seed: int = 0):
        lg = cfg["league"]
        self.blend: dict = dict(lg.get("blend", {"checkpoint": 0.4, "heuristic": 0.4, "self": 0.2}))
        self.opponents: dict = dict(lg["opponents"])  # heuristic spec -> weight
        self.seats: dict = {int(k): v for k, v in lg["seats"].items()}  # seat count -> weight
        self.variation_seats: dict = {
            name: {int(k): v for k, v in seats.items()} for name, seats in (lg.get("variations") or {}).items()
        }
        self.partner_learner = float(lg.get("partner_learner", 0.5))
        self.partner_bot = lg.get("partner_bot", "hard")
        self.ckpt_temp = float(lg.get("checkpoint_temperature", 1.0))
        self.ckpt_decay = float(lg.get("checkpoint_decay", 0.8))
        self.pool_size = int(lg.get("pool_size", 20))
        self.partnerships = bool(lg.get("partnerships", False))
        self.learners: dict = {int(k): v for k, v in (lg.get("learners") or {1: 1.0}).items()}
        self.pool: list[Path] = []
        self.rng = random.Random(seed)

    def add(self, path: str | Path) -> None:
        self.pool.append(Path(path).resolve())
        if len(self.pool) > self.pool_size:
            self.pool.pop(0)

    def _checkpoint(self) -> str:
        n = len(self.pool)
        # The newest has weight 1, the one before decay, and so on.
        w = [self.ckpt_decay ** (n - 1 - i) for i in range(n)]
        path = self.rng.choices(self.pool, weights=w)[0]
        return f"net:{path}@{self.ckpt_temp:g}"

    def sample(self, variation: str = "") -> Table:
        rng = self.rng
        n = _weighted(rng, self.variation_seats.get(variation, self.seats))
        blend = dict(self.blend)
        if not self.pool:
            blend.pop("checkpoint", None)
        kind = _weighted(rng, blend)
        if kind == "self":
            return Table([LEARNER] * n, "self")

        if kind == "checkpoint":
            opp_label = "ckpt"
            opp = self._checkpoint
        else:
            spec = _weighted(rng, self.opponents)
            opp_label = spec
            opp = lambda: spec  # noqa: E731

        plan = [opp() for _ in range(n)]
        me = rng.randrange(n)
        plan[me] = LEARNER
        label = opp_label
        k = 1
        if self.learners != {1: 1.0} and not self.partnerships:  # the default draws nothing: old runs replay
            k = min(_weighted(rng, self.learners), n - 1)
        if k > 1:
            for seat in rng.sample([i for i in range(n) if i != me], k - 1):
                plan[seat] = LEARNER
            label += f"/{k}L"
        if self.partnerships and n >= 4 and n % 2 == 0:
            partner = (me + n // 2) % n
            if rng.random() < self.partner_learner:
                plan[partner] = LEARNER
                label += "+pl"  # partnered with another learner seat
            else:
                plan[partner] = self.partner_bot
                label += "+pb"
        return Table(plan, label)
