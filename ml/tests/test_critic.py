"""The perfect-information critic and the match reward: training-only, so the
policy and the exported file must be exactly what they are without them."""

import shutil

import numpy as np
import pytest
import torch

from zolik_ml import export
from zolik_ml.env import GameEnv
from zolik_ml.model import Policy
from zolik_ml.ppo import PPO, PPOConfig, Segment, Step, build_batch


def critic_model(seed=0):
    torch.manual_seed(seed)
    return Policy(11, 5, [8, 6], [7], [4], game="toy", priv_dim=9, priv=[10, 3])


def test_policy_never_reads_the_privileged_observation():
    m = critic_model()
    obs, cands = torch.rand(4, 11), torch.rand(4, 3, 5)
    p1, p2 = torch.rand(4, 9), torch.rand(4, 9) * 10
    with torch.no_grad():
        l1, v1 = m(obs, cands, priv=p1)
        l2, v2 = m(obs, cands, priv=p2)
        l0, _ = m(obs, cands)
    assert torch.equal(l1, l2) and torch.equal(l1, l0)
    assert not torch.allclose(v1, v2)  # the critic does read it
    # No gradient path from the privileged input to a logit.
    p = torch.rand(4, 9, requires_grad=True)
    logits, value = m(obs, cands, priv=p)
    assert torch.autograd.grad(logits.sum(), p, allow_unused=True)[0] is None
    assert torch.autograd.grad(value.sum(), p)[0].abs().sum() > 0


def test_exported_policy_is_identical_with_or_without_the_critic(tmp_path):
    m = critic_model()
    plain = Policy(11, 5, [8, 6], [7], [4], game="toy")
    # The same policy and plain head, without a critic.
    plain.load_state_dict({k: v for k, v in m.state_dict().items() if not k.startswith(("critic.", "priv_net."))})
    assert export.to_bytes(m) == export.to_bytes(plain)
    back = export.load(export.save(m, tmp_path / "m.bin"))
    assert not back.has_critic and back.config() == plain.config()
    obs, cands = torch.rand(3, 11), torch.rand(3, 4, 5)
    with torch.no_grad():
        a, b = m(obs, cands), back(obs, cands)  # no priv: the plain head
    assert torch.equal(a[0], b[0]) and torch.equal(a[1], b[1])


def test_load_into_a_critic_model_and_start_the_critic_from_the_value_head(tmp_path):
    src = Policy(11, 5, [8, 6], [7], [4], game="toy")
    path = export.save(src, tmp_path / "src.bin")
    m = critic_model(seed=3)
    export.load_into(m, path)
    m.init_critic_from_value()
    obs, cands, priv = torch.rand(5, 11), torch.rand(5, 2, 5), torch.rand(5, 9)
    with torch.no_grad():
        emb = m.embed(obs)
        critic, plain = m.values(emb, priv)
        assert torch.allclose(critic, src.value(src.embed(obs)), atol=1e-6)
        assert torch.allclose(plain, critic, atol=1e-6)
    # widen too
    wide = Policy(13, 6, [8, 6], [7], [4], game="toy", priv_dim=9, priv=[10, 3])
    export.widen_into(wide, path)
    wide.init_critic_from_value()


def test_update_trains_the_critic_and_the_plain_head():
    m = critic_model()
    ppo = PPO(m, PPOConfig(minibatch=16, epochs=2, target_kl=None))
    rng = np.random.default_rng(0)
    segs = []
    for _ in range(8):
        steps = []
        for t in range(6):
            o, c, p = rng.random(11, dtype=np.float32), rng.random((3, 5), dtype=np.float32), rng.random(9, dtype=np.float32)
            a, logp, v, vp = ppo.act([o], [c], priv=[p])
            steps.append(Step(o, c, int(a[0]), float(logp[0]), float(v[0]), reward=float(p[0]), priv=p, value_plain=float(vp[0])))
        steps[-1].done = True
        segs.append(Segment(steps, 0.0))
    batch = build_batch(segs, 0.99, 0.95, 1.0)
    assert batch.priv is not None and batch.values_plain is not None
    before = {k: v.clone() for k, v in m.state_dict().items()}
    st = ppo.update(batch)
    assert np.isfinite(st.value_loss) and np.isfinite(st.explained_var_plain)
    for k in ("critic.0.weight", "priv_net.0.weight", "value_head.0.weight", "trunk.0.weight"):
        assert not torch.equal(before[k], m.state_dict()[k]), k


