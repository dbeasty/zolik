from pathlib import Path

import yaml

from zolik_ml.league import League

CONFIGS = Path(__file__).resolve().parent.parent / "configs"


def league(game):
    return League(yaml.safe_load((CONFIGS / f"{game}.yaml").read_text()), seed=3)


def test_holdem_plans():
    lg = league("holdem")
    seen = set()
    for _ in range(300):
        t = lg.sample()
        assert 2 <= len(t.plan) <= 6 and "learner" in t.plan
        assert not any(p.startswith("net:") for p in t.plan)  # no pool yet
        seen.add(t.label)
    assert "self" in seen and "station" in seen and "ckpt" not in seen
    lg.add("/tmp/a.bin")
    lg.add("/tmp/b.bin")
    plans = [lg.sample() for _ in range(300)]
    ckpt = [p for t in plans for p in t.plan if p.startswith("net:")]
    assert ckpt and all(p.endswith("@1") for p in ckpt)
    assert sum("b.bin" in p for p in ckpt) > sum("a.bin" in p for p in ckpt)  # recent favoured


def test_canasta_partners():
    lg = league("canasta")
    kinds = set()
    for _ in range(300):
        t = lg.sample()
        assert len(t.plan) in (2, 4)
        if len(t.plan) == 4 and t.label != "self":
            me = t.plan.index("learner")
            partner = t.plan[(me + 2) % 4]
            kinds.add(partner)
            assert t.plan[(me + 1) % 4] != "learner" and t.plan[(me + 3) % 4] != "learner"
    assert kinds == {"learner", "hard"}


def test_samba_seats():
    lg = league("canasta")
    assert {len(lg.sample("samba").plan) for _ in range(200)} == {2, 4, 6}
