"""Train a learning bot with PPO against the Go game environment.

    uv run python train.py --game holdem --run holdem-1 --minutes 60
    uv run python train.py --game canasta --run canasta-1 --minutes 600 --set env.envs=12

Writes runs/<name>/: TensorBoard logs (tb/), metrics.jsonl (one line per
update), ckpt/<update>.bin snapshots (also the league's checkpoint pool),
state.pt (model and optimiser, for resuming with --resume) and final.bin at
the end — the file the server loads.
"""

from __future__ import annotations

import argparse
import json
import random
import selectors
import signal
import sys
import time
from collections import defaultdict
from pathlib import Path

import numpy as np
import torch
import yaml

from zolik_ml import export
from zolik_ml.env import HELD_OUT_SEED, GameEnv, binary
from zolik_ml.league import League
from zolik_ml.model import from_config
from zolik_ml.ppo import PPO, PPOConfig, Step, Tracker, build_batch

ML_DIR = Path(__file__).resolve().parent


def load_config(path: Path, overrides: list[str]) -> dict:
    cfg = yaml.safe_load(path.read_text())
    for o in overrides:
        key, _, raw = o.partition("=")
        node = cfg
        parts = key.split(".")
        for p in parts[:-1]:
            node = node.setdefault(p, {})
        node[parts[-1]] = yaml.safe_load(raw)
    return cfg


