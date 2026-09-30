from pathlib import Path

import yaml

from zolik_ml.league import League, assign_variations

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


def test_zolik_plans():
    lg = league("zolik")
    seen, seats = set(), set()
    for _ in range(400):
        t = lg.sample("zolik_classic+floor35")
        assert t.plan.count("learner") >= 1
        seats.add(len(t.plan))
        seen.add(t.label)
        if t.label != "self":
            assert t.plan.count("learner") == 1  # no partnerships in Žolíky
    assert seats == {2, 3, 4}
    assert seen == {"self", "hard", "medium", "easy"}


def test_variation_mix():
    cfg = yaml.safe_load((CONFIGS / "zolik.yaml").read_text())
    v = assign_variations(cfg, 8)
    assert v.count("zolik_classic") == 6 and v.count("zolik_classic+floor35") == 2
    assert set(v[:2]) == {"zolik_classic", "zolik_classic+floor35"}  # interleaved
    assert assign_variations(cfg, 1) == ["zolik_classic"]
    assert "continental" not in assign_variations(cfg, 16)
    # Without a mix: the config's variation, and Samba on every other process when asked.
    canasta = yaml.safe_load((CONFIGS / "canasta.yaml").read_text())
    assert assign_variations(canasta, 4) == ["classic"] * 4
    canasta["league"]["samba"] = True
    assert assign_variations(canasta, 4) == ["classic", "samba", "classic", "samba"]
