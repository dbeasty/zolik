"""Measure a trained model against the hand-written bots on held-out seeds.

    uv run python eval.py --game holdem --model runs/holdem-1/final.bin --opponents station,maniac

Wraps server/cmd/gamebench (the duplicate bench: every seed played twice with
the seats swapped) with ``-a net:<model>@<temp> -first 1000000``. Training
only ever deals seeds below 1,000,000, so these are hands the model has not
seen. The unit is the game's own: big blinds per match for Hold'em, points per
match for Canasta, and for Žolíky penalty points per match (positive: the
model took fewer than its opponent). Temperature 0 plays the model's favourite
move, as the server's hard bot would.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
import time
from pathlib import Path

import yaml

from zolik_ml.env import HELD_OUT_SEED, SERVER_DIR, binary

ML_DIR = Path(__file__).resolve().parent
LINE = re.compile(
    r": ([+-]?\d+(?:\.\d+)?) ± (\d+(?:\.\d+)?) per match over (\d+) seeds \(illegal (\d+), stalls (\d+)\)"
)


def bench(
    game: str, model: Path, opp: str, seeds: int, seats: int, temp: float, variation: str = "", first: int = HELD_OUT_SEED, a_seats: int = 0
) -> dict:
    cmd = [
        str(binary("gamebench")),
        "-game", game,
        "-a", f"net:{model.resolve()}@{temp:g}",
        "-b", opp,
        "-seeds", str(seeds),
        "-seats", str(seats),
        "-first", str(first),
    ]
    if variation:
        cmd += ["-variation", variation]
    if a_seats:
        cmd += ["-a-seats", str(a_seats)]
    t = time.time()
    p = subprocess.run(cmd, cwd=SERVER_DIR, capture_output=True, text=True)
    m = LINE.search(p.stdout)
    if not m:
        raise RuntimeError(f"gamebench failed ({p.returncode}): {p.stdout}{p.stderr}")
    mean, se = float(m.group(1)), float(m.group(2))
    return {
        "opponent": opp,
        "mean": mean,
        "se": se,
        "seeds": int(m.group(3)),
        "illegal": int(m.group(4)),
        "stalls": int(m.group(5)),
        "significant": se > 0 and abs(mean) > 2 * se,
        "seconds": round(time.time() - t, 1),
    }


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--game", required=True, choices=["holdem", "canasta", "zolik"])
    ap.add_argument("--model", required=True, type=Path)
    ap.add_argument("--opponents", help="comma-separated; defaults to the config's eval.opponents")
    ap.add_argument("--seeds", type=int)
    ap.add_argument("--seats", type=int)
    ap.add_argument("--variation", help="defaults to the config's eval.variation, else the game's default")
    ap.add_argument("--temp", type=float, default=0.0)
    ap.add_argument("--a-seats", type=int, default=0, help="seats the model plays, the opponent the rest (0: alternate)")
    ap.add_argument("--config", type=Path)
    args = ap.parse_args(argv)

    cfg = yaml.safe_load((args.config or ML_DIR / "configs" / f"{args.game}.yaml").read_text()).get("eval", {})
    opps = args.opponents.split(",") if args.opponents else cfg.get("opponents", ["hard"])
    seeds = args.seeds or cfg.get("seeds", 200)
    seats = args.seats or cfg.get("seats", 2)
    variation = args.variation if args.variation is not None else cfg.get("variation", "")
    unit = {"holdem": "BB/match", "zolik": "penalty/match"}.get(args.game, "points/match")

    print(f"{args.model} at temperature {args.temp:g}, {seats} seats{f' (model in {args.a_seats})' if args.a_seats else ''}{', ' + variation if variation else ''}, {seeds} held-out seeds from {HELD_OUT_SEED} (x2 seatings)")
    print(f"{'opponent':<14} {unit:>14} {'± se':>8}  {'verdict':<10} {'illegal':>7} {'stalls':>6} {'secs':>6}")
    bad = False
    for opp in opps:
        r = bench(args.game, args.model, opp, seeds, seats, args.temp, variation, a_seats=args.a_seats)
        verdict = ("ahead" if r["mean"] > 0 else "behind") if r["significant"] else "even"
        print(f"{opp:<14} {r['mean']:>+14.2f} {r['se']:>8.2f}  {verdict:<10} {r['illegal']:>7} {r['stalls']:>6} {r['seconds']:>6}", flush=True)
        bad |= r["illegal"] + r["stalls"] > 0
    if bad:
        sys.exit(1)


if __name__ == "__main__":
    main()