class Trainer:
    def __init__(self, args, cfg: dict):
        self.args = args
        self.cfg = cfg
        self.game = cfg["game"]
        self.run_dir = ML_DIR / "runs" / args.run
        self.ckpt_dir = self.run_dir / "ckpt"
        self.ckpt_dir.mkdir(parents=True, exist_ok=True)
        (self.run_dir / "config.yaml").write_text(yaml.safe_dump(cfg, sort_keys=False))

        seed = args.seed
        random.seed(seed)
        np.random.seed(seed)
        torch.manual_seed(seed)
        self.rng = random.Random(seed)

        ec = cfg["env"]
        self.n_envs = int(ec["envs"])
        self.n_tables = int(ec["tables"])
        exe = binary("gameenv")
        lg = cfg["league"]
        self.variations = [
            "samba" if lg.get("samba") and i % 2 == 1 else cfg.get("variation", "") for i in range(self.n_envs)
        ]
        self.envs = [
            GameEnv(self.game, v, int(ec.get("budget", 50_000)), exe, ec.get("procs"), ec.get("gogc")) for v in self.variations
        ]
        state_dim, cand_dim = self.envs[0].info()
        self.selector = selectors.DefaultSelector()
        for i, env in enumerate(self.envs):
            self.selector.register(env.proc.stdout, selectors.EVENT_READ, i)

        self.model = from_config(cfg, state_dim, cand_dim, self.game)
        if args.resume:
            st = torch.load(args.resume, map_location="cpu")
            self.model.load_state_dict(st["model"])
        self.ppo = PPO(self.model, PPOConfig.from_dict(cfg["ppo"]))
        if args.resume and "opt" in st:
            self.ppo.opt.load_state_dict(st["opt"])
        self.league = League(cfg, seed)
        for p in sorted(self.ckpt_dir.glob("*.bin"), key=lambda p: int(p.stem)):
            self.league.add(p)  # a resumed run keeps its pool
        self.tracker = Tracker()

        from torch.utils.tensorboard import SummaryWriter

        self.tb = SummaryWriter(str(self.run_dir / "tb"))
        self.metrics = (self.run_dir / "metrics.jsonl").open("a")

        # Per-env state.
        self.labels: list[list[str]] = [[] for _ in self.envs]
        self.seed_base = [0] * self.n_envs
        self.prev_counts: dict = {}
        self.reset_due = [False] * self.n_envs

    # --- env management --------------------------------------------------------

    def send_reset(self, i: int) -> None:
        self.tracker.drop(lambda k: k[0] == i)
        tables = [self.league.sample(self.variations[i]) for _ in range(self.n_tables)]
        self.labels[i] = [t.label for t in tables]
        # Training seeds stay below HELD_OUT_SEED; leave headroom for the
        # matches the tables play before the next reset.
        self.seed_base[i] = self.rng.randrange(0, HELD_OUT_SEED - 200_000)
        self.prev_counts = {k: v for k, v in self.prev_counts.items() if k[0] != i}
        self.envs[i].send_reset([t.plan for t in tables], self.seed_base[i])
        self.reset_due[i] = False

    # --- the loop --------------------------------------------------------------

    def run(self) -> None:
        cfg, args = self.cfg, self.args
        tc = cfg["train"]
        rollout = int(tc["rollout"])
        total_updates = int(args.updates or tc["total_updates"])
        snapshot_every = int(tc["snapshot_every"])
        reset_every = int(tc["reset_every"])
        deadline = time.time() + args.minutes * 60 if args.minutes else None
        start = time.time()

        stop = {"now": False}
        signal.signal(signal.SIGINT, lambda *_: stop.update(now=True))

        for i in range(self.n_envs):
            self.send_reset(i)

        update = 0
        total_decisions = 0
        while not stop["now"]:
            t0 = time.time()
            ep = defaultdict(list)  # label -> returns
            counts = defaultdict(int)
            stall_why: dict[str, int] = defaultdict(int)
            decisions = 0
            env_wait = 0.0
            while decisions < rollout and not stop["now"]:
                # Serve whichever process answers first. A step waits for the
                # slowest of its tables, and a slow step on one process must not
                # hold the others idle behind it.
                tw = time.time()
                ready = self.selector.select()
                env_wait += time.time() - tw
                for key, _ in ready:
                    i = key.data
                    env = self.envs[i]
                    obs = env.recv()
                    decisions += self._absorb(i, obs, ep, counts, stall_why)
                    if self.reset_due[i]:
                        self.send_reset(i)
                        continue
                    actions, logp, values = self.ppo.act([o.obs for o in obs], [o.cands for o in obs])
                    for t, o in enumerate(obs):
                        self.tracker.decision(
                            (i, t, o.seat), Step(o.obs, o.cands, int(actions[t]), float(logp[t]), float(values[t]))
                        )
                    env.send_step(actions)
            collect_s = time.time() - t0
            total_decisions += decisions

            # Learning-rate annealing on whichever horizon comes first.
            progress = update / total_updates
            if deadline:
                progress = max(progress, (time.time() - start) / (args.minutes * 60))
            lr = self.ppo.set_lr_fraction(1.0 - min(progress, 0.99))

            t1 = time.time()
            batch = build_batch(self.tracker.harvest(), self.ppo.cfg.gamma, self.ppo.cfg.lam, self.ppo.cfg.reward_scale)
            st = self.ppo.update(batch)
            update_s = time.time() - t1
            update += 1

            if update % snapshot_every == 0:
                path = export.save(self.model, self.ckpt_dir / f"{update}.bin")
                self.league.add(path)
                torch.save({"model": self.model.state_dict(), "opt": self.ppo.opt.state_dict()}, self.run_dir / "state.pt")
            for i in range(self.n_envs):
                if update % reset_every == i % reset_every:
                    self.reset_due[i] = True

            self._log(update, total_decisions, decisions, collect_s, update_s, env_wait, len(batch), lr, st, ep, counts, stall_why, start)

            if update >= total_updates or (deadline and time.time() >= deadline):
                break

        final = export.save(self.model, self.run_dir / "final.bin")
        torch.save({"model": self.model.state_dict(), "opt": self.ppo.opt.state_dict()}, self.run_dir / "state.pt")
        print(f"wrote {final}", flush=True)

    def _absorb(self, i, obs, ep, counts, stall_why) -> int:
        """Credit events, collect episode statistics and env counters."""
        for t, o in enumerate(obs):
            label = self.labels[i][t] if t < len(self.labels[i]) else "?"
            for e in o.events:
                s = self.tracker.event((i, t, e.seat), e.reward, e.done)
                if s is not None:
                    ep[label].append(s.ret)
            prev = self.prev_counts.get((i, t), (0, 0, 0))
            d_ill, d_stall, d_match = o.illegal - prev[0], o.stalls - prev[1], o.matches - prev[2]
            counts["illegal"] += d_ill
            counts["stalls"] += d_stall
            counts["matches"] += d_match
            if d_stall and o.last_stall:
                stall_why[f"{label}: {o.last_stall}"] += d_stall
            self.prev_counts[(i, t)] = (o.illegal, o.stalls, o.matches)
            if self.seed_base[i] + (o.matches + 2) * self.n_tables >= HELD_OUT_SEED:
                self.reset_due[i] = True
        return len(obs)

    def _log(self, update, total_decisions, decisions, collect_s, update_s, env_wait, n, lr, st, ep, counts, stall_why, start):
        tb = self.tb
        dps = decisions / max(collect_s, 1e-9)
        all_ret = [r for rs in ep.values() for r in rs]
        rec = {
            "update": update,
            "elapsed": round(time.time() - start, 1),
            "decisions": total_decisions,
            "dps": round(dps, 1),
            "batch": n,
            "collect_s": round(collect_s, 2),
            "update_s": round(update_s, 2),
            "lr": lr,
            "policy_loss": st.policy_loss,
            "value_loss": st.value_loss,
            "entropy": st.entropy,
            "kl": st.approx_kl,
            "clip_frac": st.clip_frac,
            "explained_var": st.explained_var,
            "grad_norm": st.grad_norm,
            "episodes": len(all_ret),
            "ep_reward": float(np.mean(all_ret)) if all_ret else None,
            "illegal": counts["illegal"],
            "stalls": counts["stalls"],
            "matches": counts["matches"],
            "orphan_reward": self.tracker.orphan_reward,
            "by_opponent": {},
        }
        tb.add_scalar("perf/decisions_per_s", dps, update)
        rec["env_wait_s"] = round(env_wait, 2)
        tb.add_scalar("perf/env_wait_s", env_wait, update)
        tb.add_scalar("perf/update_s", update_s, update)
        tb.add_scalar("loss/policy", st.policy_loss, update)
        tb.add_scalar("loss/value", st.value_loss, update)
        tb.add_scalar("loss/entropy", st.entropy, update)
        tb.add_scalar("loss/approx_kl", st.approx_kl, update)
        tb.add_scalar("loss/clip_frac", st.clip_frac, update)
        tb.add_scalar("loss/explained_var", st.explained_var, update)
        tb.add_scalar("loss/grad_norm", st.grad_norm, update)
        tb.add_scalar("train/lr", lr, update)
        tb.add_scalar("env/illegal", counts["illegal"], update)
        tb.add_scalar("env/stalls", counts["stalls"], update)
        tb.add_scalar("env/matches", counts["matches"], update)
        if all_ret:
            tb.add_scalar("reward/episode", float(np.mean(all_ret)), update)
        for label, rs in sorted(ep.items()):
            m = float(np.mean(rs))
            w = float(np.mean([r > 0 for r in rs]))
            rec["by_opponent"][label] = {"n": len(rs), "reward": round(m, 4), "win": round(w, 3)}
            tb.add_scalar(f"reward_vs/{label}", m, update)
            tb.add_scalar(f"win_vs/{label}", w, update)
        self.metrics.write(json.dumps(rec) + "\n")
        self.metrics.flush()
        opp = " ".join(f"{k}={v['reward']:+.3f}/{v['win']:.2f}" for k, v in sorted(rec["by_opponent"].items()))
        print(
            f"[{update:4d} {rec['elapsed']:6.0f}s] dps={dps:6.0f} batch={n} "
            f"ep={rec['ep_reward'] if rec['ep_reward'] is None else round(rec['ep_reward'], 3)} "
            f"pl={st.policy_loss:+.4f} vl={st.value_loss:.4f} ent={st.entropy:.3f} kl={st.approx_kl:.4f} "
            f"ev={st.explained_var:.2f} gn={st.grad_norm:.2f} ill={counts['illegal']} stalls={counts['stalls']} | {opp}",
            flush=True,
        )
        for why, k in stall_why.items():
            print(f"    stall x{k}: {why}", flush=True)

    def close(self) -> None:
        for env in self.envs:
            env.close()
        self.tb.close()
        self.metrics.close()


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game", required=True, choices=["holdem", "canasta"])
    ap.add_argument("--config", type=Path, help="defaults to configs/<game>.yaml")
    ap.add_argument("--run", required=True, help="run name; output goes to runs/<name>/")
    ap.add_argument("--minutes", type=float, help="stop after this much wall time")
    ap.add_argument("--updates", type=int, help="stop after this many updates (default: train.total_updates)")
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--threads", type=int, default=4, help="torch CPU threads")
    ap.add_argument("--resume", type=Path, help="state.pt to start from")
    ap.add_argument("--set", action="append", default=[], metavar="KEY=VALUE", help="override a config value, e.g. env.envs=4")
    args = ap.parse_args(argv)

    cfg = load_config(args.config or ML_DIR / "configs" / f"{args.game}.yaml", args.set)
    if cfg["game"] != args.game:
        sys.exit(f"config is for {cfg['game']}, not {args.game}")
    torch.set_num_threads(args.threads)
    tr = Trainer(args, cfg)
    try:
        tr.run()
    finally:
        tr.close()


if __name__ == "__main__":
    main()
