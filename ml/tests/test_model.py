import torch

from zolik_ml.env import pad_batch
from zolik_ml.model import Policy, entropy, masked_log_softmax


def model():
    torch.manual_seed(1)
    return Policy(6, 3, [8], [5], [4])


def test_padding_is_minus_inf_and_never_sampled():
    m = model()
    import numpy as np

    obs = [np.random.rand(6).astype("f4") for _ in range(3)]
    cands = [np.random.rand(k, 3).astype("f4") for k in (2, 5, 1)]
    o, c, mask = (torch.from_numpy(x) for x in pad_batch(obs, cands))
    assert mask.tolist() == [[1, 1, 0, 0, 0], [1, 1, 1, 1, 1], [1, 0, 0, 0, 0]]
    logits, _ = m(o, c, mask)
    assert torch.isinf(logits[~mask]).all() and (logits[~mask] < 0).all()
    assert torch.isfinite(logits[mask]).all()
    p = torch.softmax(logits, -1)
    assert torch.allclose(p.sum(-1), torch.ones(3))
    assert (p[~mask] == 0).all()
    s = torch.distributions.Categorical(logits=logits).sample((200,))
    assert (s[:, 0] < 2).all() and (s[:, 2] == 0).all()
    # Entropy and log-probs stay finite with padding present.
    assert torch.isfinite(entropy(logits, mask)).all()
    assert entropy(logits, mask)[2].abs() < 1e-6  # one candidate: no choice
    assert torch.isfinite(masked_log_softmax(logits, mask)).all()


def test_padding_does_not_change_real_scores():
    m = model()
    obs = torch.rand(1, 6)
    cands = torch.rand(1, 2, 3)
    padded = torch.cat([cands, torch.rand(1, 3, 3)], 1)
    mask = torch.tensor([[True, True, False, False, False]])
    a, va = m(obs, cands)
    b, vb = m(obs, padded, mask)
    assert torch.allclose(a, b[:, :2]) and torch.allclose(va, vb)


def test_split_first_layer_equals_concatenation():
    """logits() reassociates the first scorer layer; it must equal mlp.go's concat."""
    m = model()
    obs, cands = torch.rand(4, 6), torch.rand(4, 3, 3)
    with torch.no_grad():
        emb = m.embed(obs)
        got = m.logits(emb, cands)
        x = torch.cat([emb.unsqueeze(1).expand(-1, 3, -1), cands], -1)
        for i, layer in enumerate(m.scorer):
            x = layer(torch.relu(x) if i else x)
        assert torch.allclose(got, x.squeeze(-1), atol=1e-6)
