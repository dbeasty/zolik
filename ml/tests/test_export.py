import struct

import numpy as np
import pytest
import torch

from zolik_ml import export
from zolik_ml.model import Policy
from zolik_ml.parity import PARITY_DIR


def test_round_trip(tmp_path):
    torch.manual_seed(0)
    m = Policy(11, 5, [8, 6], [7], [4, 3], game="toy")
    path = export.save(m, tmp_path / "m.bin")
    back = export.load(path)
    assert back.config() == m.config()
    for (n1, p1), (n2, p2) in zip(m.state_dict().items(), back.state_dict().items()):
        assert n1 == n2
        assert torch.equal(p1, p2)
    obs, cands = torch.rand(3, 11), torch.rand(3, 4, 5)
    with torch.no_grad():
        a, b = m(obs, cands), back(obs, cands)
    assert torch.equal(a[0], b[0]) and torch.equal(a[1], b[1])


def test_layout_matches_mlp_go(tmp_path):
    """Header then each layer's W (out x in, row-major) then B, trunk/scorer/value."""
    m = Policy(3, 2, [4], [], [], game="g")
    b = export.to_bytes(m)
    assert b.startswith(b"ZLNET1\n")
    (hlen,) = struct.unpack_from("<I", b, 7)
    header = b[11 : 11 + hlen].decode()
    assert header == '{"game":"g","stateDim":3,"candDim":2,"trunk":[[3,4]],"scorer":[[6,1]],"value":[[4,1]]}'
    body = np.frombuffer(b[11 + hlen :], dtype="<f4")
    w0 = m.trunk[0].weight.detach().numpy()
    assert np.array_equal(body[:12].reshape(4, 3), w0)
    assert np.array_equal(body[12:16], m.trunk[0].bias.detach().numpy())
    assert len(body) == 12 + 4 + 6 + 1 + 4 + 1


def test_rejects_garbage():
    with pytest.raises(ValueError):
        export.from_bytes(b"nope")
    m = Policy(3, 2, [4], [], [])
    with pytest.raises(ValueError):
        export.from_bytes(export.to_bytes(m) + b"\0\0\0\0")


def test_committed_parity_fixture_reproduces():
    """The fixture the Go parity test reads is what this code computes."""
    import json

    m = export.load(PARITY_DIR / "model.bin")
    cases = json.loads((PARITY_DIR / "cases.json").read_text())
    for c in cases:
        with torch.no_grad():
            logits, value = m(torch.tensor([c["obs"]]), torch.tensor([c["cands"]]))
        np.testing.assert_allclose(logits[0].numpy(), c["logits"], atol=1e-6)
        assert abs(float(value[0]) - c["value"]) < 1e-6
