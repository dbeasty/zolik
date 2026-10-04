"""The candidate-scoring policy, layer for layer what server/internal/learn/mlp.go runs.

    trunk   state             -> embedding   (ReLU after every layer)
    scorer  embedding ++ cand -> one logit   (ReLU between layers, linear last)
    value   embedding         -> one number  (ReLU between layers, linear last)

Nothing is normalised inside the model: the server feeds Encode's vector and
the candidates' features straight in, and so must training. Any scaling the
network needs it learns in its first layer.

With ``priv_dim`` > 0 the model also has a perfect-information critic
(PerfectDou's perfect-training, imperfect-execution), for training only:

    priv    privileged obs           -> priv embedding (ReLU after every layer)
    critic  embedding ++ priv emb    -> one number (the value GAE uses)

The policy — trunk and scorer — never reads the privileged observation, and
the exported ZLNET1 file carries trunk, scorer and the plain value head only
(export.py), so a model trained with a critic plays, and loads in Go, exactly
as one without. The plain head is still trained, by regression on the same
returns from a detached embedding (so it never shapes the trunk), and is what
Go's ValueOf reads; nothing in the server uses it.
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
        priv_dim: int = 0,
        priv: list[int] | None = None,
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
        self.priv_dim = priv_dim
        self.priv_net = None
        self.critic = None
        if priv_dim:
            priv = list(priv or [128])
            self.priv_net = _stack([priv_dim, *priv])
            self.critic = _stack([self.emb_dim + priv[-1], *value, 1])

    @property
    def has_critic(self) -> bool:
        return self.critic is not None

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
        """The plain value head: the one exported, reading the embedding only."""
        return _head(self.value_head, emb)

    def critic_value(self, emb: torch.Tensor, priv: torch.Tensor) -> torch.Tensor:
        """The perfect-information critic: embedding ++ privileged embedding."""
        p = priv
        for layer in self.priv_net:
            p = F.relu(layer(p))
        return _head(self.critic, torch.cat([emb, p], dim=-1))

    def values(self, emb: torch.Tensor, priv: torch.Tensor | None = None) -> tuple[torch.Tensor, torch.Tensor]:
        """(the value GAE uses, the plain head's).

        With a critic and a privileged observation the first is the critic and
        the plain head reads a detached embedding; otherwise both are the plain
        head, as before there was a critic.
        """
        if self.has_critic and priv is not None:
            return self.critic_value(emb, priv), self.value(emb.detach())
        v = self.value(emb)
        return v, v

    def init_critic_from_value(self) -> None:
        """Start the critic as the plain head: its privileged inputs at weight
        zero, so a fine-tune from a model without one begins from that model's
        value estimates rather than from noise."""
        if not self.has_critic:
            return
        with torch.no_grad():
            for dst, src in zip(self.critic, self.value_head):
                dst.weight.zero_()
                dst.weight[:, : src.in_features] = src.weight
                dst.bias.copy_(src.bias)

    def forward(self, obs: torch.Tensor, cands: torch.Tensor, mask: torch.Tensor | None = None, priv: torch.Tensor | None = None):
        emb = self.embed(obs)
        return self.logits(emb, cands, mask), self.values(emb, priv)[0]


def _head(stack: nn.ModuleList, x: torch.Tensor) -> torch.Tensor:
    for i, layer in enumerate(stack):
        if i:
            x = F.relu(x)
        x = layer(x)
    return x.squeeze(-1)


def masked_log_softmax(logits: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    """log-probabilities with padding at 0 rather than -inf, safe to multiply."""
    return torch.log_softmax(logits, dim=-1).masked_fill(~mask, 0.0)


def entropy(logits: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    logp = masked_log_softmax(logits, mask)
    return -(logp.exp() * logp).sum(-1)


def from_config(cfg: dict, state_dim: int, cand_dim: int, game: str, priv_dim: int = 0) -> Policy:
    """The config's network; with priv_dim and ``model.priv`` (the privileged
    MLP's sizes) set, with a perfect-information critic."""
    m = cfg["model"]
    priv = m.get("priv")
    return Policy(
        state_dim,
        cand_dim,
        list(m["trunk"]),
        list(m["scorer"]),
        list(m["value"]),
        game=game,
        priv_dim=priv_dim if priv else 0,
        priv=list(priv) if priv else None,
    )
