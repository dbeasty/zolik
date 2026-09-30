import numpy as np
import pytest

from zolik_ml.ppo import Step, Tracker, build_batch, gae


def step(v=0.0):
    return Step(np.zeros(2, "f4"), np.zeros((2, 1), "f4"), 0, -0.7, v)


def test_gae_terminal():
    r = [0.0, 0.0, 1.0]
    v = [0.5, 0.6, 0.7]
    adv, ret = gae(r, v, [False, False, True], bootstrap=99.0, gamma=1.0, lam=1.0)
    # lambda=1, gamma=1: advantage is the episode return minus the value.
    np.testing.assert_allclose(adv, [0.5, 0.4, 0.3], atol=1e-6)
    np.testing.assert_allclose(ret, [1.0, 1.0, 1.0], atol=1e-6)


def test_gae_bootstrap_and_lambda():
    r = [1.0, 0.0]
    v = [0.0, 0.5]
    adv, _ = gae(r, v, [False, False], bootstrap=2.0, gamma=0.9, lam=0.5)
    d1 = 0.0 + 0.9 * 2.0 - 0.5
    d0 = 1.0 + 0.9 * 0.5 - 0.0
    np.testing.assert_allclose(adv, [d0 + 0.9 * 0.5 * d1, d1], atol=1e-6)


def test_gae_does_not_cross_episodes():
    adv, _ = gae([1.0, 5.0], [0.0, 0.0], [True, False], bootstrap=0.0, gamma=1.0, lam=1.0)
    np.testing.assert_allclose(adv, [1.0, 5.0])


def test_delayed_reward_goes_to_the_seats_last_decision():
    tr = Tracker()
    a, b = (0, 0, "p0"), (0, 0, "p1")  # two learner seats at one table
    tr.decision(a, step())
    tr.decision(b, step())
    tr.decision(a, step())
    # The hand ends after b's next decision; both rewards arrive together.
    tr.decision(b, step())
    assert tr.event(a, 3.0, True).ret == 3.0
    assert tr.event(b, -3.0, True).ret == -3.0
    segs = tr.harvest()
    assert [len(s.steps) for s in segs] == [2, 2]
    sa, sb = segs
    assert [s.reward for s in sa.steps] == [0.0, 3.0] and sa.steps[-1].done
    assert [s.reward for s in sb.steps] == [0.0, -3.0] and sb.steps[-1].done
    assert sa.bootstrap == 0.0
    assert not tr.open


def test_rewards_accumulate_until_done():
    tr = Tracker()
    k = (1, 2, "p0")
    tr.decision(k, step())
    assert tr.event(k, 0.5, False) is None
    assert tr.event(k, 0.25, False) is None
    stat = tr.event(k, 0.25, True)
    assert stat.ret == 1.0 and stat.decisions == 1
    (seg,) = tr.harvest()
    assert seg.steps[0].reward == 1.0 and seg.steps[0].done


def test_extra_done_at_match_end_is_a_no_op():
    tr = Tracker()
    k = (0, 0, "p0")
    tr.decision(k, step())
    tr.event(k, 2.0, True)
    # The env's extra zero-reward Done for an episode that already closed.
    assert tr.event(k, 0.0, True) is None
    segs = tr.harvest()
    assert len(segs) == 1 and segs[0].steps[0].reward == 2.0
    assert tr.orphan_reward == 0.0
    # And a new episode starts cleanly afterwards.
    tr.decision(k, step())
    assert tr.event(k, -1.0, True).ret == -1.0


def test_reward_before_first_decision_is_not_training_signal():
    tr = Tracker()
    k = (0, 0, "p0")
    # A hand the seat never had a choice in (a walk in the big blind).
    stat = tr.event(k, 1.0, True)
    assert stat.ret == 1.0 and stat.decisions == 0  # still counted in the stats
    assert tr.harvest() == []
    assert tr.orphan_reward == 1.0


def test_open_trajectory_is_bootstrapped_and_keeps_its_tail():
    tr = Tracker()
    k = (0, 0, "p0")
    for v in (0.1, 0.2, 0.3):
        tr.decision(k, step(v))
    tr.event(k, 1.0, False)  # lands on the third decision
    (seg,) = tr.harvest()
    assert len(seg.steps) == 2 and seg.bootstrap == pytest.approx(0.3)
    assert len(tr.open[k]) == 1 and tr.open[k][0].reward == 1.0
    tr.event(k, 0.0, True)
    (seg,) = tr.harvest()
    assert len(seg.steps) == 1 and seg.steps[0].done and seg.steps[0].reward == 1.0


def test_drop_discards_the_unfinished_tail():
    tr = Tracker()
    for t in range(2):
        for _ in range(3):
            tr.decision((t, 0, "p0"), step())
    tr.drop(lambda key: key[0] == 0)
    assert (0, 0, "p0") not in tr.open and (1, 0, "p0") in tr.open
    segs = tr.harvest()
    assert sorted(len(s.steps) for s in segs) == [2, 2]


def test_build_batch_scales_rewards():
    tr = Tracker()
    k = (0, 0, "p0")
    tr.decision(k, step(0.0))
    tr.event(k, 10.0, True)
    b = build_batch(tr.harvest(), gamma=1.0, lam=0.95, reward_scale=0.1)
    assert len(b) == 1 and b.returns[0] == pytest.approx(1.0)
