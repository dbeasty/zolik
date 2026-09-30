# ml/ — training zolik's learned bots

The trainer for the bots in `server/internal/learn`. It never re-implements a
game: the Go environment (`server/cmd/gameenv`) plays every table with the real
engine and hands Python only what the game's adapter produces — `Encode`'s
state vector, one feature vector per legal candidate move, and rewards. Python
learns a policy that scores candidates, and writes it in the `ZLNET1` format
that `server/internal/learn/mlp.go` loads.

```
ml/
  train.py            PPO training loop (TensorBoard, checkpoints, final.bin)
  eval.py             held-out duplicate bench against the hand-written bots
  configs/            holdem.yaml, canasta.yaml, zolik.yaml — network sizes, env, PPO, league
  zolik_ml/env.py     gameenv client: JSON lines, candidate padding, VecEnv
  zolik_ml/model.py   the policy, layer for layer what mlp.go runs
  zolik_ml/export.py  ZLNET1 writer and loader
  zolik_ml/parity.py  writes the Go parity fixtures
  zolik_ml/ppo.py     per-seat trajectories, GAE, PPO update
  zolik_ml/league.py  table plans: checkpoints, heuristics, self-play
  tests/              pytest
```

## Setup

Needs Go (the environment is built from `server/`) and [uv](https://docs.astral.sh/uv/).

```sh
cd ml
uv sync            # Python 3.11+, torch (CPU), numpy, tensorboard, pyyaml, orjson, pytest
uv run pytest      # includes a smoke test against the real gameenv for both games
```

The first `train.py`/`eval.py`/test run builds `gameenv` and `gamebench` into
`ml/.bin/` with `go build` (incremental, so a Go edit is always picked up).

## Train

```sh
uv run python train.py --game holdem  --run holdem-1  --minutes 60
uv run python train.py --game canasta --run canasta-1 --minutes 600
uv run python train.py --game zolik   --run zolik-1   --minutes 30
uv run tensorboard --logdir runs
```

Any config value can be overridden with `--set key.path=value` (YAML values):

```sh
# Heads-up heavy, aimed at the exploitable styles:
uv run python train.py --game holdem --run exploit --minutes 20 \
  --set "league.seats={2: 0.7, 3: 0.1, 4: 0.1, 6: 0.1}" \
  --set "league.blend={checkpoint: 0.2, heuristic: 0.7, self: 0.1}" \
  --set "league.opponents={station: 1.0, maniac: 1.0, hard: 0.5, rock: 0.25, riverbluffer: 0.25, medium: 0.25}"
```

To fine-tune from a trained model instead of from scratch, give it with
`--init` (a ZLNET1 file; the config's model sizes must match it, or it refuses).
The optimiser starts fresh and the model is also the league's first opponent.
`configs/zolik-4p35.yaml` is such a fine-tune for the four-seat table with a
35-point first meld (`zolik_classic+floor35`, the lobby's rules exactly):

```sh
uv run python train.py --game zolik --config configs/zolik-4p35.yaml --run zolik-4p35 \
  --minutes 120 --init runs/zolik-long/final.bin
```

A run writes `runs/<name>/`:

| file | what |
|---|---|
| `tb/` | TensorBoard: `reward/episode`, `reward_vs/<opp>`, `win_vs/<opp>`, `loss/{policy,value,entropy,approx_kl,clip_frac,explained_var}`, `perf/decisions_per_s`, `env/{illegal,stalls,matches}` |
| `metrics.jsonl` | the same, one line per update |
| `ckpt/<update>.bin` | snapshots every `train.snapshot_every` updates; also the league's opponent pool |
| `state.pt` | model + optimiser, for `--resume runs/<name>/state.pt` |
| `final.bin` | the model at the end (also written on Ctrl-C) |

How it works, briefly:

- `env.envs` gameenv processes (default 8) × `env.tables` tables each. The
  loop serves whichever process answers first, so a slow step on one never
  idles the rest. Each process gets `GOMAXPROCS=env.procs` and `GOGC=env.gogc`.
- Trajectories are keyed by (process, table, seat). A reward event is added to
  that seat's latest decision; Done closes the trajectory. Events for a seat
  with no open trajectory — a reward before its first real decision (a walk in
  the big blind), or the extra zero-reward Done the env sends at the end of a
  match — are ignored for training (still counted in the episode statistics).
- The league deals each table as checkpoint / heuristic / self-play by
  `league.blend`, with seat counts from `league.seats`. Canasta at four seats
  partners the learner with another learner seat or with `league.partner_bot`.
  Samba is off unless `league.samba: true` (every other process then plays
  Samba with `league.variations.samba` seat counts). `league.learners`
  ({learner seats: weight}) seats more than one learner at a checkpoint or
  heuristic table (labelled `<opp>/<k>L`), for the table where several copies
  of the model sit with one other player. Žolíky names its
  rulesets in `league.variation_mix` instead, and the env processes are split
  between them in proportion (default 6 classic : 2 floor35 of 8; Continental
  is left out while the Hard heuristic wedges there). Table plans are re-dealt
  every `train.reset_every` updates, staggered across processes.
- Training seeds are always below 1,000,000; evaluation starts there.

## Evaluate

```sh
uv run python eval.py --game holdem --model runs/holdem-1/final.bin --opponents station,maniac,hard --seeds 500
uv run python eval.py --game canasta --model runs/canasta-1/final.bin --opponents hard --seeds 100 --seats 2
uv run python eval.py --game zolik --model runs/zolik-1/final.bin --opponents hard --seeds 300 --variation zolik_classic+floor35
```

This is `server/cmd/gamebench -a net:<model>@<temp> -b <opp> -first 1000000`:
the duplicate bench, every seed played twice with the seats swapped, on seeds
training never dealt. Temperature defaults to 0 (always the favourite move).
`--a-seats k` (gamebench `-a-seats`) seats the model in k seats and the
opponent in the rest, each seed played once per rotation of the table — one
model with three hard bots, or three copies of it with one.
"ahead"/"behind" means more than two standard errors either way; any illegal
move or stall exits non-zero. Units are the game's own — big blinds per match
for Hold'em (15-hand matches, 50 BB stacks), points per match for Canasta,
penalty points per match for Žolíky (positive: the model took fewer).

## Parity with the server

`server/internal/learn/parity_test.go` loads `testdata/parity/model.bin` and
`cases.json` (random weights, inputs, and PyTorch's logits and value) and
checks `LoadNet`/`Embed`/`Logits`/`ValueOf` agree to 1e-5. The fixtures are
committed, so CI runs it without Python. Regenerate them only when the format
or architecture changes:

```sh
uv run python -m zolik_ml.parity
cd ../server && go test ./internal/learn/ -run Parity
```

The model normalises nothing: whatever scaling the network needs, it learns in
its first layer, because the server feeds `Encode`'s vector straight in.

## Shipping weights

A trained model is one small file (`final.bin`, 0.8–0.9 MB at the default sizes).
To ship one, evaluate it on held-out seeds, then copy it into the game's
package — Phase 3 embeds it at `server/internal/<game>/models/hard.bin` and
plays it through `learn.HardBot(game, bytes, heuristic)`: one `learn.Policy`
per model per process, shared by every seat at every table, with the heuristic
as fallback. The server
refuses a model whose `stateDim`/`candDim` differ from the adapter's, so any
change to an encoder means retraining.

## Known issues

- Canasta can still stall, rarely (about one match in a few thousand): at the
  end of the deck a seat's only enabled move is taking the pile onto a meld,
  and that leaves it where nothing but taking it back is enabled. The engine
  should end the deal there. `env/stalls` and the `stall xN:` log lines count
  it; the env closes the episode and deals the next seed.