def test_without_priv_the_batch_is_as_before():
    s = Step(np.zeros(2, "f4"), np.zeros((2, 1), "f4"), 0, -0.7, 0.3, done=True)
    b = build_batch([Segment([s], 0.0)], 1.0, 1.0, 1.0)
    assert b.priv is None and b.values_plain[0] == np.float32(0.3)


needs_go = pytest.mark.skipif(shutil.which("go") is None, reason="needs go")


@needs_go
def test_env_sends_privileged_only_when_asked():
    plans = [["learner", "hard"], ["learner", "hard", "learner", "closer"]]
    with GameEnv("canasta", "classic") as plain, GameEnv("canasta", "classic", privileged=True) as priv:
        info = priv.info_full()
        assert info["privDim"] == 121 and plain.info() == (539, 44)
        a, b = plain.reset(plans, 77), priv.reset(plans, 77)
        for _ in range(10):
            for x, y in zip(a, b):
                assert x.priv is None and y.priv is not None and y.priv.shape == (121,)
                np.testing.assert_array_equal(x.obs, y.obs)
            choices = [0] * len(a)
            a, b = plain.step(choices), priv.step(choices)


@needs_go
def test_env_match_reward_alpha_one_is_the_deal_reward():
    model = {"features": 7, "layers": [{"w": [[0, 0, 3, 0, 0, 0, 0]], "b": [0]}]}
    plans = [["learner", "hard"]]

    def rewards(env, base=False):
        obs, out = env.reset(plans, 5), []
        for _ in range(400):
            out += [e.base if base else e.reward for e in obs[0].events if e.done and e.seat == "p0"]
            obs = env.step([0])
        return out

    with GameEnv("canasta", "classic") as e1, GameEnv("canasta", "classic", match_reward={"alpha": 1.0, "k": 2.0, "model": model}) as e2:
        r1, r2 = rewards(e1), rewards(e2)
    assert r1 and r1 == r2
    with GameEnv("canasta", "classic", match_reward={"alpha": 0.0, "k": 2.0, "model": model}) as e3:
        r3 = rewards(e3)
        bases = rewards(e3, base=True)
    assert bases == r1  # the deal reward still reported beside the mixed one
    assert len(r3) == len(r1) and r3 != r1
    assert all(abs(r) <= 2.0 + 1e-6 for r in r3)  # k * ΔP, |ΔP| <= 1


@needs_go
def test_env_go_out_shaping_reaches_the_env():
    plans = [["learner", "hard"]]

    def episodes(env):
        obs, out = env.reset(plans, 5), []
        for _ in range(400):
            out += [(e.reward, e.base) for e in obs[0].events if e.done and e.seat == "p0"]
            obs = env.step([0])
        return out

    with GameEnv("canasta", "classic") as plain, GameEnv("canasta", "classic", go_out={"out": 0.1, "held": 1.0, "cap": 0.3}) as shaped:
        a, b = episodes(plain), episodes(shaped)
    assert a and len(a) == len(b)
    assert [r for r, _ in a] == [base for _, base in b]  # the game's own reward still reported
    for r, base in b:
        d = r - base
        assert abs(d - 0.1) < 1e-5 or -0.3 - 1e-5 <= d <= 1e-6
    assert any(abs(r - base) > 1e-6 for r, base in b)
