"""Write the fixtures that pin PyTorch's forward pass to mlp.go's.

    uv run python -m zolik_ml.parity

writes server/internal/learn/testdata/parity/{model.bin,cases.json}: a model
with random weights, some inputs, and what PyTorch made of them.
server/internal/learn/parity_test.go loads both and checks the Go network
gives the same numbers, so CI holds the two to each other without Python.
Regenerate only when the format or the architecture changes.
"""

from __future__ import annotations

import json
from pathlib import Path

import torch

from . import export
from .env import SERVER_DIR
from .model import Policy

PARITY_DIR = SERVER_DIR / "internal" / "learn" / "testdata" / "parity"

# Hold'em's real widths, and small hidden layers so the fixture stays small.
# Two scorer layers and two value layers, so the ReLU-between-but-not-after
# rule is exercised in every stack.
STATE_DIM, CAND_DIM = 295, 14
TRUNK, SCORER, VALUE = [48, 32], [24, 16], [16]


def make(seed: int = 7, n_cases: int = 6) -> tuple[Policy, list[dict]]:
    g = torch.Generator().manual_seed(seed)
    torch.manual_seed(seed)
    model = Policy(STATE_DIM, CAND_DIM, TRUNK, SCORER, VALUE, game="parity")
    # Default init gives outputs near zero, where a transposed weight or a
    # misplaced ReLU would hide inside the tolerance. A gain on every weight
    # and spread-out biases put them at order one.
    with torch.no_grad():
        for name, p in model.named_parameters():
            if name.endswith("weight"):
                p.mul_(2.0)
            else:
                p.add_(0.3 * torch.randn(p.shape, generator=g))
    cases = []
    for i in range(n_cases):
        obs = torch.rand(STATE_DIM, generator=g)
        obs[torch.rand(STATE_DIM, generator=g) < 0.6] = 0  # sparse, like one-hots
        k = 1 + (i * 3) % 8
        cands = torch.rand(k, CAND_DIM, generator=g) * 2 - 0.5
        with torch.no_grad():
            logits, value = model(obs.unsqueeze(0), cands.unsqueeze(0))
        cases.append(
            {
                "obs": obs.tolist(),
                "cands": cands.tolist(),
                "logits": logits[0].tolist(),
                "value": float(value[0]),
            }
        )
    return model, cases


def write(out_dir: Path = PARITY_DIR) -> Path:
    model, cases = make()
    out_dir.mkdir(parents=True, exist_ok=True)
    export.save(model, out_dir / "model.bin")
    (out_dir / "cases.json").write_text(json.dumps(cases) + "\n")
    return out_dir


if __name__ == "__main__":
    print("wrote", write())
