"""The ZLNET1 weight file server/internal/learn/mlp.go loads.

    magic   "ZLNET1\\n"
    uint32  length of the JSON header, little-endian
    header  {"game","stateDim","candDim","trunk":[[in,out]..],"scorer":..,"value":..}
    float32 weights, little-endian: trunk, scorer, value; each layer W (out x in,
            row-major — torch's own Linear layout) then B

Only what Go runs is written. A model trained with a perfect-information
critic (model.py) exports its policy and its *plain* value head; the critic
and its privileged MLP are training state (state.pt) and never leave Python.
"""

from __future__ import annotations

import json
import os
import struct
import tempfile
from pathlib import Path

import numpy as np
import torch

from .model import Policy

MAGIC = b"ZLNET1\n"


def _shape(layers) -> list[list[int]]:
    return [[l.in_features, l.out_features] for l in layers]


def to_bytes(model: Policy) -> bytes:
    header = {
        "game": model.game,
        "stateDim": model.state_dim,
        "candDim": model.cand_dim,
        "trunk": _shape(model.trunk),
        "scorer": _shape(model.scorer),
        "value": _shape(model.value_head),
    }
    hb = json.dumps(header, separators=(",", ":")).encode()
    parts = [MAGIC, struct.pack("<I", len(hb)), hb]
    with torch.no_grad():
        for stack in (model.trunk, model.scorer, model.value_head):
            for layer in stack:
                parts.append(layer.weight.detach().cpu().numpy().astype("<f4").tobytes(order="C"))
                parts.append(layer.bias.detach().cpu().numpy().astype("<f4").tobytes(order="C"))
    return b"".join(parts)


def save(model: Policy, path: str | os.PathLike) -> Path:
    """Write atomically, so an env process loading a checkpoint never sees half a file."""
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=".tmp-", suffix=".bin")
    with os.fdopen(fd, "wb") as f:
        f.write(to_bytes(model))
    os.chmod(tmp, 0o644)
    os.replace(tmp, path)
    return path


def from_bytes(b: bytes) -> Policy:
    if not b.startswith(MAGIC):
        raise ValueError("not a ZLNET1 model")
    off = len(MAGIC)
    (hlen,) = struct.unpack_from("<I", b, off)
    off += 4
    h = json.loads(b[off : off + hlen])
    off += hlen

    def outs(shape):
        return [o for _, o in shape]

    model = Policy(
        h["stateDim"],
        h["candDim"],
        outs(h["trunk"]),
        outs(h["scorer"])[:-1],
        outs(h["value"])[:-1],
        game=h.get("game", ""),
    )
    for stack, shape in ((model.trunk, h["trunk"]), (model.scorer, h["scorer"]), (model.value_head, h["value"])):
        if _shape(stack) != [list(s) for s in shape]:
            raise ValueError(f"layer shapes do not chain as this model's: {shape}")
        for layer in stack:
            for p in (layer.weight, layer.bias):
                n = p.numel()
                arr = np.frombuffer(b, dtype="<f4", count=n, offset=off).reshape(p.shape)
                off += 4 * n
                with torch.no_grad():
                    p.copy_(torch.from_numpy(arr.astype(np.float32)))
    if off != len(b):
        raise ValueError(f"{len(b) - off} trailing bytes")
    return model


def load(path: str | os.PathLike) -> Policy:
    return from_bytes(Path(path).read_bytes())


def load_into(model: Policy, path: str | os.PathLike) -> Policy:
    """Copy a ZLNET1 file's weights into ``model``, which must be the same network.

    Fine-tuning starts a run from a shipped model: the file's game, input
    widths and every layer size have to match the model the config builds, or
    the weights would mean something else (or not fit), so any difference is an
    error naming both shapes rather than a partial load.
    """
    src = load(path)
    want, got = model.config(), src.config()
    if want != got:
        diff = {k: (got.get(k), want.get(k)) for k in want if got.get(k) != want.get(k)}
        raise ValueError(f"{path} is not this model: file vs config {diff}")
    _load_policy_weights(model, src)
    return model


def _load_policy_weights(model: Policy, src: Policy) -> None:
    """src's weights into model, which may also have a critic the file never
    carries (model.py); the critic is left as it is."""
    missing, unexpected = model.load_state_dict(src.state_dict(), strict=False)
    if unexpected or any(not k.startswith(("critic.", "priv_net.")) for k in missing):
        raise ValueError(f"weights do not fit: missing {missing}, unexpected {unexpected}")


def widen_into(model: Policy, path: str | os.PathLike) -> Policy:
    """Load a ZLNET1 file trained for a narrower encoder into ``model``.

    For an encoder that only appended features: the file's state and candidate
    vectors are prefixes of the model's, and every layer size is the same. The
    old weights go where their inputs still are and the new inputs get weight
    zero, so the widened model scores every position and candidate exactly as
    the file did until training moves the new weights. Anything else (a
    different game, layer sizes, or a narrower model) is an error.
    """
    src = load(path)
    want, got = model.config(), src.config()
    same = {k: v for k, v in want.items() if k not in ("state_dim", "cand_dim")}
    if same != {k: v for k, v in got.items() if k not in ("state_dim", "cand_dim")}:
        raise ValueError(f"{path} is not this network: file {got}, config {want}")
    if src.state_dim > model.state_dim or src.cand_dim > model.cand_dim:
        raise ValueError(f"{path} is wider than this model ({src.state_dim}/{src.cand_dim} > {model.state_dim}/{model.cand_dim})")
    with torch.no_grad():
        for dst, old in zip(
            [*model.trunk, *model.scorer, *model.value_head], [*src.trunk, *src.scorer, *src.value_head]
        ):
            if dst.weight.shape == old.weight.shape:
                dst.weight.copy_(old.weight)
            else:
                dst.weight.zero_()
            dst.bias.copy_(old.bias)
        # The two layers that read the encoder: the trunk's first reads the
        # state; the scorer's first reads [embedding ; candidate].
        model.trunk[0].weight[:, : src.state_dim] = src.trunk[0].weight
        e = model.emb_dim
        model.scorer[0].weight[:, :e] = src.scorer[0].weight[:, :e]
        model.scorer[0].weight[:, e : e + src.cand_dim] = src.scorer[0].weight[:, e:]
    return model
