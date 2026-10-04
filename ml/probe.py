"""Linear probe: how much does a trained model's trunk already know about the
next player's hand? (research survey E2, step 1.)

    uv run python probe.py --model <v2 final.bin> --decisions 60000

The model plays the learner seats (temperature 0) at the lobby table on held-out
seeds, the environment sending each decision's privileged observation
(learn.Privileged) — whose first block is the next seat's actual hand. At every
decision we keep the model's trunk embedding, the public observation, and
whether the next player holds each card. One logistic regression per card is
fitted on the tables of the first three quarters and scored (ROC AUC) on the
rest, for four inputs:

    emb       the model's trunk embedding (what its heads can read)
    obs       the observation the model was trained on (its encoder's prefix)
    obs+inf   the full current observation, card-inference blocks included
    inf       the next seat's card-inference block alone

AUC is reported over every card, over the cards the deciding seat holds (the
ones it might discard — what "feeds the next player" is about), and for the
joker.
"""

from __future__ import annotations

import argparse
import time

import numpy as np
import torch

from zolik_ml import export
from zolik_ml.env import HELD_OUT_SEED, GameEnv
from zolik_ml.ppo import PPO, PPOConfig

N_SLOT = 53
JOKER = 52
OFF_INFER, INFER_DIM = 900, 87


def auc(score: np.ndarray, label: np.ndarray) -> float:
    pos = label.sum()
    neg = len(label) - pos
    if pos == 0 or neg == 0:
        return float("nan")
    order = np.argsort(score, kind="mergesort")
    ranks = np.empty(len(score))
    ranks[order] = np.arange(1, len(score) + 1)
    # average ranks over ties
    s = score[order]
    i = 0
    while i < len(s):
        j = i
        while j + 1 < len(s) and s[j + 1] == s[i]:
            j += 1
        if j > i:
            ranks[order[i : j + 1]] = (i + j + 2) / 2
        i = j + 1
    return float((ranks[label].sum() - pos * (pos + 1) / 2) / (pos * neg))


def fit_logistic(x_tr: np.ndarray, y_tr: np.ndarray, x_te: np.ndarray, l2: float = 1e-3, steps: int = 300) -> np.ndarray:
    """Independent logistic regressions, one per output column; returns test logits."""
    mu, sd = x_tr.mean(0), x_tr.std(0) + 1e-6
    xt = torch.from_numpy(((x_tr - mu) / sd).astype(np.float32))
    yt = torch.from_numpy(y_tr.astype(np.float32))
    xe = torch.from_numpy(((x_te - mu) / sd).astype(np.float32))
    lin = torch.nn.Linear(xt.shape[1], yt.shape[1])
    opt = torch.optim.LBFGS(lin.parameters(), lr=1, max_iter=steps, history_size=20, line_search_fn="strong_wolfe")

    def closure():
        opt.zero_grad()
        loss = torch.nn.functional.binary_cross_entropy_with_logits(lin(xt), yt) + l2 * lin.weight.pow(2).sum() / yt.shape[1]
        loss.backward()
        return loss

    opt.step(closure)
    with torch.no_grad():
        return lin(xe).numpy()


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--model", required=True)
    ap.add_argument("--variation", default="zolik_classic+floor35")
    ap.add_argument("--seats", type=int, default=4)
    ap.add_argument("--opponent", default="hard")
    ap.add_argument("--tables", type=int, default=32)
    ap.add_argument("--decisions", type=int, default=60000)
    args = ap.parse_args(argv)

    model = export.load(args.model)
    sd, cd = model.state_dim, model.cand_dim
    ppo = PPO(model, PPOConfig())
    env = GameEnv("zolik", args.variation, privileged=True)
    # Half the tables: the model in one seat against the bot; half: the model
    # in every seat.
    plans = []
    for t in range(args.tables):
        if t % 2 == 0:
            plans.append(["learner"] + [args.opponent] * (args.seats - 1))
        else:
            plans.append(["learner"] * args.seats)
    obs = env.reset(plans, HELD_OUT_SEED + 50_000)
    embs, pubs, hold, mine, table = [], [], [], [], []
    t0 = time.time()
    while len(embs) < args.decisions:
        o_np = np.stack([o.obs[:sd] for o in obs])
        with torch.no_grad():
            e = model.embed(torch.from_numpy(o_np)).numpy()
        for i, o in enumerate(obs):
            p = o.priv
            if p is None or p[159] == 0 and not p[:N_SLOT].any():
                continue  # between deals: nothing hidden
            embs.append(e[i])
            pubs.append(o.obs)
            hold.append(p[:N_SLOT] > 0)
            mine.append(o.obs[:N_SLOT] > 0)
            table.append(i)
        a, _, _, _ = ppo.act([o.obs[:sd] for o in obs], [c.cands[:, :cd] for c in obs], greedy=True)
        obs = env.step(a)
    env.close()
    print(f"collected {len(embs)} decisions in {time.time() - t0:.0f}s", flush=True)

    emb = np.stack(embs)
    pub = np.stack(pubs)
    y = np.stack(hold)
    held = np.stack(mine)
    table = np.asarray(table)
    te = table >= (args.tables * 3) // 4
    tr = ~te
    print(f"train {tr.sum()}, test {te.sum()}; next player holds a given card {y.mean():.3f} of the time")

    inputs = {
        "emb": emb,
        "obs": pub[:, :sd],
        "obs+inf": pub,
        "inf": pub[:, OFF_INFER : OFF_INFER + INFER_DIM],
    }
    print(f"{'input':8s} {'all cards':>10s} {'held cards':>11s} {'joker':>6s}")
    for name, x in inputs.items():
        logits = fit_logistic(x[tr], y[tr], x[te])
        yt, ht = y[te], held[te]
        all_auc = [auc(logits[:, k], yt[:, k]) for k in range(N_SLOT)]
        held_auc = []
        for k in range(N_SLOT):
            m = ht[:, k]
            if m.sum() >= 200:
                held_auc.append(auc(logits[m, k], yt[m, k]))
        print(
            f"{name:8s} {np.nanmean(all_auc):10.3f} {np.nanmean(held_auc):11.3f} {all_auc[JOKER]:6.3f}"
            f"   (per-card min {np.nanmin(all_auc):.3f}, max {np.nanmax(all_auc):.3f})",
            flush=True,
        )


if __name__ == "__main__":
    main()
