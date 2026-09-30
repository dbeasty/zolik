"""Smoke tests against the real gameenv (builds it with go)."""

import shutil

import numpy as np
import pytest

from zolik_ml.env import GameEnv, VecEnv, pad_batch
from zolik_ml.ppo import Step, Tracker

pytestmark = pytest.mark.skipif(shutil.which("go") is None, reason="needs go")

DIMS = {"holdem": (295, 14), "canasta": (383, 40), "zolik": (900, 62)}
PLANS = {
    "holdem": [["learner", "station"], ["learner", "learner", "maniac"], ["learner"] * 6, ["hard", "learner", "rock", "riverbluffer"]],
    "canasta": [["learner", "hard"], ["learner", "medium", "learner", "easy"], ["learner", "hard", "hard", "hard"]],
    "zolik": [["learner", "hard"], ["learner", "medium", "learner"], ["easy", "learner", "hard", "learner"], ["closer", "learner", "closer", "closer"]],
}


@pytest.mark.parametrize(
    "game,variation",
    [("holdem", ""), ("canasta", ""), ("zolik", "zolik_classic"), ("zolik", "zolik_classic+floor35")],
)
def test_reset_and_50_steps(game, variation):
    rng = np.random.default_rng(0)
    tracker = Tracker()
    with VecEnv(2, game, variation, budget=50_000) as venv:
        assert venv.envs[0].info() == DIMS[game]
        plans = PLANS[game]
        obs = venv.reset([plans, plans], [11, 500])
        s_dim, c_dim = DIMS[game]
        events = 0
        for _ in range(50):
            choices = []
            for i, tables in enumerate(obs):
                assert len(tables) == len(plans)
                o, c, m = pad_batch([t.obs for t in tables], [t.cands for t in tables])
                assert o.shape == (len(plans), s_dim) and c.shape[2] == c_dim
                assert m.sum(1).min() >= 2  # single-candidate decisions are taken in Go
                ch = []
                for t, tob in enumerate(tables):
                    for e in tob.events:
                        tracker.event((i, t, e.seat), e.reward, e.done)
                        events += 1
                    a = int(rng.integers(len(tob.cands)))
                    tracker.decision((i, t, tob.seat), Step(tob.obs, tob.cands, a, 0.0, 0.0))
                    ch.append(a)
                choices.append(ch)
            obs = venv.step(choices)
        for tables in obs:
            for t in tables:
                assert t.illegal == 0, t
                assert t.stalls == 0, t.last_stall
    assert tracker.harvest(), "no trainable steps"
    if game == "holdem":
        assert events > 0  # hands end inside 50 decisions


def test_error_is_raised():
    from zolik_ml.env import EnvError

    with GameEnv("holdem") as env:
        with pytest.raises(EnvError):
            env.reset([["learner", "nobody"]], 1)
