"""The auxiliary hidden-card head and potential-based shaping: training-only,
so the exported file is what it is without them, and shaping sums over an
episode to what the potential says."""

import numpy as np
import torch

from zolik_ml import export
from zolik_ml.model import Policy
from zolik_ml.ppo import PPO, PPOConfig, Segment, Step, Tracker, build_batch


def aux_model(seed=0):
    torch.manual_seed(seed)
    return Policy(11, 5, [8, 6], [7], [4], game="toy", aux_dim=4, aux=[5])


def test_aux_head_is_not_exported(tmp_path):
    m = aux_model()
    plain = Policy(11, 5, [8, 6], [7], [4], game="toy")
    plain.load_state_dict({k: v for k, v in m.state_dict().items() if not k.startswith("aux_head.")})
    assert export.to_bytes(m) == export.to_bytes(plain)
    # and a file loads into a model that has one
    m2 = aux_model(seed=4)
    export.load_into(m2, export.save(plain, tmp_path / "p.bin"))
    assert m2.has_aux and m2.wants_priv and not m2.has_critic


def test_aux_loss_trains_the_trunk_and_the_head():
    m = aux_model()
    ppo = PPO(m, PPOConfig(minibatch=64, epochs=1, aux_coef=1.0))
    rng = np.random.default_rng(0)
    steps = []
    for _ in range(40):
        priv = (rng.random(9) > 0.5).astype(np.float32)
        steps.append(Step(rng.random(11, dtype=np.float32), rng.random((3, 5), dtype=np.float32), 0, -1.1, 0.0, priv=priv))
    steps[-1].done = True
    batch = build_batch([Segment(steps, 0.0)], 1.0, 0.95, 1.0)
    assert batch.priv is not None
    before = [p.clone() for p in m.aux_head.parameters()] + [m.trunk[0].weight.clone()]
    st = ppo.update(batch)
    after = [*m.aux_head.parameters(), m.trunk[0].weight]
    assert st.aux_loss > 0
    assert all(not torch.equal(a, b) for a, b in zip(before, after))


def test_shaping_sums_to_minus_the_first_potential():
    tr = Tracker({"index": 0, "weight": 0.5}, gamma=1.0)
    key = (0, 0, "p1")
    dist = [6.0, 5.0, 5.0, 3.0, 1.0]
    for d in dist:
        tr.decision(key, Step(np.array([d], np.float32), np.zeros((1, 1), np.float32), 0, 0.0, 0.0))
    tr.event(key, 1.0, True)
    seg = tr.harvest()
    total = sum(s.reward for seg_ in seg for s in seg_.steps)
    # the deal's own reward plus -Phi(first) = +0.5 * 6
    assert abs(total - (1.0 + 0.5 * 6.0)) < 1e-6
    # the step that closed the distance most is paid the most
    rewards = [s.reward for s in seg[0].steps]
    assert rewards[0] == 0.5 and rewards[1] == 0.0 and rewards[2] == 1.0
