"""Value-head calibration: how well a run's value estimates predict what a seat
goes on to collect, for the critic as trained and for the plain head exported.

    uv run python valuecal.py --run runs/canasta-v4 --seats 4 --tables 32 --decisions 30000

The run's model (state.pt, so the critic too) plays every seat at temperature
0 on held-out seeds (from 1,000,000). At each real decision the critic's and
the plain head's estimates are set beside the reward the seat collected from
there to the end of its episode (the deal), in the training reward's unit:
the run's own reward (with its match reward, if any) and, separately, the
deal reward alone. Prints the correlation, R², the slope of actual on
predicted (1 is calibrated), and the means. The Go counterpart for an exported
file alone is cmd/valuecal on claude/search-on-policy.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from pathlib import Path

import numpy as np
import torch
import yaml

from train import load_match_reward
from zolik_ml.env import HELD_OUT_SEED, GameEnv
from zolik_ml.model import from_config
from zolik_ml.ppo import PPO, PPOConfig


def stats(pred: np.ndarray, actual: np.ndarray) -> str:
    r = float(np.corrcoef(pred, actual)[0, 1])
    slope = float(np.cov(pred, actual)[0, 1] / pred.var(ddof=1))
    return f"corr {r:+.3f} (R² {r * r:.3f}), slope {slope:.3f}, mean pred {pred.mean():+.3f} actual {actual.mean():+.3f}"


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--run", type=Path, required=True)
    ap.add_argument("--seats", type=int, default=4)
    ap.add_argument("--tables", type=int, default=32)
    ap.add_argument("--decisions", type=int, default=30000)
    ap.add_argument("--threads", type=int, default=4)
    args = ap.parse_args(argv)
    torch.set_num_threads(args.threads)

    cfg = yaml.safe_load((args.run / "config.yaml").read_text())
    game = cfg["game"]
    privileged = bool(cfg["model"].get("priv"))
    env = GameEnv(game, cfg.get("variation", ""), privileged=privileged, match_reward=load_match_reward(cfg))
    info = env.info_full()
    model = from_config(cfg, info["stateDim"], info["candDim"], game, info["privDim"])
    model.load_state_dict(torch.load(args.run / "state.pt", map_location="cpu")["model"])
    model.eval()
    ppo = PPO(model, PPOConfig())

    plans = [["learner"] * args.seats for _ in range(args.tables)]
    obs = env.reset(plans, HELD_OUT_SEED)
    open_: dict = defaultdict(list)  # (table, seat) -> [[critic, plain, acc, acc_base]]
    done = []
    n = 0
    while len(done) < args.decisions:
        for t, o in enumerate(obs):
            for e in o.events:
                base = e.reward if e.base is None else e.base
                for p in open_[(t, e.seat)]:
                    p[2] += e.reward
                    p[3] += base
                if e.done:
                    done += open_.pop((t, e.seat), [])
        privs = [o.priv for o in obs] if privileged else None
        a, _, v, vp = ppo.act([o.obs for o in obs], [o.cands for o in obs], greedy=True, priv=privs)
        for t, o in enumerate(obs):
            open_[(t, o.seat)].append([float(v[t]), float(vp[t]), 0.0, 0.0])
        obs = env.step(a)
        n += 1
    env.close()
    d = np.asarray(done)
    print(f"{args.run.name}: {len(d)} decisions, {args.seats} seats, self-play at temperature 0, held-out seeds")
    label = "critic" if privileged else "value (no critic)"
    print(f"  {label:<18} vs training reward: {stats(d[:, 0], d[:, 2])}")
    print(f"  {'plain (exported)':<18} vs training reward: {stats(d[:, 1], d[:, 2])}")
    if load_match_reward(cfg):
        print(f"  {label:<18} vs deal reward:     {stats(d[:, 0], d[:, 3])}")
        print(f"  {'plain (exported)':<18} vs deal reward:     {stats(d[:, 1], d[:, 3])}")


if __name__ == "__main__":
    main()
