"""The candidate-scoring policy, layer for layer what server/internal/learn/mlp.go runs.

    trunk   state             -> embedding   (ReLU after every layer)
    scorer  embedding ++ cand -> one logit   (ReLU between layers, linear last)
    value   embedding         -> one number  (ReLU between layers, linear last)

Nothing is normalised inside the model: the server feeds Encode's vector and
the candidates' features straight in, and so must training. Any scaling the
network needs it learns in its first layer.
"""

from __future__ import annotations

import torch
import torch.nn as nn
import torch.nn.functional as F


def _stack(sizes: list[int]) -> nn.ModuleList:
    return nn.ModuleList(nn.Linear(a, b) for a, b in zip(sizes[:-1], sizes[1:]))


class Policy(nn.Module):
    def __init__(
        self,
        state_dim: int,
        cand_dim: int,
        trunk: list[int],
        scorer: list[int],
        value: list[int],
        game: str = "",
    ):
        super().__init__()
        if not trunk:
            raise ValueError("the trunk needs at least one layer")
        self.game = game
        self.state_dim = state_dim
        self.cand_dim = cand_dim
        self.emb_dim = trunk[-1]
        self.trunk = _stack([state_dim, *trunk])
        self.scorer = _stack([self.emb_dim + cand_dim, *scorer, 1])
        self.value_head = _stack([self.emb_dim, *value, 1])

    def config(self) -> dict:
        return {
            "game": self.game,
            "state_dim": self.state_dim,
            "cand_dim": self.cand_dim,
            "trunk": [l.out_features for l in self.trunk],
            "scorer": [l.out_features for l in self.scorer][:-1],
            "value": [l.out_features for l in self.value_head][:-1],
        }

    def embed(self, obs: torch.Tensor) -> torch.Tensor:
        x = obs
        for layer in self.trunk:
            x = F.relu(layer(x))
        return x

    def logits(self, emb: torch.Tensor, cands: torch.Tensor, mask: torch.Tensor | None = None) -> torch.Tensor:
        """Score ``cands [B,K,C]`` against ``emb [B,E]``; padding is -inf.

        The first scorer layer is applied to the embedding once per state and
        to each candidate separately — W·[e;c] = W_e·e + W_c·c — rather than to
        B·K concatenations. The same arithmetic mlp.go does, reassociated; the
        parity test holds the two to 1e-5.
        """
        first = self.scorer[0]
        w_e = first.weight[:, : self.emb_dim]
        w_c = first.weight[:, self.emb_dim :]
        x = F.linear(emb, w_e, first.bias).unsqueeze(1) + F.linear(cands, w_c)
        for layer in self.scorer[1:]:
            x = layer(F.relu(x))
        out = x.squeeze(-1)
        if mask is not None:
            out = out.masked_fill(~mask, float("-inf"))
        return out

    def value(self, emb: torch.Tensor) -> torch.Tensor:
        x = emb
        for i, layer in enumerate(self.value_head):
            if i:
                x = F.relu(x)
            x = layer(x)
        return x.squeeze(-1)

    def forward(self, obs: torch.Tensor, cands: torch.Tensor, mask: torch.Tensor | None = None):
        emb = self.embed(obs)
        return self.logits(emb, cands, mask), self.value(emb)


def masked_log_softmax(logits: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    """log-probabilities with padding at 0 rather than -inf, safe to multiply."""
    return torch.log_softmax(logits, dim=-1).masked_fill(~mask, 0.0)


def entropy(logits: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    logp = masked_log_softmax(logits, mask)
    return -(logp.exp() * logp).sum(-1)


def from_config(cfg: dict, state_dim: int, cand_dim: int, game: str) -> Policy:
    m = cfg["model"]
    return Policy(state_dim, cand_dim, list(m["trunk"]), list(m["scorer"]), list(m["value"]), game=game)
