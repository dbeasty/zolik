"""PPO with GAE over variable candidate sets.

The environment is multi-agent and asynchronous from any one seat's point of
view: a table can hold several learner seats, and a seat's reward arrives in
an ``events`` list after other seats (at the same table) have decided. So
trajectories are kept per (env, table, seat), and:

* a reward event is added to that seat's most recent decision;
* a Done event closes that seat's trajectory;
* an event for a seat with no open trajectory is not training signal — a
  reward that landed before the seat's first real decision of the episode, or
  the environment's extra zero-reward Done at the end of a match for an
  episode that already closed — and is ignored (the episode statistics still
  count the reward).

Trajectories outlive a rollout. At update time every closed trajectory is
used whole, and every open one gives up all but its last decision,
bootstrapped from that decision's value (truncated GAE); the last decision
stays behind to collect its reward and successor.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np
import torch
import torch.nn as nn

from .env import pad_batch
from .model import Policy, entropy, masked_log_softmax

Key = tuple  # (env, table, seat)


@dataclass
class Step:
    obs: np.ndarray
    cands: np.ndarray
    action: int
    logp: float
    value: float
    reward: float = 0.0
    done: bool = False


@dataclass
class Segment:
    steps: list[Step]
    bootstrap: float  # value after the last step; 0 when it ended the episode


@dataclass
class EpisodeStat:
    key: Key
    ret: float  # every reward the seat was credited in the episode
    decisions: int


class Tracker:
    """Per-seat trajectory bookkeeping with delayed rewards."""

    def __init__(self):
        self.open: dict[Key, list[Step]] = {}
        self.ready: list[Segment] = []
        # Episode accounting, independent of whether the seat had decisions.
        self._ep_ret: dict[Key, float] = {}
        self._ep_dec: dict[Key, int] = {}
        self._ep_live: set[Key] = set()
        self.orphan_reward = 0.0  # reward that reached no decision
        self.orphan_events = 0

    def event(self, key: Key, reward: float, done: bool) -> EpisodeStat | None:
        if reward != 0.0:
            self._ep_live.add(key)
            self._ep_ret[key] = self._ep_ret.get(key, 0.0) + reward
        traj = self.open.get(key)
        if traj:
            traj[-1].reward += reward
            if done:
                traj[-1].done = True
                self.ready.append(Segment(traj, 0.0))
                del self.open[key]
        elif reward != 0.0 or done:
            self.orphan_reward += reward
            self.orphan_events += 1
        if done:
            return self._close_episode(key)
        return None

    def _close_episode(self, key: Key) -> EpisodeStat | None:
        live = key in self._ep_live
        stat = EpisodeStat(key, self._ep_ret.pop(key, 0.0), self._ep_dec.pop(key, 0)) if live else None
        self._ep_live.discard(key)
        return stat

    def decision(self, key: Key, step: Step) -> None:
        self.open.setdefault(key, []).append(step)
        self._ep_live.add(key)
        self._ep_dec[key] = self._ep_dec.get(key, 0) + 1

    def harvest(self) -> list[Segment]:
        """Everything trainable now: closed trajectories, and open ones up to
        their last decision."""
        for key, traj in self.open.items():
            if len(traj) >= 2:
                self.ready.append(Segment(traj[:-1], traj[-1].value))
                self.open[key] = traj[-1:]
        out, self.ready = self.ready, []
        return out

    def drop(self, pred) -> None:
        """Abandon open trajectories whose key matches (an env being reset).

        Their completed prefix is kept, bootstrapped as in harvest; the last
        decision, whose outcome will never arrive, is discarded.
        """
        for key in [k for k in self.open if pred(k)]:
            traj = self.open.pop(key)
            if len(traj) >= 2:
                self.ready.append(Segment(traj[:-1], traj[-1].value))
            self._ep_ret.pop(key, None)
            self._ep_dec.pop(key, None)
            self._ep_live.discard(key)

    def pending_steps(self) -> int:
        return sum(len(s.steps) for s in self.ready) + sum(len(t) - 1 for t in self.open.values())


def gae(rewards, values, dones, bootstrap: float, gamma: float, lam: float):
    """Generalised advantage estimation for one segment of one seat."""
    n = len(rewards)
    adv = np.zeros(n, dtype=np.float32)
    last = 0.0
    next_value = bootstrap
    for t in reversed(range(n)):
        nonterminal = 0.0 if dones[t] else 1.0
        delta = rewards[t] + gamma * next_value * nonterminal - values[t]
        last = delta + gamma * lam * nonterminal * last
        adv[t] = last
        next_value = values[t]
    return adv, adv + np.asarray(values, dtype=np.float32)


@dataclass
class Batch:
    obs: list[np.ndarray]
    cands: list[np.ndarray]
    actions: np.ndarray
    logp: np.ndarray
    values: np.ndarray
    adv: np.ndarray
    returns: np.ndarray

    def __len__(self) -> int:
        return len(self.actions)


def build_batch(segments: list[Segment], gamma: float, lam: float, reward_scale: float) -> Batch:
    obs, cands, actions, logp, values, advs, rets = [], [], [], [], [], [], []
    for seg in segments:
        r = [s.reward * reward_scale for s in seg.steps]
        v = [s.value for s in seg.steps]
        d = [s.done for s in seg.steps]
        a, ret = gae(r, v, d, seg.bootstrap, gamma, lam)
        advs.append(a)
        rets.append(ret)
        for s in seg.steps:
            obs.append(s.obs)
            cands.append(s.cands)
            actions.append(s.action)
            logp.append(s.logp)
            values.append(s.value)
    return Batch(
        obs,
        cands,
        np.asarray(actions, dtype=np.int64),
        np.asarray(logp, dtype=np.float32),
        np.asarray(values, dtype=np.float32),
        np.concatenate(advs) if advs else np.zeros(0, np.float32),
        np.concatenate(rets) if rets else np.zeros(0, np.float32),
    )


@dataclass
class PPOConfig:
    lr: float = 3e-4
    gamma: float = 1.0
    lam: float = 0.95
    clip: float = 0.2
    vf_coef: float = 0.5
    ent_coef: float = 0.01
    max_grad_norm: float = 0.5
    epochs: int = 4
    minibatch: int = 2048
    target_kl: float | None = 0.03
    reward_scale: float = 1.0

    @classmethod
    def from_dict(cls, d: dict) -> "PPOConfig":
        known = {k: v for k, v in d.items() if k in cls.__dataclass_fields__}
        return cls(**known)


@dataclass
class UpdateStats:
    policy_loss: float = 0.0
    value_loss: float = 0.0
    entropy: float = 0.0
    approx_kl: float = 0.0
    clip_frac: float = 0.0
    explained_var: float = 0.0
    grad_norm: float = 0.0  # before clipping
    minibatches: int = 0
    stopped_early: bool = False
    extra: dict = field(default_factory=dict)


class PPO:
    def __init__(self, model: Policy, cfg: PPOConfig, device: str = "cpu"):
        self.model = model
        self.cfg = cfg
        self.device = device
        self.opt = torch.optim.Adam(model.parameters(), lr=cfg.lr, eps=1e-5)

    def set_lr_fraction(self, frac: float) -> float:
        lr = self.cfg.lr * max(frac, 0.0)
        for g in self.opt.param_groups:
            g["lr"] = lr
        return lr

    @torch.no_grad()
    def act(self, obs: list[np.ndarray], cands: list[np.ndarray], greedy: bool = False):
        """Sample one candidate per row; returns (actions, logp, values)."""
        o, c, m = pad_batch(obs, cands, self.model.cand_dim)
        o, c, m = (torch.from_numpy(x).to(self.device) for x in (o, c, m))
        logits, value = self.model(o, c, m)
        if greedy:
            a = logits.argmax(-1)
        else:
            a = torch.distributions.Categorical(logits=logits).sample()
        logp = torch.log_softmax(logits, -1).gather(1, a.unsqueeze(1)).squeeze(1)
        return a.cpu().numpy(), logp.cpu().numpy(), value.cpu().numpy()

    def update(self, batch: Batch) -> UpdateStats:
        cfg = self.cfg
        n = len(batch)
        st = UpdateStats()
        if n == 0:
            return st
        adv_all = batch.adv
        adv_all = (adv_all - adv_all.mean()) / (adv_all.std() + 1e-8)
        kls = []
        for _ in range(cfg.epochs):
            perm = np.random.permutation(n)
            epoch_kl = []
            for start in range(0, n, cfg.minibatch):
                idx = perm[start : start + cfg.minibatch]
                if len(idx) < 2:
                    continue
                o, c, m = pad_batch([batch.obs[i] for i in idx], [batch.cands[i] for i in idx], self.model.cand_dim)
                o, c, m = (torch.from_numpy(x).to(self.device) for x in (o, c, m))
                act = torch.from_numpy(batch.actions[idx]).to(self.device)
                old_logp = torch.from_numpy(batch.logp[idx]).to(self.device)
                adv = torch.from_numpy(adv_all[idx]).to(self.device)
                ret = torch.from_numpy(batch.returns[idx]).to(self.device)

                logits, value = self.model(o, c, m)
                logp_all = masked_log_softmax(logits, m)
                logp = logp_all.gather(1, act.unsqueeze(1)).squeeze(1)
                log_ratio = logp - old_logp
                ratio = log_ratio.exp()
                pg = -torch.min(ratio * adv, ratio.clamp(1 - cfg.clip, 1 + cfg.clip) * adv).mean()
                vloss = 0.5 * (value - ret).pow(2).mean()
                ent = entropy(logits, m).mean()
                loss = pg + cfg.vf_coef * vloss - cfg.ent_coef * ent

                self.opt.zero_grad()
                loss.backward()
                st.grad_norm += float(nn.utils.clip_grad_norm_(self.model.parameters(), cfg.max_grad_norm))
                self.opt.step()

                with torch.no_grad():
                    kl = ((ratio - 1) - log_ratio).mean().item()
                    st.clip_frac += ((ratio - 1).abs() > cfg.clip).float().mean().item()
                st.policy_loss += pg.item()
                st.value_loss += vloss.item()
                st.entropy += ent.item()
                epoch_kl.append(kl)
                st.minibatches += 1
            kls.extend(epoch_kl)
            if cfg.target_kl and epoch_kl and np.mean(epoch_kl) > cfg.target_kl:
                st.stopped_early = True
                break
        k = max(st.minibatches, 1)
        st.policy_loss /= k
        st.value_loss /= k
        st.entropy /= k
        st.clip_frac /= k
        st.grad_norm /= k
        st.approx_kl = float(np.mean(kls)) if kls else 0.0
        var = batch.returns.var()
        st.explained_var = float(1 - (batch.returns - batch.values).var() / var) if var > 0 else 0.0
        return st
