"""Fit the match reward's win-probability model (Suphx's global reward prediction).

    go run ./cmd/matchstates -game canasta -seats 4 -a hard -b closer -seeds 200 > ms/4-hard-closer.jsonl
    uv run python winmodel.py ms/*.jsonl --out configs/canasta-winmodel.json

Each input line is one side's score state before a deal of a finished match
(learn.WinFeatures: own and best-opponent score over the target, the lead,
the higher of the two, the deal number, three sides, four seats or more) and
whether that side went on to win. The model is a small tanh MLP with a sigmoid
output, P(win | score state), which learn.WinModel evaluates in Go; the output
JSON is what reward.match.model names.

Why: a deal's reward is its points difference, which prices 300 points the
same whether the side is 4,800 to 1,000 or 1,000 to 4,800. What ends a match
is reaching the target first, and a side ahead gains most by ending deals
quickly (closing), while a side behind needs points. The deal-only reward
cannot see that; the change in P(win) can.

It also prints k, the scale that brings that change to the deal reward's:
std(deal points difference / 1000) / std(ΔP) over consecutive deals.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import numpy as np
import orjson
import torch
import torch.nn as nn


def read(paths: list[Path]):
    """Rows, and the sequences they form (one side of one match, deal 0 on)."""
    xs, ys, groups, seqs = [], [], [], []
    for path in paths:
        cur = None
        for line in path.read_bytes().splitlines():
            r = orjson.loads(line)
            if r["s"]["deal"] == 0 or cur is None:
                cur = []
                seqs.append(cur)
            cur.append(r)
            xs.append(r["x"])
            ys.append(r["won"])
            groups.append(len(seqs) - 1)
    return np.asarray(xs, np.float32), np.asarray(ys, np.float32), np.asarray(groups), seqs


class Net(nn.Module):
    def __init__(self, n_in: int, hidden: int):
        super().__init__()
        self.l1 = nn.Linear(n_in, hidden)
        self.l2 = nn.Linear(hidden, 1)

    def forward(self, x):
        return self.l2(torch.tanh(self.l1(x))).squeeze(-1)


def fit(x, y, hidden: int, epochs: int, seed: int, wd: float = 1e-4) -> Net:
    torch.manual_seed(seed)
    net = Net(x.shape[1], hidden)
    opt = torch.optim.Adam(net.parameters(), lr=1e-2, weight_decay=wd)
    xt, yt = torch.from_numpy(x), torch.from_numpy(y)
    lossf = nn.BCEWithLogitsLoss()
    for _ in range(epochs):
        opt.zero_grad()
        loss = lossf(net(xt), yt)
        loss.backward()
        opt.step()
    return net


def logloss(net: Net, x, y) -> float:
    with torch.no_grad():
        return float(nn.BCEWithLogitsLoss()(net(torch.from_numpy(x)), torch.from_numpy(y)))


def predict(net: Net, x) -> np.ndarray:
    with torch.no_grad():
        return torch.sigmoid(net(torch.from_numpy(np.asarray(x, np.float32)))).numpy()


def to_json(net: Net) -> dict:
    layers = []
    for l in (net.l1, net.l2):
        layers.append({"w": l.weight.detach().double().numpy().round(6).tolist(), "b": l.bias.detach().double().numpy().round(6).tolist()})
    return {"features": net.l1.in_features, "layers": layers}


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("inputs", nargs="+", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--hidden", type=int, default=16)
    ap.add_argument("--epochs", type=int, default=1500)
    ap.add_argument("--seed", type=int, default=1)
    args = ap.parse_args(argv)

    x, y, g, seqs = read(args.inputs)
    if len(x) == 0:
        sys.exit("no rows")
    # Hold out whole matches, not rows: rows of one match share its result.
    rng = np.random.default_rng(args.seed)
    test_groups = set(rng.choice(len(seqs), size=max(1, len(seqs) // 5), replace=False).tolist())
    te = np.isin(g, list(test_groups))
    base = float(np.mean(y[~te]))
    const = -float(np.mean(y[te] * np.log(base) + (1 - y[te]) * np.log(1 - base)))

    net = fit(x[~te], y[~te], args.hidden, args.epochs, args.seed)
    print(f"{len(x)} rows from {len(seqs)} side-matches; held-out log-loss {logloss(net, x[te], y[te]):.4f} (constant {const:.4f}), train {logloss(net, x[~te], y[~te]):.4f}")

    # Calibration on held-out rows, by predicted decile.
    p = predict(net, x[te])
    order = np.argsort(p)
    print("decile  predicted  actual")
    for d in range(10):
        idx = order[len(order) * d // 10 : len(order) * (d + 1) // 10]
        print(f"{d + 1:6d}  {p[idx].mean():9.3f}  {y[te][idx].mean():6.3f}")

    # Refit on everything for the file.
    net = fit(x, y, args.hidden, args.epochs, args.seed)
    model = to_json(net)

    # k: the deal reward's spread over the change in P's, deal to deal.
    diffs, dps = [], []
    for seq in seqs:
        ps = predict(net, [r["x"] for r in seq])
        for a, b, pa, pb in zip(seq, seq[1:], ps, ps[1:]):
            if a["s"]["sides"] != 2:
                continue
            diffs.append(((b["s"]["own"] - a["s"]["own"]) - (b["s"]["best"] - a["s"]["best"])) / 1000)
            dps.append(pb - pa)
        dps_last = seq[-1]["won"] - ps[-1]
        dps.append(dps_last)
    diffs, dps = np.asarray(diffs), np.asarray(dps)
    k = float(diffs.std() / dps.std())
    print(f"deal reward sd {diffs.std():.3f}, ΔP sd {dps.std():.3f} (last deals included), k = {k:.2f}")
    p00 = predict(net, [[0, 0, 0, 0, 0, 0, 1]])[0]
    print(f"P(win) at 0-0, four seats: {p00:.3f}")

    model["k"] = round(k, 3)  # informational; reward.match.k is what is used
    args.out.write_text(json.dumps(model, indent=1))
    print(f"wrote {args.out}")


if __name__ == "__main__":
    main()
