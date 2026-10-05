# AI for imperfect-information card games: a research survey mapped to our bots

*Written 2026-10-02. Research only: nothing in the codebase was changed. Each citation says how
deeply it was checked: **[read]** means the primary text was read (the relevant sections, or all of
it); **[abstract]** means only the official abstract or proceedings page was checked;
**[secondary]** means a detail comes from reputable secondary summaries and should be re-checked
before anyone relies on it.*

---

## 0. Our context

**Server and games.** A Go card-game server runs:

- **Texas Hold'em**: 2 to 9 seats, no-limit. Bench matches are 15 hands with 50 BB stacks.
- **Canasta**: 2 to 6 seats, with partnerships at 4 and 6 seats, in Classic and Samba variations.
- **Žolíky**: Czech rummy played with two decks plus jokers. A player must meet a contract or point
  minimum to go down, and needs a clean run.
- **Mariáš**: a trick-taking game. Its bot already uses PIMC (perfect-information Monte Carlo) over a
  double-dummy alpha-beta solver (`server/internal/marias/sampling.go` and `server/internal/tricks/search`).

**Two kinds of bot.**

1. **Rule-based "Hard" heuristics.**
   - Žolíky: meld search and outs counting (`server/internal/ai`).
   - Hold'em: the Chen formula before the flop, then Monte Carlo equity against pot odds, with bluff
     knobs (`server/internal/holdem/bot.go`). Practice styles live in `styles.go`.
   - Canasta: rules for the pile, wild cards and going out (`server/internal/canasta/bot.go`,
     including `worthGoingOut`).
2. **Trained models.** A candidate-scoring actor-critic MLP:
   - The state comes from the seat's legal view (`Encode`). Each legal move gets its own feature
     vector (`Candidates`), so the network copes with a different number of moves every turn
     (`server/internal/learn`).
   - Training is PPO self-play in `ml/` (`zolik_ml/ppo.py`, `league.py`). The league holds the
     heuristics, the practice styles (closer, maniac, rock, station, solid, riverbluffer) and past
     checkpoints.
   - Inference is pure Go and takes under about 2 ms per move. The server allows 5 s per move.
   - Evaluation is `server/cmd/gamebench`: a duplicate bench on held-out seeds (≥ 1,000,000), with
     seats swapped.

**Known weaknesses.**

- (W1) The Canasta model rarely closes, so fast closers catch it holding big hands.
- (W2) The Žolíky model sometimes misses a clean run, and its discards feed the next player.
- (W3) The Hold'em model shoves weak hands too often against solid callers.
- (W4) Rule-based Canasta Hard is reluctant to go out at partnership tables.

**Experiments in progress.**

- Opponent card inference from pickups, discards and lay-offs. This is the game-agnostic
  `server/internal/cardinfer` package on the `claude/card-inference` branch: exact counting plus
  likelihood weights, fitted by iterative proportional fitting, giving `Hold` and `Want` per seat.
- PIMC-style determinized search using the network's value head.
- Exact endgame search.

---

## 1. Executive summary: top ideas ranked by expected impact

Effort scale: **S** is days, **M** is one to two weeks, **L** is a month or more. "Impact" is my
estimate of the gain on the duplicate bench, weighted by how directly the idea attacks W1 to W4.

| # | Idea | Tag | Games | Impact | Effort | Main sources |
|---|---|---|---|---|---|---|
| 1 | **Perfect-information critic.** The value head sees every hand during training; the actor sees only the legal view, so serving cost is unchanged. | **[AI]** | Canasta, Žolíky, Hold'em | High | S–M | PerfectDou (NeurIPS 2022); Suphx oracle guiding (2020) |
| 2 | **Global (match-level) reward prediction.** A small model predicts the match result from the score state. Each deal's reward becomes the change in predicted match equity, which should teach the model *when* closing is worth it (W1). | **[AI]** | Canasta (most), Žolíky, Hold'em 15-hand matches | High for W1 | S | Suphx |
| 3 | **Inference-weighted determinization.** Sample worlds from `cardinfer`, then re-weight them by how likely each world makes the opponents' actual moves under our own policy network (policy-based inference). This replaces uniform PIMC sampling. | **[AI] [Hard]** | Canasta, Žolíky (also Mariáš) | High (conditional on PIMC shipping) | M | Kermit (IJCAI 2009); Solinas et al. (AAAI 2019); Rebstock et al. (CoG 2019) |
| 4 | **Auxiliary hidden-card head.** Predict each opponent's hand (52+joker slots per seat) from the trunk. It shapes the representation, and its output can feed discard-danger features and world sampling. | **[AI]** | Žolíky (W2), Canasta, Hold'em (range) | Medium–High | S | Gin Rummy hand estimation (EAAI 2021); IRumAI probe (2026); Solinas et al. 2019 |
| 5 | **Equilibrium bluff and call frequencies tied to bet size, plus push/fold charts** for rule-based Hold'em Hard. The same numbers become candidate features and a "nash-pusher" league opponent. | **[Hard] [AI]** | Hold'em (W3) | Medium–High | S | Indifference maths (Chen & Ankenman 2006, a book); Miltersen & Sørensen (AAMAS 2007); Ganzfried & Sandholm (AAMAS 2008) |
| 6 | **Explicit going-out / closing decision search.** A short determinized rollout over "go out now" vs "keep building", scored by match equity (needs #2). It works as a rule for Canasta Hard (W4) and as a search override for the model (W1). | **[Hard] [AI]** | Canasta | High for W1/W4 | M | PIMC literature; Suphx pMCPA; Gin Rummy knock-early findings (2026) |
| 7 | **Distance-to-go-out shaping for rummy.** A potential function from the existing meld search (fewest cards or turns to go out), applied as potential-based shaping, as an oracle reward computed on the true deal (PerfectDou), or both. | **[AI]** | Žolíky (W2), Canasta | Medium (evidence is mixed) | S | PerfectDou node reward; IRumAI; Kelidari et al. (AIIDE 2026) as counter-evidence |
| 8 | **League hygiene.** Prioritised fictitious self-play (PFSP) weighting towards the opponents we lose to, exploiter agents aimed at the main agent, keep-the-best checkpoint selection, and value-target clipping for Hold'em. | **[AI]** | All, especially Hold'em (W3) and Canasta vs closers (W1) | Medium | S | AlphaStar (Nature 2019); AlphaHoldem (AAAI 2022); Kelidari et al. 2026 |
| 9 | **Exact endgame search** once the stock is low or hands are nearly known, with the value head at the leaves. Use it for the last few turns only, where PIMC's known weaknesses shrink. | **[AI] [Hard]** | Žolíky (clean-run finish, W2), Canasta | Medium | M–L | Long et al. (AAAI 2010); αμ (Cazenave & Ventos); GIB |
| 10 | **Deep Monte Carlo (DouZero) as an alternative to PPO.** Probably *not* worth a full switch (see §7, E6). A cheap variant is worth it: an MC-return Q head next to the actor, used as a search leaf evaluator. | **[AI]** | Žolíky, Canasta | Low–Medium | M | DouZero (ICML 2021); PerfectDou; DanZero (2022) |

**What not to expect.**

- Full CFR, ReBeL or Student-of-Games machinery is the state of the art for heads-up poker. It does
  not carry over to our multi-player rummy games: the public belief state is far too large there, and
  the guarantees hold only for two-player zero-sum games (§2, §8).
- For Hold'em, the cheap equilibrium *facts* (bet-size-to-bluff ratios, push/fold) give most of the
  practical value.

---

## 2. Poker: equilibrium computation, search and exploitation

### 2.1 CFR, CFR+, MCCFR, Deep CFR

- **Zinkevich, Johanson, Bowling, Piccione (2007). *Regret Minimization in Games with Incomplete
  Information*. NIPS 2007.** [citation only, not re-fetched]
  - **Summary:** Counterfactual regret minimisation splits overall regret into per-information-set
    regrets and minimises each locally with regret matching. In two-player zero-sum games, the
    *average* strategy converges to a Nash equilibrium. It is the basis of every serious poker bot
    since.
- **Tammelin (2014). *Solving Large Imperfect Information Games Using CFR+*. arXiv:1407.5042.**
  [read, header]
  - **Summary:** CFR+ floors cumulative regrets at zero ("regret matching+") and weights later
    iterations more heavily. It is typically an order of magnitude faster than CFR. CFR+ was used to
    essentially solve heads-up limit hold'em (Bowling, Burch, Johanson, Tammelin, *Science* 2015)
    [citation only].
- **Lanctot, Waugh, Zinkevich, Bowling (2009). *Monte Carlo Sampling for Regret Minimization in
  Extensive Games*. NIPS 2009.**
  [abstract](https://papers.nips.cc/paper/3713-monte-carlo-sampling-for-regret-minimization-in-extensive-games)
  - **Summary:** MCCFR samples part of the tree each iteration (outcome or external sampling) and
    keeps regret bounds that hold with high probability. Libratus and Pluribus use it to compute
    their blueprints.
- **Brown, Lerer, Gross, Sandholm (2019). *Deep Counterfactual Regret Minimization*. ICML 2019.**
  [arXiv:1811.00164](https://arxiv.org/abs/1811.00164) [read, intro]
  - **Summary:** Deep CFR replaces the regret tables with neural networks trained on sampled
    traversals, so no hand-built abstraction is needed. It was the first non-tabular CFR to do well
    in large poker games, and the paper reports it beating NFSP.

**What applies to us.**

- CFR needs a two-player zero-sum framing to give guarantees. Our Hold'em tables are usually
  multi-way and our matches are short, so a full CFR solver is a poor return on effort.
- Two things do apply:
  - CFR+ or MCCFR can compute **small offline tables** for subgames we can define exactly. The
    heads-up or three-way jam/fold game at ≤ 15 BB is one; a river spot with a polarised range is
    another. Those tables become rules in Hold'em Hard (E5).
  - "Average strategy, not current strategy" is the key lesson for our PPO league (see NFSP, below).

### 2.2 NFSP

- **Heinrich & Silver (2016). *Deep Reinforcement Learning from Self-Play in Imperfect-Information
  Games*. arXiv:1603.01121.** [read, abstract and method]
  - **Summary:** NFSP trains two networks. One is a best response learned by RL. The other is an
    *average policy* learned by supervised learning from a reservoir of the agent's own past
    best-response actions; the agent plays a mix of the two.
  - On Leduc, NFSP approached a Nash equilibrium, whereas ordinary RL methods diverged. On limit
    hold'em it came close to strong hand-built programs.
- **What applies to us.** Plain PPO self-play chases its own latest policy and can cycle; this is one
  reading of W3, where the model over-shoves because recent opponents folded too much. We already
  approximate fictitious play by keeping past checkpoints in the league. Two cheap steps go further:
  - Make checkpoint sampling closer to a uniform average over history, not just recent checkpoints.
  - Optionally, distil an average-policy network for Hold'em and serve that.

### 2.3 DeepStack and Libratus (heads-up no-limit)

- **Moravčík, Schmid, Burch, Lisý, Morrill, Bard, Davis, Waugh, Johanson, Bowling (2017).
  *DeepStack: Expert-level artificial intelligence in heads-up no-limit poker*. Science
  356(6337):508–513.** [arXiv:1701.01724](https://arxiv.org/abs/1701.01724) [read, intro]
  - **Summary:** DeepStack re-solves the game at every decision ("continual re-solving"). The search
    is depth-limited, and a value network trained on random poker situations estimates the leaves.
    It defeated professionals with statistical significance over 44,000 hands.
- **Brown & Sandholm (2018). *Superhuman AI for heads-up no-limit poker: Libratus beats top
  professionals*. Science 359:418–424.**
  [doi](https://www.science.org/doi/10.1126/science.aao1733) [abstract]
  - **Summary:** Libratus has three parts: an MCCFR "blueprint" for the whole game, nested safe
    subgame solving during play, and an overnight self-improver that patched blueprint holes the
    pros had found.
- **What applies to us.** The sound techniques (safe subgame solving, range-vs-range values) are
  heavy. The transferable ideas are:
  - Reason about **ranges**, not hands. Hold'em Hard already estimates equity against a range via
    the `bluffShare` dial; the auxiliary range head in E2 is the learned version.
  - **Depth-limited search with a learned value at the leaves.** This is exactly our "PIMC with the
    value head" experiment. In poker it is only sound with range-aware leaves; in rummy it is a
    heuristic.

### 2.4 Pluribus (multi-player)

- **Brown & Sandholm (2019). *Superhuman AI for multiplayer poker*. Science 365:885–890.**
  [doi](https://www.science.org/doi/10.1126/science.aay2400) [abstract; details from
  secondary sources]
  - **Summary:** In six-player no-limit Hold'em, Pluribus computed an MCCFR blueprint offline. Online
    it used depth-limited search in which each player, at the leaves, may switch to one of several
    continuation strategies. These are reported as the blueprint plus versions biased towards
    folding, calling and raising [secondary].
  - The blueprint reportedly cost about 12,400 CPU core-hours [secondary]. Pluribus does not adapt
    to individual opponents.
- **What applies to us.**
  - **This is the closest published system to our Hold'em setting (multi-way).** It shows that a
    non-adaptive, roughly balanced strategy beats strong humans multi-way. Exploiting opponents is
    optional, not required.
  - The **"opponents may switch to fold/call/raise-biased continuations"** trick is cheap to borrow.
    In any Hold'em rollout or search we add, evaluate leaves against a small set of opponent styles
    (the league's rock, station and maniac already *are* such biased continuations) and take the
    *minimum*, not the average. That guards against W3: a shove that only works if opponents fold
    gets scored against the station continuation too.

### 2.5 ReBeL and Student of Games

- **Brown, Bakhtin, Lerer, Gong (2020). *Combining Deep Reinforcement Learning and Search for
  Imperfect-Information Games* (ReBeL). NeurIPS 2020.**
  [arXiv:2007.13544](https://arxiv.org/abs/2007.13544) [read, intro and limitations]
  - **Summary:** ReBeL turns the game into a game over *public belief states* (a probability
    distribution over every player's private information) and runs AlphaZero-style RL plus search
    there. It provably converges in two-player zero-sum games and is superhuman in heads-up no-limit.
  - The authors state the main limitation: the value network's input grows linearly with the number
    of information sets in a public state.
- **Schmid et al. (2023). *Student of Games: A unified learning algorithm for both perfect and
  imperfect information games*. Science Advances 9(46).**
  [doi](https://www.science.org/doi/10.1126/sciadv.adg3256),
  [arXiv:2112.03178](https://arxiv.org/abs/2112.03178) [read, abstract and intro]
  - **Summary:** It combines growing-tree CFR search with learned counterfactual value-and-policy
    networks. It is strong at chess and Go, beats the strongest open heads-up poker agent, and beats
    the previous best agent at Scotland Yard.
- **What applies to us.** In Canasta or Žolíky, an opponent's possible hands number in the billions.
  A belief state cannot be the network input, so ReBeL and Student of Games are **out of scope** for
  the rummy games. For heads-up Hold'em they are feasible but costly. The heuristic and PPO route,
  plus equilibrium facts, is a better return. I recommend against both (§8).

### 2.6 End-to-end RL for no-limit: AlphaHoldem

- **Zhao, Yan, Li, Li, Xing (2022). *AlphaHoldem: High-Performance Artificial Intelligence for
  Heads-Up No-Limit Poker via End-to-End Reinforcement Learning*. AAAI 2022.**
  [pdf](https://cdn.aaai.org/ojs/20394/20394-13-24407-1-2-20220628.pdf) [read, method]
  - **Summary:** This is the system nearest to ours: PPO self-play, no CFR, about 2.9 ms per
    decision, and it beat Slumbot and DeepStack.
  - Plain PPO was unstable in no-limit for two reasons: large-ratio updates with negative advantages
    caused high variance, and the value targets were huge because of all-ins and bluffs. Their
    **Trinal-Clip PPO** adds an extra clip on the ratio when the advantage is negative, and clips
    the return used as the value target to the chips actually committed.
  - They also pick opponents with "K-best" self-play.
- **What applies to us, directly.** Clip value targets and add the negative-advantage ratio clip in
  `ml/zolik_ml/ppo.py` for Hold'em. One plausible cause of W3 is the value head being pulled around
  by rare huge pots, which makes shove candidates look better than they are. This is a few lines of
  code (E9).

### 2.7 Push/fold equilibria for short stacks

- **Miltersen & Sørensen (2007). *A near-optimal strategy for a heads-up no-limit Texas Hold'em
  poker tournament*. AAMAS 2007.** [abstract](https://dl.acm.org/doi/10.1145/1329125.1329357)
  - **Summary:** They computed exact equilibria of the heads-up tournament game in which the only
    actions are jam (all-in) or fold. They showed these restricted strategies are close to
    equilibrium in the unrestricted game at the stack depths studied.
- **Ganzfried & Sandholm (2008). *Computing an approximate jam/fold equilibrium for 3-player
  no-limit Texas hold'em tournaments*. AAMAS 2008.** [citation verified, text not read]
  - **Summary:** This extends jam/fold to three players with tournament payoffs (the Independent
    Chip Model).
- **What applies to us.**
  - At short effective stacks (roughly ≤ 10–15 BB), jam/fold charts are the near-equilibrium
    strategy, both for which hands to shove and which hands to call a shove with. They are small
    tables, keyed by seat position and stack in BB, and can be computed with CFR+ offline in
    seconds.
  - Our 15-hand, 50 BB matches produce short stacks after early losses. W3 is precisely a shoving
    range that is too wide for how often opponents call.
  - Use them in three places:
    1. **Hold'em Hard:** rules below a stack threshold.
    2. **Model:** a candidate feature on the all-in candidate, giving the equilibrium shove EV for
       this hand and stack (or simply "is in the Nash shove range").
    3. **League:** a "pusher" opponent that plays the charts exactly.
  - Heads-up and three-way charts are published in this literature. Beyond three-way I could not
    verify a peer-reviewed source; compute those ourselves (E5).

### 2.8 Equilibrium bluffing frequency and bet size

The result is elementary game theory, from von Neumann & Morgenstern's poker model onwards. It is
presented systematically in **Chen & Ankenman (2006), *The Mathematics of Poker* (ConJelCo; a book,
not peer-reviewed)**. I derive it here rather than quote it.

**Setup.** A river spot with a polarised bettor (strong hands and pure bluffs only) and a bluff-catcher.
The pot is *P* and the bet is *B*.

- **Bettor's bluff fraction.** The caller risks *B* to win *P + B*, and is indifferent when the
  fraction of bluffs in the betting range is α = B / (P + 2B).
  - Half-pot bet: α = 1/4 (one bluff for every three value bets).
  - Pot-size bet: α = 1/3.
  - Twice the pot: α = 2/5.
- **Caller's defence frequency.** The bluffer risks *B* to win *P*, and is indifferent when the
  caller continues with probability P / (P + B). This is the "minimum defence frequency".
  - Half-pot: 2/3.
  - Pot-size: 1/2.
  - Twice the pot: 1/3.

**What applies to us.** Hold'em Hard currently sets bluffs with fixed knobs (`bluff`, `semiBluff`,
`bluffRaise`) and reads opponents' bets through `bluffShare`. Both should be **functions of B/P**:

- Bluff so that bluffs ≈ α(B/P) of the value bets it makes in that spot.
- Against an unknown opponent, call down with at least the top P/(P+B) of its bluff-catching range.
  Deviate from this only on evidence (§2.9).

This removes a whole class of exploitable tells, such as bluffing as often with a two-times-pot
overbet as with a third-pot bet. Multi-way, the defence share is split among the remaining callers,
and bluffing should drop sharply as the number of opponents grows. Hold'em Hard already moves its
pre-flop threshold with the number of opponents; extend that to its bluff rate (E5).

### 2.9 Opponent modelling and exploitation

- **Johanson, Zinkevich, Bowling (2007). *Computing Robust Counter-Strategies* (restricted Nash
  response). NIPS 2007.** [abstract](https://papers.nips.cc/paper/2007/hash/6e7b33fdea3adc80ebd648fffb665bb8-Abstract.html)
  - **Summary:** Solve a modified game in which the opponent must play its modelled strategy with
    probability *p* and may play anything otherwise. Sweeping *p* traces a curve trading
    exploitation against exploitability. The resulting counter-strategies are much more robust than
    a pure best response.
- **Johanson & Bowling (2009). *Data Biased Robust Counter Strategies*. AISTATS 2009.**
  [abstract](https://mlanthology.org/aistats/2009/johanson2009aistats-data/)
  - **Summary:** The same idea, but the model is trusted *per information set*, in proportion to
    how much data was observed there. That avoids overfitting to thin or stale data.
- **Ganzfried & Sandholm (2011). *Game theory-based opponent modeling in large imperfect-information
  games*. AAMAS 2011.** [citation only]
  - **Summary:** It starts from an equilibrium and moves the opponent model away from it only as
    observed action frequencies deviate.
- **Ganzfried & Sandholm (2012). *Safe Opponent Exploitation*. ACM EC 2012** (journal version ACM
  TEAC 2015). [pdf](http://www.cs.cmu.edu/~sandholm/safeExploitation.ec12.pdf) [abstract]
  - **Summary:** It characterises when an agent can deviate from equilibrium to exploit someone
    while still guaranteeing at least the game value in expectation. The key is to risk only what
    the opponent has already "gifted" through earlier mistakes.
- **Burch, Schmid, Moravčík, Morrill, Bowling (2018). *AIVAT: A New Variance Reduction Technique for
  Agent Evaluation in Imperfect Information Games*. AAAI 2018.**
  [abstract](https://ojs.aaai.org/index.php/AAAI/article/view/11481)
  - **Summary:** An unbiased evaluator. It subtracts control variates built from a heuristic value
    function and the known strategy of our own agent. In a human-vs-machine match it cut the
    standard deviation by about 85%, which means about 44 times fewer games. Relevant to how we
    *measure* (§7).
- **What applies to us.**
  - Our styles (maniac, rock, station, riverbluffer) are deliberately exploitable caricatures,
    which is fine *for training*. W3 shows the danger the papers warn about: a policy that learned
    to exploit folders keeps exploiting when the table is full of solid callers.
  - Two concrete moves:
    1. Track opponent statistics *per opponent seat during a match*, such as fold-to-shove and VPIP
       (how often a player voluntarily puts money in pre-flop), with a count. Feed them to the model
       and the heuristic **shrunk towards the population prior by sample size**, as in
       data-biased response.
    2. In Hold'em Hard, deviate from the equilibrium ranges of §2.7–2.8 only in proportion to that
       shrunk evidence.
  - A safe-exploitation budget (risk only what you are already up against that opponent this match)
    is a simple, defensible rule.

---

## 3. Determinization and search under hidden information

### 3.1 PIMC and its failure modes

- **Frank & Basin (1998). *Search in games with incomplete information: a case study using Bridge
  card play*. Artificial Intelligence 100:87–123.** [citation only; concepts verified via Long et al.]
  - **Summary:** Named the two errors of determinized search:
    - **Strategy fusion:** in each sampled world the searcher assumes it can play differently,
      though it cannot tell the worlds apart.
    - **Non-locality:** a node's true value depends on parts of the tree outside its subtree,
      because the opponent steers play using private information.
- **Ginsberg (2001). *GIB: Imperfect Information in a Computationally Challenging Game*. JAIR
  14:303–358.** [abstract](https://mlanthology.org/jair/2001/ginsberg2001jair-gib/)
  - **Summary:** The first strong bridge program. It dealt many consistent worlds, solved each
    double-dummy with fast alpha-beta (partition search), and picked the card with the best
    average. It is the template our Mariáš bot follows.
  - Critics, from Russell and Norvig onwards, described this as "averaging over clairvoyance"
    (quoted in Cowling et al. 2012). The point is that it never plays to gather information or to
    hide it.
- **Long, Sturtevant, Buro, Furtak (2010). *Understanding the Success of Perfect Information Monte
  Carlo Sampling in Game Tree Search*. AAAI 2010, pp. 134–140.**
  [ojs](https://ojs.aaai.org/index.php/AAAI/article/view/7562) [read]
  - **Summary:** On synthetic trees, three measurable properties predict how well PIMC does against
    an equilibrium player:
    - **Leaf correlation:** the probability that sibling terminal nodes share a payoff.
    - **Bias:** how much the game favours one side.
    - **Disambiguation factor:** how fast information sets shrink as play goes on.
  - PIMC is worst when **leaf correlation is low**, that is, when the outcome can still swing on
    the last moves. It improves sharply as disambiguation rises. Measured Skat and Hearts trees had
    high correlation, which explains PIMC's success there.
- **What applies to us. This paper is the most important caution for our PIMC experiment.**
  - **Mariáš** looks like Skat (trick-taking, every play reveals a card, high disambiguation). PIMC
    is the right tool there, as the existing bot shows.
  - **Canasta and Žolíky** look more like poker: an opponent's hand is never revealed until it
    goes out. The discards, pickups and lay-offs that *are* public reveal only part of it, so
    disambiguation is low for most of a deal.
  - Leaf correlation is also low near the end. Whether the opponent goes out next turn, catching us
    with a big hand, is precisely an outcome that swings late (W1).
  - Expect PIMC to help **late in a deal** (stock low, opponents' hands small and partly known) and
    on narrow tactical questions. Expect less from it early.
  - The cheapest check is to **measure the three properties ourselves** on sampled Canasta and
    Žolíky positions, as the paper did for Skat. We have the engines and gamebench, so this is a
    one-day script before committing to PIMC (part of E1).

### 3.2 ISMCTS

- **Cowling, Powley, Whitehouse (2012). *Information Set Monte Carlo Tree Search*. IEEE Trans.
  Computational Intelligence and AI in Games 4(2):120–143.**
  [pdf](https://eprints.whiterose.ac.uk/id/eprint/75048/1/CowlingPowleyWhitehouse2012.pdf) [read,
  intro and results summary]
  - **Summary:** ISMCTS builds a single search tree over the searcher's *information sets*, using a
    new determinization on each iteration. This removes strategy fusion for the searcher and
    spreads the compute budget better.
  - Across three games, ISMCTS beat determinized UCT where deep search or strategy fusion mattered.
    In **Dou Di Zhu** (a shedding card game) the two were on a par.
  - The authors also report that, in a simplified Dou Di Zhu, a *perfect* opponent model gave only
    a small gain over uniform beliefs.
- **What applies to us.** That Dou Di Zhu result is a useful prior for our rummy games. Better
  beliefs matter, but maybe less than in Skat. Measure first (E1).
  - ISMCTS is a reasonable search shell for the rummy games if PIMC is limited by strategy fusion.
    Our candidate generator already gives a manageable branching factor per information set.
  - Kelidari et al. (2026, §4.4) found fair determinized ISMCTS *weaker* than a small PPO agent in
    Gin Rummy, at 1–2 s per move. That is a clear warning against expecting search alone to beat
    the network.

### 3.3 Skat: Kermit and inference-guided sampling

- **Buro, Long, Furtak, Sturtevant (2009). *Improving State Evaluation, Inference, and Search in
  Trick-Based Card Games*. IJCAI 2009, pp. 1407–1413.**
  [pdf](https://www.ijcai.org/Proceedings/09/Papers/236.pdf) [read, §4]
  - **Summary:** This is Kermit, the first expert-level Skat program. It biases PIMC's sampled worlds
    with P(world | observed bids and contract), estimated offline from game data over *features* of
    a hand (suit lengths, high cards) and treated as independent, rather than over whole hands.
  - The authors note that a deterministic bot's own P(move | world) is 0 or 1, so using it in Bayes'
    rule is brittle against other players. They learn the probabilities from data instead.
  - Adding inference made Kermit significantly stronger. The defenders' inference of the soloist's
    hand helped most.
- **Solinas, Rebstock, Buro (2019). *Improving Search with Supervised Learning in Trick-Based Card
  Games*. AAAI 2019, 33(01):1158–1165.** [arXiv:1903.09604](https://arxiv.org/abs/1903.09604) [read,
  method and results]
  - **Summary:** A network trained on about 20 million human games predicts *where each individual
    card is*. The product of those probabilities weights the sampled worlds for PIMC.
  - It significantly beat Kermit's inference in suit and null games. Including the move history as
    input helped substantially.
- **Rebstock, Solinas, Buro, Sturtevant (2019). *Policy Based Inference in Trick-Taking Card
  Games*. IEEE CoG 2019.** [arXiv:1905.10911](https://arxiv.org/abs/1905.10911) [read, method and
  conclusion]
  - **Summary:** It weights each sampled world by its *reach probability*: the product, over every
    opponent action so far, of the probability that a learned opponent policy would have taken that
    action in that world. It beat both earlier inference methods at identifying the true world and
    in tournament points (for example, +2.32 tournament points per game over card-location
    inference in suit games).
- **Solinas, Rebstock, Sturtevant, Buro (2023). *History Filtering in Imperfect Information Games:
  Algorithms and Complexity*. NeurIPS 2023.**
  [pdf](https://proceedings.neurips.cc/paper_files/paper/2023/file/87ee1bbac4635e7c948f3eea83c1f262-Paper-Conference.pdf)
  [read, abstract]
  - **Summary:** Even generating a *single* world consistent with the public history is generally
    intractable. The paper gives conditions under which enumeration is efficient, and an MCMC
    sampler for trick-taking games.
- **What applies to us. This is the core of E1.**
  1. Our network already outputs a **stochastic** policy over candidates. Unlike Kermit's
     deterministic bot, it directly gives P(action | world) for any sampled world. Policy-based
     inference is therefore nearly free: replay the opponent's observed public actions in each
     sampled world, evaluate the net from *their* seat, and multiply the probabilities.
  2. `cardinfer` gives a cheap, fair proposal distribution (exact counting plus likelihood
     weights). Use it to *propose* worlds, then importance-weight them with policy-based inference
     (with the proposal's own probability divided out).
  3. Consistency matters. A sampled world must respect every hard constraint: cards known held
     after a pickup, cards laid down, the multiset counts of a two-deck pack. `cardinfer`'s
     `Held` already handles this. History filtering warns that rejection sampling can stall when
     constraints are tight, so cap the retries and fall back to the `cardinfer` expectation.

### 3.4 αμ search

- **Cazenave & Ventos. *The αμ Search Algorithm for the Game of Bridge*.**
  [arXiv:1911.07960](https://arxiv.org/abs/1911.07960) (2019). Published in *Monte Carlo Search,
  First Workshop MCS 2020 (held with IJCAI 2020)*, Springer CCIS, 2021. [read, abstract and §2–3]
  - **Summary:** αμ assumes the defenders play with perfect information. It searches M of our own
    moves ahead, carrying a vector of outcomes (one per world) and keeping Pareto fronts, so that
    one strategy is chosen across all worlds. With enough time this fixes strategy fusion and
    non-locality, and it beat PIMC at bridge card play.
- **What applies to us.** It is mainly for Mariáš: an upgrade path for `tricks/search` when PIMC's
  strategy fusion shows up in endgames. For rummy games the outcome vectors are too wide early on.
  At the very end, though, a small αμ with M = 1–2 over "go out now / hold / which discard" is
  exactly the decision that matters for W1 and W4 (E7).

---

## 4. Large card games with combinatorial actions (most relevant to our rummy games)

### 4.1 DouZero (Dou Dizhu)

- **Zha, Xie, Ma, Zhang, Lian, Hu, Liu (2021). *DouZero: Mastering DouDizhu with Self-Play Deep
  Reinforcement Learning*. ICML 2021.** [arXiv:2106.06135](https://arxiv.org/abs/2106.06135) [read,
  §3–4]
- **Summary.**
  - Deep Monte Carlo (DMC) is every-visit Monte Carlo Q-learning with a neural network: Q(s, a) is
    regressed onto the *actual* episode return with mean squared error, with epsilon-greedy
    exploration and many parallel actors.
  - Cards in the state and in the action are encoded as 4×15 count matrices. An LSTM summarises the
    move history, and a 6×512 MLP scores each (state, legal action) pair.
  - The authors argue DMC suits Dou Dizhu for three reasons:
    1. Episodes are short and terminal-reward only, so there is no bootstrapping bias.
    2. Unlike a DQN-style max over actions, it does not overestimate values.
    3. Unlike a softmax over a fixed output layer, action features let it generalise to rarely seen
       actions.
  - It ranked first on the Botzone leaderboard out of 344 agents, after days of training on one
    four-GPU server.
- **What applies to us.**
  - **Our design already has DouZero's key property.** We score each legal candidate from its own
    feature vector. DouZero's main argument against policy-gradient methods was the classifier-style
    output layer, and that does not apply to us.
  - What we could borrow:
    - **Count-matrix action features for melds.** For Žolíky and Canasta, encode the cards a
      candidate plays as a rank × suit (+ joker) count matrix, in the same slot layout
      `cardinfer` uses, so that "3♥4♥5♥" and "4♥5♥6♥" share structure.
    - **An LSTM or GRU over recent public moves.** Discard and pickup history is exactly what
      inference needs.
  - For whether to switch algorithm, see E6.

### 4.2 PerfectDou

- **Yang, Liu, Hong, Zhang, Fang, Zeng, Lin (2022). *PerfectDou: Dominating DouDizhu with Perfect
  Information Distillation*. NeurIPS 2022.** [arXiv:2203.16406](https://arxiv.org/abs/2203.16406)
  [read, §3–4 and §6.3]
- **Summary.**
  - "Perfect-training-imperfect-execution" (PTIE) is an actor-critic in which the **critic receives
    the full deal** (all hands) and the **actor receives only the player's view**. The critic exists
    only at training time, so play never uses hidden information.
  - The actor scores each legal action by concatenating a state embedding with an action embedding.
    That is *our architecture*. Training is PPO with GAE.
  - It also adds a per-step **oracle reward**: the change in each side's "minimum number of plays
    to empty the hand", computed by dynamic programming on the true deal.
  - It beat DouZero with about one tenth of the samples.
- **Ablation (Table 4, against DouZero at about 1e9 samples).**
  - The full system: 0.732 win percentage.
  - Imperfect-information critic: 0.717.
  - No node reward: 0.738 win percentage, but much lower average points.
  - Vanilla PPO (neither the perfect critic nor the oracle reward, with their imperfect features):
    0.509.
  - So, of the two additions, the perfect critic and the reward mainly bought **points**, not wins.
  - Against DouZero trained ten times longer (about 1e10), the critic-only variant lost (0.486).
- **What applies to us. This is the single most directly transferable paper.**
  - We already run PPO with a candidate-scoring actor, and gameenv already holds the full state. A
    perfect-information critic is therefore a change to what `Encode` exports *for training only*,
    plus a separate value input in `model.py`. The served `ZLNET1` file keeps only actor weights
    plus the imperfect value head (E3).
  - The oracle reward maps naturally onto our meld search: "minimum turns to go out given the true
    hand", or, in Žolíky, "cards still needed for the contract plus a clean run" (E8).
  - Their result that the reward mostly protected *points*, not win rate, matters for Canasta,
    where W1 is about points lost when caught.

### 4.3 Suphx (Riichi Mahjong)

- **Li, Koyamada, Ye, Liu, Wang, Yang, Zhao, Qin, Liu, Hon (2020). *Suphx: Mastering Mahjong with
  Deep Reinforcement Learning*. arXiv:2003.13590 (preprint; I found no peer-reviewed version).**
  [arXiv](https://arxiv.org/abs/2003.13590) [read, §3–5]
- **Summary.**
  - *Pipeline:* supervised pre-training on top human logs, then distributed policy-gradient RL with
    entropy regularisation, using an adaptive coefficient that keeps entropy near a target.
  - **Global reward prediction.** A game has 8 to 12 rounds and is scored by final rank. A two-layer
    GRU, trained on human logs, predicts the final game reward from per-round features: round
    score, cumulative scores, dealer, and so on. Each round's RL reward is the *change* in that
    prediction. The authors' example: a leader in the last round learns to play safe and forgo
    winning that round.
  - **Oracle guiding.** First train an "oracle" agent by RL that also sees opponents' hands and the
    wall. Then drop those extra features out with a probability that decays to zero, so the oracle
    becomes a normal agent. After that, keep training with the learning rate at one tenth and with
    rejection of large importance weights.
    - The authors report that **plain distillation from oracle to normal agent did not work well**.
  - **Parametric Monte Carlo policy adaptation (pMCPA).** At the start of a round, sample opponent
    hands, roll out with the current policy, and fine-tune the policy on those rollouts for this
    hand only. It won 66% against the unadapted agent over a few hundred rounds. It was too slow to
    deploy.
  - **Results:** each step helped (supervised < RL < RL plus global reward < RL plus global reward
    plus oracle guiding). On Tenhou it reached a record rank of 10 dan and a stable rank of 8.74 dan.
- **What applies to us.**
  - **Global reward prediction targets W1 directly.** A Canasta match runs over several deals to a
    target score. The model is currently rewarded per deal or at the end of the match, but what
    matters is how *going out now* changes P(win match).
    - A small predictor Φ(score state, deal index, variation, seats) trained on bench logs (no
      humans needed) turns that into a dense, correctly weighted per-deal reward (E4).
    - It also makes Žolíky's multi-deal penalty matches and Hold'em's 15-hand matches
      chip-equity-aware.
  - **Oracle guiding** is the second stage of E3: train with the perfect features as *actor inputs*
    and anneal them away. It is riskier than a perfect critic alone; see the counter-evidence in §4.4.
  - **pMCPA** is the cleanest published way to use our 5 s budget (we use 2 ms). It could run once
    per deal on the opening hand, or at the close/go-out decision only. Gradient updates in Go at
    serve time are a big build, so prefer search (E1/E7).

### 4.4 Gin Rummy and other rummy work

- **Kotnik & Kalita (2003). *The Significance of Temporal-Difference Learning in Self-Play Training:
  TD-Rummy versus EVO-Rummy*. ICML 2003, pp. 369–375.**
  [anthology](https://mlanthology.org/icml/2003/kotnik2003icml-significance/) [abstract]
  - **Summary:** The authors trained the same Gin Rummy evaluator by TD self-play and by
    co-evolution. Despite the title, **co-evolution gave the better player** in their experiments.
  - **Applicability:** historical. It shows population-based search over a small network is
    competitive in rummy, which supports population-based or league ideas (E9). It also predates
    modern policy-gradient methods.
- **EAAI-21 Gin Rummy Undergraduate Research Challenge** (organised by Todd Neller; 13 peer-reviewed
  papers in the AAAI-21 / EAAI-21 proceedings; limit of 30 s per player per game).
  [challenge page](http://cs.gettysburg.edu/~tneller/games/ginrummy/eaai/) [read]. Relevant papers:
  - **Mishra & Aggarwal (2021). *Opponent Hand Estimation in Gin Rummy Using Deep Neural Networks
    and Heuristic Strategies*. EAAI-21.**
    [pdf](https://cdn.aaai.org/ojs/17838/17838-13-21332-1-2-20210518.pdf) [read]
    - *Inputs:* four 13×4 planes: our hand plus the discard pile; cards the opponent took from the
      pile, by turn; cards it declined; cards it discarded.
    - *Network and output:* an MLP (2×500) gives a per-card probability that the opponent holds it.
      The agent uses this to avoid discarding cards the opponent probably wants.
    - *Result:* it won 57% of 1,500 games (95% CI about 54.5–59.5%) against the same agent with
      uniform beliefs.
    - **This is W2's fix, almost exactly**, and it is what `cardinfer.Want` approximates by hand.
  - **Francis, Just, Neller (2021). *Opponent Hand Estimation in the Game of Gin Rummy*. EAAI-21.**
    Also Hein et al., *Random Forests for Opponent Hand Estimation in Gin Rummy*; Goldman et al.,
    *Evaluating Gin Rummy Hands Using Opponent Modeling and Myopic Meld Distance*; and Maravich,
    Neller & Neller, *Knocking in the Game of Gin Rummy*. [titles verified on the challenge page;
    texts not read]
- **Kelidari, Haghi, Salmani (2026). *A Gold-Standard Study of What Makes a Lightweight Game-Playing
  Agent Strong*. arXiv:2607.06854; poster at AIIDE 2026 (per the paper).**
  [arXiv](https://arxiv.org/abs/2607.06854) [read, main sections]
  - **Setup:** more than 100 Gin Rummy PPO runs, graded against a fixed rule-based expert that is
    never used in training.
  - **What helped:** trust-region updates, a well-aimed terminal reward (knock-first), a curriculum
    of rising opponents, warm-starting, and keeping the best checkpoint.
  - **What did not help:**
    - dense or step reward shaping;
    - compressed state embeddings (the raw sparse card planes won);
    - imitation from an LLM guide;
    - bigger or structured encoders (Deep Sets, convolutional, recurrent and attention encoders
      matched the MLP);
    - **giving the policy the opponent's hidden hand as an extra input plane:** 24.7% vs 25.5%
      against the expert over four seeds, no detectable benefit.
  - **Search:** fair determinized ISMCTS was weak (10–26% vs the expert at 10–120 rollouts) and lost
    66–69% of games to the trained agents. An oracle version that saw the cards reached up to 85%.
  - **Style:** an expert that holds out for gin instead of knocking early drops from 70% to 15%.
    Paying three times more for gin did not make learners gin more against strong play.
  - **What applies to us:**
    - (a) **Counter-evidence for naive oracle *inputs*** (not for a perfect critic, which they did
      not test). Prefer E3's critic-only variant first.
    - (b) Dense shaping is not a free win; E8 must be A/B-tested, not assumed.
    - (c) **"Going out early beats greed against strong play"** in a closely related game. This
      supports making the Canasta model and Hard bot close more readily (W1, W4).
    - (d) Keep-best checkpoint selection on a fixed reference is a cheap habit to adopt in
      `train.py` (E9).
- **Mohan (2026). *IRumAI: Reinforcement Learning for Indian Rummy*. arXiv:2606.21975 (preprint).**
  [arXiv](https://arxiv.org/abs/2606.21975) [read, ablations and probe]
  - **Setup:** PPO with meld-aware encoding, potential-based shaping where Φ = −(meld deadwood), a
    one-off behaviour-cloning warm start, and self-play against heuristics; 0.33 ms per action.
  - **Ablations (single seed):**
    - shaping added up to 4.9 points of win rate;
    - behaviour cloning sped up the first ~2k updates, but both runs met by ~7k;
    - PFSP did not beat uniform opponent sampling.
  - **Linear probe on the frozen trunk:** predicted the opponent's hand with AUC 0.744, against
    0.710 for card counting, and top-13 recall of 47% against 37%. So the policy *implicitly* infers
    hands from pickups and discards.
  - **What applies to us:**
    - The probe is a cheap **diagnostic we should run on our Žolíky and Canasta trunks before
      building E2**. If the trunk already beats `cardinfer`, the auxiliary head adds less.
    - The shaping result contradicts Kelidari et al. Treat shaping as an open question for our
      games.
- **Canasta-specific AI.** I searched for peer-reviewed Canasta AI papers and **found none**.
  RLCard (Zha et al. 2019, arXiv:1910.04376) lists several card-game environments, but I did not
  find Canasta among them. Canasta-specific guidance has to come from the rummy and Dou Dizhu work
  above. Our partnership format makes the cooperative and Hanabi literature (§5) and the team
  shedding game below the nearest analogues.
- **Lu, Zhao, Zhao, Zhou, Li (2022). *DanZero: Mastering GuanDan Game with Reinforcement Learning*.
  arXiv:2210.17087.** [abstract]
  - **Summary:** It applies DouZero-style DMC to GuanDan, a 2-vs-2 *partnership* shedding game. It
    beat eight rule-based baselines and played at human level, after 30 days on 160 CPUs and one
    GPU.
  - **What applies to us:** evidence that DMC works with partnerships, and a sobering compute
    figure.

---

## 5. Opponent hand inference and belief modelling

- **Foerster, Song, Hughes, Burch, Dunning, Whiteson, Botvinick, Bowling (2019). *Bayesian Action
  Decoder for Deep Multi-Agent Reinforcement Learning*. ICML 2019.**
  [arXiv:1811.01458](https://arxiv.org/abs/1811.01458) [read, abstract]
  - **Summary:** Every agent maintains a common-knowledge *public belief*, updated by Bayes' rule
    from the actions taken. Agents learn to act *informatively* because partners decode their
    actions. Demonstrated in Hanabi.
- **Hu & Foerster (2020). *Simplified Action Decoder for Deep Multi-Agent Reinforcement Learning*.
  ICLR 2020.** [arXiv:1912.02288](https://arxiv.org/abs/1912.02288) [abstract]
  - **Summary:** During training, each agent also observes its teammate's *greedy* action, not just
    the exploratory one actually taken. This resolves the clash between exploring and being
    interpretable, and reached state-of-the-art self-play Hanabi scores for 2 to 5 players.
  - The paper also uses an **auxiliary task of predicting the agent's own hidden cards** [from
    memory of the paper; not re-checked in the text].
- **Lerer, Hu, Foerster, Brown (2020). *Improving Policies via Search in Cooperative Partially
  Observable Games* (SPARTA). AAAI 2020.** [arXiv:1912.02318](https://arxiv.org/abs/1912.02318)
  [read, abstract]
  - **Summary:** All other players are fixed to an agreed "blueprint" policy. One agent then runs
    Monte Carlo search over worlds weighted by the exact belief that the blueprint implies.
- **Hu, Lerer, Brown, Foerster (2021). *Learned Belief Search*. arXiv:2106.09086** (workshop
  version at AAAI-21 RLG; I could not confirm a main-track venue). [abstract]
  - **Summary:** A supervised model trained on self-play learns the belief over hidden cards
    autoregressively, replacing exact belief tracking. The authors report it recovers a large share
    of the benefit of exact-belief search at much lower cost.
- **Hu, Lerer, Cui, Pineda, Brown, Foerster (2021). *Off-Belief Learning*. ICML 2021.**
  [pmlr](https://proceedings.mlr.press/v139/hu21c.html) [abstract]
  - **Summary:** Training assumes past actions came from a fixed reference policy. This controls
    which conventions agents may read into each other's moves.
- **What applies to us.**
  1. **The Learned Belief Search recipe fits us closely.** Train a hidden-card predictor by
     supervised learning on self-play data, which gameenv can log with the true hands. Sample
     worlds from it for search. This is the learned replacement for `cardinfer` (E1 and E2).
  2. **Partnership Canasta is cooperative between partners.** BAD and SAD show that partners'
     actions carry information, and that RL can learn to *signal* when partners model each other.
     - Our learner seats partnered with other learner seats (`league.partner_bot`) will drift
       towards private conventions that a human partner cannot read. Off-Belief Learning is the
       principled fix.
     - The pragmatic fix is to train a share of partnership tables with the heuristic partner, and
       to measure partnership strength with a *heuristic* partner on the bench.
  3. **The SPARTA pattern also fits Canasta.** At four seats, our partner is our own policy, so its
     behaviour is known exactly. Inference about the *partner's* hand can use our own policy without
     approximation. That is cheap policy-based inference for half the table.

---

## 6. Training methods that apply to us

- **Vinyals et al. (2019). *Grandmaster level in StarCraft II using multi-agent reinforcement
  learning* (AlphaStar). Nature 575:350–354.**
  [doi](https://www.nature.com/articles/s41586-019-1724-z) [abstract]
  - **Summary:** League training with three kinds of agent:
    - a *main agent*, trained with prioritised fictitious self-play (PFSP), which samples opponents
      by how often they beat it;
    - *main exploiters*, trained only to beat the current main agent;
    - *league exploiters*, trained to find weaknesses across the whole league.
  - Combined with imitation from human games, this reached Grandmaster level (top 0.2%).
  - **What applies to us:**
    - A **"closer exploiter" for Canasta** is the AlphaStar answer to W1. Train one learner whose
      only opponent is the current main model, and add its checkpoints to the main agent's league
      with PFSP weight.
    - A **"caller exploiter" for Hold'em** does the same for W3: it learns to call the main model's
      shoves.
- **Jaderberg et al. (2017). *Population Based Training of Neural Networks*. arXiv:1711.09846.**
  [abstract]
  - **Summary:** A population trains in parallel. Every so often the weak members copy the weights
    of the strong ones and perturb their hyperparameters.
  - **What applies to us:** PBT is useful for tuning the entropy coefficient, the learning rate and
    the league blend. It needs enough parallel compute for a population of four or more; check the
    budget before committing.
- **Jaderberg et al. (2017). *Reinforcement Learning with Unsupervised Auxiliary Tasks* (UNREAL).
  ICLR 2017.** [abstract]
  - **Summary:** Auxiliary prediction and control heads, sharing the trunk, speed up and stabilise RL
    a great deal.
  - **What applies to us:** this is the general justification for E2's hidden-card head, and for a
    "final deal score" or "will an opponent go out within k turns" head.
- **Jacob, Wu, Farina, Lerer, Hu, Bakhtin, Andreas, Brown (2022). *Modeling Strong and Human-Like
  Gameplay with KL-Regularized Search* (piKL). ICML 2022.**
  [pmlr](https://proceedings.mlr.press/v162/jacob22a.html) [abstract]
  - **Summary:** Search or RL is regularised towards a policy imitated from humans. The agent is
    both stronger and more human-like than imitation alone, across chess, Go, Hanabi and Diplomacy.
  - **What applies to us:**
    - We have human games, through `cmd/export-games`, which already matches stored moves to
      candidates.
    - Behaviour cloning from those logs gives a warm start (Suphx and IRumAI both used one; IRumAI
      found it sped up learning without changing the final strength).
    - A KL anchor to the human-cloned policy is the principled way to make "styles" and difficulty
      levels feel human, not just weak.
- **Reward shaping vs global reward prediction.**
  - Potential-based shaping (Ng, Harada, Russell 1999, ICML; citation only) leaves optimal policies
    unchanged *only* if the shaping term has the form γΦ(s′) − Φ(s).
  - Suphx's global reward prediction is a special case: the potential is the predicted match
    outcome.
  - PerfectDou's oracle reward is a potential computed on the true deal.
  - Kelidari et al.'s dense step reward was not potential-based and did not help.
  - **Rule for us:** any shaping we add should be written as a potential difference. Prefer
    potentials that estimate the *true objective* (match equity) over proxies (cards melded).

---

## 7. Proposed experiments

**Shared protocol.**

- **Bench:** `gamebench` duplicate on held-out seeds (≥ 1,000,000), seats swapped. Report the mean
  with a 95% CI; "ahead" means more than two standard errors.
- **Pre-registration:** fix the seed counts *before* running: Hold'em ≥ 2,000 matches, Canasta ≥ 400
  matches per seat count, Žolíky ≥ 1,000.
- **Behaviour metrics:** add them to `learn/bench.go` output when missing.
- **Claude test game:** a human-style session through the client against the candidate, with a short
  written log of three to five decisions that matter (closing, a dangerous discard, a shove), to
  catch failures the averages hide.
- **Search variants:** evaluate under the server's real time budget and node budgets, so results
  replay the same moves on any machine, like the Mariáš bot's `nodeBudget`.

### E1. Inference-weighted determinization vs uniform PIMC (Canasta, Žolíky) [AI] [Hard]

- **Hypothesis.** Search that samples worlds from `cardinfer`, re-weighted by our own policy's
  likelihood of the opponents' observed actions, beats the same search with uniform consistent
  sampling. The gain concentrates in the late deal and in discard choice.
- **Step 0 (one day): measure whether PIMC can work here.** Port Long et al.'s measurement: leaf
  correlation, bias, and disambiguation (how fast the number of consistent worlds shrinks per turn).
  Run it on about 2,000 sampled Canasta and Žolíky positions bucketed by stock size. If correlation
  is low early, restrict search to late positions.
- **Implementation.**
  1. `Sample(obs, n)` in `cardinfer`: draw worlds consistent with `Held`, counts and melds. Propose
     each card's location in proportion to the fitted IPF matrix, with capped rejection.
  2. **Policy-based inference weights:** for each world, replay the last K public actions of each
     opponent (K = 6–10 turns is enough). Call `Policy.Logits` from that seat's reconstructed view,
     and multiply the softmax probabilities of the actions actually taken. Divide by the proposal
     probability. Use a temperature or floor so that one surprising move does not zero a world
     (Kermit's brittleness point).
  3. **Search:** for each candidate at the root, average over weighted worlds the value-head estimate
     after a short rollout of d plies (opponents' moves from the policy). Choose by weighted mean.
     Compare three settings: uniform weights, `cardinfer` proposal only, and proposal plus
     policy-based inference.
  4. **Budget:** 20–50 worlds × ≤ 30 candidates × d ≤ 4 forward passes at about 10 µs each fits
     well inside 5 s. Fix node counts, not time.
- **Measurement.**
  - Bench: the search bot vs the raw model, and vs Hard.
  - Inference quality, logged from gameenv with the true deal: the true world's average
    log-likelihood rank, and the AUC for "opponent holds card c", for each method.
  - Behaviour: discards later taken by the next player (W2), and points held when caught (W1).
- **Success.**
  - Proposal plus policy-based inference beats uniform by more than 2 SE in at least one game.
  - The inference AUC beats `cardinfer` alone.
  - No regression in mean think time beyond 500 ms p95.

### E2. Auxiliary hidden-card prediction head (all games) [AI]

- **Hypothesis.** Predicting opponents' hidden cards from the trunk, as an auxiliary loss, improves
  representation and therefore play, especially Žolíky discards (W2). The head's output is also a
  better proposal for E1 than the hand-built weights.
- **Step 0 (diagnostic):** fit a linear probe on the frozen trunk of the shipped Žolíky and Canasta
  models (IRumAI's method). Compare its AUC with `cardinfer.Hold`. If the probe is already as good,
  the auxiliary head's gain will come from the *explicit output*, not from representation.
- **Implementation.**
  - gameenv emits the true per-opponent hand vector (`NumSlots` × seats; counts for the two-deck
    pack) as a training-only target.
  - `model.py` adds a head from the shared trunk with a per-slot output, scored by a
    binomial/Poisson count loss with weight λ ∈ {0.1, 0.3, 1}. For Hold'em, use a 169-class
    hand-class distribution per opponent.
  - The head is exported (a version bump of `ZLNET1`), so Go can read it. Its outputs then become
    **state features** in a second training round (a "danger" score for each discard candidate: the
    predicted probability that the next seat can use the card), or feed E1.
- **Measurement.**
  - Bench vs the baseline at equal training steps (three seeds each).
  - Head AUC and calibration on held-out seeds.
  - The W2 metric: the share of our discards picked up by the next seat, or completing a meld for
    it within one turn.
- **Success.**
  - The fed-discard rate falls by ≥ 20% relative, with a bench gain above 2 SE in Žolíky.
  - No loss elsewhere.

### E3. Perfect-information critic, then oracle guiding (Canasta, Žolíky, Hold'em) [AI]

- **Hypothesis.** A critic that sees all hands gives lower-variance advantages and learns faster and
  better (PerfectDou). Serving cost is unchanged because the actor's inputs do not change.
- **Implementation.**
  - **Stage A (critic only).** gameenv also returns `EncodeFull`: the legal view plus all hidden
    hands and the stock order.
    - `model.py` gets a separate value MLP on `EncodeFull`, used only for GAE.
    - The exported file keeps an imperfect-information value head, trained by regression on the
      same returns, because the search experiments need a value they can compute at serve time.
  - **Stage B (Suphx oracle guiding, only if A helps).** Feed the hidden features to the *actor*
    through a mask whose keep-probability anneals from 1 to 0. Then fine-tune at one tenth of the
    learning rate with importance-weight rejection.
    - Kelidari et al. found the hidden hand as a plain input did nothing in Gin Rummy, so B is
      lower-priority.
- **Measurement.**
  - Learning curves: bench score against Hard at fixed checkpoints.
  - The final bench at equal environment steps, three seeds each.
  - Explained variance of the value head.
- **Success.**
  - Equal strength in ≤ 60% of the environment steps, or a gain above 2 SE at equal steps.
  - Value explained variance clearly higher.

### E4. Global reward prediction for long Canasta matches (and Žolíky, Hold'em) [AI]

- **Hypothesis.** The model under-closes (W1) because its reward does not price the *match*
  consequence of letting an opponent go out. Rewarding the change in predicted match-win
  probability per deal teaches it when closing is right.
- **Implementation.**
  1. Log about 200k bench matches (mixed bots) with the score state at each deal boundary:
     cumulative scores per side, deal index, the variation's target score and minimum-meld
     thresholds, seat count.
  2. Train Φ: P(win match | state). Start with gradient-boosted trees or a small MLP; a GRU is only
     needed if the history matters beyond the scores.
  3. In `ppo.py`, set the reward at each deal end to Φ(after) − Φ(before), plus the terminal match
     result. As a potential difference, the optimal policy is unchanged.
  4. Optionally **within a deal**: Φ′(state) predicts the deal's points difference from the public
     state plus the true hands (training only). That gives a dense oracle reward for "points at
     risk if caught".
- **Measurement.**
  - Bench match win rate *and* points per match.
  - Behaviour: close rate, points held when an opponent goes out, a histogram of deals by turns
    taken. Report these against the closer style in particular.
- **Success.**
  - Against closer: points held when caught fall by ≥ 25%, and the match win rate rises by more than
    2 SE.
  - Against Hard: no loss.

### E5. Equilibrium bluffing and push/fold for rule-based Hold'em Hard [Hard], plus model features [AI]

- **Hypothesis.**
  - Tying bluff frequency and call-down thresholds to B/P (§2.8), and using jam/fold charts at
    ≤ 15 BB (§2.7), makes Hard harder to exploit without making it weaker against the styles.
  - Giving the model the same quantities as candidate features fixes over-shoving (W3).
- **Implementation.**
  1. Offline Go tool: CFR+ on the jam/fold game for 2 and 3 players, at stacks of 1–20 BB in 0.5 BB
     steps (169 hand classes; equities from a precomputed table). Emit `pushfold_tables.go`.
     For 4+ players, approximate by treating the remaining players to act collectively, or compute
     with MCCFR. Note that no peer-reviewed source covers this.
  2. In `holdem/bot.go`, replace the flat `bluff` and `bluffRaise` chances by α(B/P) scaled by
     1/(number of opponents). Replace `bluffShare`'s flat discount with "defend the top P/(P+B)
     share of the range against an unknown opponent". Keep the style knobs as *deviations* from
     these.
  3. Below the stack threshold, Hard plays the charts exactly.
  4. Model features on the all-in candidate: the chart decision (in or out of range), the chart
     margin, and the EV of shoving against the chart's calling ranges. Retrain.
  5. League: add a `pusher` style that plays the charts and calls by them, so the model meets exact
     calling ranges.
- **Measurement.**
  - Bench vs every style and vs the previous Hard, in BB per match. Use an all-in-adjusted
    (AIVAT-lite) estimator: replace realised all-in pots with their equity.
  - Behaviour: shove frequency by hand class × stack, compared with the chart; fold-to-shove;
    showdown win rate of shoves.
- **Success.**
  - Hard gains over its previous version against `station` and `solid`, and loses no more than 1 SE
    against `rock`.
  - The model's off-chart shoves with hands below the chart fall by ≥ 50%, and BB per match against
    `solid` improves by more than 2 SE.

### E6. Deep Monte Carlo (DouZero-style) vs PPO for Žolíky and Canasta [AI]: is it worth it?

- **Assessment first.**
  - DouZero preferred DMC because policy gradients with a fixed output layer cannot generalise
    across actions. **Our PPO actor already scores per-candidate features**, so the main argument
    does not apply.
  - PerfectDou then beat DouZero with PPO and an actor that scores candidates, using about one tenth
    of the samples.
  - DMC's real advantages for us would be simplicity (no critic, no ratio clipping) and a Q-value
    per candidate, which is directly usable as a search leaf.
  - Its costs: epsilon-greedy exploration, high-variance Monte Carlo targets over long Canasta
    matches, and a lot of compute (DanZero: 30 days on 160 CPUs).
- **Recommendation: no full switch.** Run the cheap variant.
- **Hypothesis (cheap variant).** A Q head trained by Monte Carlo regression on candidate features
  next to PPO gives a better per-candidate value for E1 and E7 leaves than V(s′) after the move.
  Optionally, a pure-DMC Žolíky run (the shorter game) tests whether DMC beats PPO at equal
  wall-clock.
- **Implementation.**
  - Add `q = MLP([state_emb, cand_emb])` with an MSE loss to the realised return, sharing the trunk.
  - For pure DMC: a new `dmc.py` loop reusing `env.py` and `league.py`, with ε from 0.1 decaying to
    0.01.
- **Measurement.**
  - Bench at equal wall-clock and equal environment steps.
  - For leaf quality: rank correlation of Q with the outcome of exhaustive rollouts on 1,000 late
    positions.
- **Success.** Pure DMC is worth keeping only if it beats PPO by more than 2 SE at equal wall-clock.
  Otherwise keep the Q head only if it improves E1/E7 by more than 1 SE.

### E7. Closing / going-out decision search for Canasta (model override and Hard rule) [AI] [Hard]

- **Hypothesis.** W1 and W4 are one decision: go out (or meld towards going out) now, or keep
  building. A narrow search for that decision alone, scored by match equity (E4's Φ), fixes most of
  both. The rest of play stays policy- or heuristic-driven.
- **Implementation.**
  1. **Trigger:** a "go out" candidate exists, or our side could go out within one turn (meld
     search says so), or an opponent's hand is ≤ 3 cards.
  2. Sample N = 50 worlds (E1's sampler, or `cardinfer` alone at first).
  3. Compare "go out now" (exact scoring) with each "continue" candidate. Roll the continuation out
     with the policy for 2–3 rounds and score at the end of the deal: deal points plus Φ for match
     equity. Pluribus-style, score each continuation against both the policy and a "closer" variant
     of the opponents, and take the minimum.
  4. **Hard:** use the same routine with the heuristic as the rollout policy. It replaces or informs
     `worthGoingOut`.
  5. **Partnerships:** if the rules variant makes "ask partner" part of going out, model the
     partner's answer with the partner's own bot. It is known exactly (SPARTA, §5).
- **Measurement.**
  - Bench Hard-plus-search vs Hard, and model-plus-search vs model, at 2, 4 and 6 seats.
  - Behaviour: close rate, mean turns to close, points held when caught, match win rate against
    closer.
- **Success.**
  - At four seats, Hard-plus-search beats Hard by more than 2 SE.
  - The model's points held when caught against closer fall by ≥ 25%.
  - p95 think time stays under 1 s.

### E8. Distance-to-go-out reward for Žolíky (and Canasta) [AI]

- **Hypothesis.** A potential Φ = −(minimum cards still needed to go down legally and finish with a
  clean run), from the existing meld search, speeds learning of clean-run completion (W2).
  PerfectDou and IRumAI support this; Kelidari et al. caution against it.
- **Implementation.**
  - Expose `ai`'s meld search as a function returning "cards short" for a hand under the active
    variation's minimum and clean-run rule.
  - Variant (a): a potential on our own hand, γΦ(s′) − Φ(s).
  - Variant (b), PerfectDou-style: a *relative* potential, our distance minus the minimum opponent
    distance, on the true deal (training only).
  - Weight β ∈ {0.05, 0.2}. Run three seeds each.
- **Measurement.**
  - Bench in penalty points per match.
  - Behaviour, logged in gameenv: the rate of missed clean runs (turns where a legal clean-run
    finish existed but was not taken, found by an exhaustive solver), and turns to go down.
- **Success.**
  - Missed clean-run rate falls by ≥ 50%, with a bench gain above 2 SE.
  - If the bench is flat while behaviour improves, keep the version with the better bench, not the
    better behaviour.

### E9. League hygiene and PPO stabilisers (Hold'em first, then Canasta) [AI]

- **Hypothesis.** W3 and W1 are partly training-distribution problems. PFSP weighting, a dedicated
  exploiter, keeping the best checkpoint, and AlphaHoldem's value clipping make the model robust to
  the opponents it now loses to.
- **Implementation.**
  - `league.py`: sample opponents in proportion to f(P(they beat us)), with f(x) = x² (AlphaStar's
    "hard" weighting), using running win rates per opponent from `reward_vs`.
  - Train an exploiter run whose only opponent is the current main snapshot. Add its snapshots to
    the main league.
  - In `train.py`, keep the best checkpoint by a fixed held-out bench every N updates.
  - In `ppo.py` for Hold'em, clip the value target to ± the chips committed in the hand, and add the
    δ₁ ratio clip for negative advantages.
- **Measurement.**
  - Bench against all styles plus `solid` and `hard`.
  - Worst-case score across opponents, a robustness metric that should rise even if the mean is
    flat.
  - Run-to-run variance across three seeds.
- **Success.** The worst-case opponent score improves by more than 2 SE, with no loss of the mean.

### E10. Rule-based Canasta Hard: partnership going-out rule [Hard]

- **Hypothesis.** W4 comes from `worthGoingOut` weighing our side's meld bonus against opponents'
  likely points without the *partner's* exposure, and without the opponents' race (how soon they
  can go out).
- **Implementation.** An explicit race estimate:
  - The expected turns until each opposing seat can go out: hand size, canastas made, and
    `cardinfer.Want` for the pile top.
  - The expected penalty our side carries if caught: our hand plus the partner's hand count × the
    average card value.
  - Go out when this deal's equity from going out now exceeds that of continuing for one more round
    under the race estimate. Use E4's Φ once it exists.
  - This is a deterministic rule, so it ships before E7.
- **Measurement.** Bench at 4 and 6 seats against the old Hard and against closer, plus close rate
  and points held when caught.
- **Success.** At four seats, the new Hard beats the old Hard by more than 2 SE.

---

## 8. Pitfalls: what not to do

1. **Do not assume PIMC transfers from Mariáš to the rummy games.**
   - PIMC works where information sets shrink fast and outcomes are correlated (trick-taking). It
     degrades where outcomes swing late (Long et al.), and Canasta's "who goes out first" is exactly
     such an outcome.
   - Measure the three properties first (E1, step 0), and use search late in a deal or for narrow
     decisions.
   - Kelidari et al.'s fair ISMCTS *lost* to a 0.3 ms PPO policy in Gin Rummy.
2. **Strategy fusion makes PIMC over-confident about "flexible" moves.** Holding cards "to decide
   later" looks free in every sampled world, because each world knows which card it will need. This
   is a direct path to W1 (never closing) and W2 (holding instead of shedding).
   - Counter it with search that commits to one choice across all worlds (ISMCTS or αμ-style).
   - Or, more cheaply, roll out with the *policy*, which cannot see the world, instead of a
     clairvoyant solver.
3. **PIMC never plays to hide or gather information.** In Canasta, taking the pile reveals what we
   hold, and a "safe" discard hides it. Determinized search values neither. Keep those choices with
   the policy unless search shows a clear gain.
4. **Inference built on a deterministic or brittle opponent model breaks against humans** (Kermit
   §4.3). Always floor or temper the action likelihoods in policy-based inference. Evaluate
   inference against *different* bots and in Claude test games, not only in self-play.
5. **Exploitation can backfire.**
   - Over-shoving (W3) is a policy that learned folders exist. Exploit only on per-opponent evidence
     shrunk towards the prior (data-biased response), and risk only what you are up (safe
     exploitation).
   - Never ship a model trained mostly against caricature styles without benching it against
     `solid` and `hard`.
   - Pluribus shows a non-adaptive, balanced strategy is enough to win multi-way.
6. **Learned partner conventions do not transfer to humans.** Self-play partners invent private
   signals (BAD, SAD, OBL). Bench Canasta partnership strength with a heuristic partner, and keep
   some training tables with a heuristic partner.
7. **Oracle inputs are not oracle critics.**
   - Plain distillation from an oracle actor failed in Suphx, and an oracle input plane did nothing
     in Gin Rummy (Kelidari et al.).
   - Put privileged information in the *critic* (PerfectDou) or anneal it out (Suphx). Never leave
     it reachable at serve time: our TestBotDoesNotPeek checks must cover any new encoder.
8. **Shaping that is not a potential difference changes the objective.** Kelidari et al.'s dense
   reward did not help. Shaping towards "melding more" can make a bot that melds instead of going
   out, which is W1 again. Write every shaping term as γΦ(s′) − Φ(s), and prefer Φ that estimates
   match equity.
9. **Over-investing in heavy equilibrium methods.**
   - ReBeL, Student of Games and DeepStack need belief-state inputs that are infeasible for rummy
     hands, and their guarantees hold only for two-player zero-sum games.
   - For Hold'em, a small CFR+ jam/fold table and indifference-based frequencies give most of the
     practical value at about 1% of the effort.
10. **Measurement traps.**
    - Self-play win rate says little (Kelidari et al.: agents tie copies of themselves).
    - A "best checkpoint" chosen on the same seeds it is reported on is optimistic; select on one
      held-out range and report on another.
    - Hold'em needs variance reduction, such as all-in EV adjustment or an AIVAT-style control
      variate, or results will be noise at our match counts.
11. **Model format churn.** Any new head or feature changes `stateDim` or `candDim`, and the server
    refuses mismatched models. Batch the encoder changes (E2 features, E5 features, the E6 Q head)
    into as few `ZLNET` version bumps as possible, and keep the heuristic fallback path tested.

---

## 9. References

Verification key: [read] means the primary text was read; [abstract] means the official
abstract or proceedings page was checked; [citation] means the citation was checked but the text was
not read; [secondary] means a detail came from secondary sources.

**Poker and equilibrium**

1. Zinkevich, M., Johanson, M., Bowling, M., Piccione, C. (2007). Regret Minimization in Games with
   Incomplete Information. *NIPS 2007*. [citation]
2. Tammelin, O. (2014). Solving Large Imperfect Information Games Using CFR+. arXiv:1407.5042.
   https://arxiv.org/abs/1407.5042 [read]
3. Bowling, M., Burch, N., Johanson, M., Tammelin, O. (2015). Heads-up limit hold'em poker is solved.
   *Science* 347(6218). [citation]
4. Lanctot, M., Waugh, K., Zinkevich, M., Bowling, M. (2009). Monte Carlo Sampling for Regret
   Minimization in Extensive Games. *NIPS 2009*.
   https://papers.nips.cc/paper/3713-monte-carlo-sampling-for-regret-minimization-in-extensive-games [abstract]
5. Brown, N., Lerer, A., Gross, S., Sandholm, T. (2019). Deep Counterfactual Regret Minimization.
   *ICML 2019*. https://arxiv.org/abs/1811.00164 [read]
6. Heinrich, J., Silver, D. (2016). Deep Reinforcement Learning from Self-Play in Imperfect-Information
   Games. arXiv:1603.01121. https://arxiv.org/abs/1603.01121 [read]
7. Moravčík, M. et al. (2017). DeepStack: Expert-level artificial intelligence in heads-up no-limit
   poker. *Science* 356(6337):508–513. https://arxiv.org/abs/1701.01724 [read]
8. Brown, N., Sandholm, T. (2018). Superhuman AI for heads-up no-limit poker: Libratus beats top
   professionals. *Science* 359:418–424. https://www.science.org/doi/10.1126/science.aao1733 [abstract]
9. Brown, N., Sandholm, T. (2019). Superhuman AI for multiplayer poker. *Science* 365:885–890.
   https://www.science.org/doi/10.1126/science.aay2400 [abstract; continuation-strategy and compute
   details secondary]
10. Brown, N., Bakhtin, A., Lerer, A., Gong, Q. (2020). Combining Deep Reinforcement Learning and Search
    for Imperfect-Information Games (ReBeL). *NeurIPS 2020*. https://arxiv.org/abs/2007.13544 [read]
11. Schmid, M. et al. (2023). Student of Games: A unified learning algorithm for both perfect and
    imperfect information games. *Science Advances* 9(46).
    https://www.science.org/doi/10.1126/sciadv.adg3256 [read]
12. Zhao, E., Yan, R., Li, J., Li, K., Xing, J. (2022). AlphaHoldem: High-Performance Artificial
    Intelligence for Heads-Up No-Limit Poker via End-to-End Reinforcement Learning. *AAAI 2022*.
    https://cdn.aaai.org/ojs/20394/20394-13-24407-1-2-20220628.pdf [read]
13. Miltersen, P. B., Sørensen, T. B. (2007). A near-optimal strategy for a heads-up no-limit Texas
    Hold'em poker tournament. *AAMAS 2007*. https://dl.acm.org/doi/10.1145/1329125.1329357 [abstract]
14. Ganzfried, S., Sandholm, T. (2008). Computing an approximate jam/fold equilibrium for 3-player
    no-limit Texas hold'em tournaments. *AAMAS 2008*. [citation]
15. Chen, B., Ankenman, J. (2006). *The Mathematics of Poker*. ConJelCo. (Book; the formulas in §2.8
    are re-derived here.) [citation]
16. Johanson, M., Zinkevich, M., Bowling, M. (2007). Computing Robust Counter-Strategies. *NIPS 2007*.
    https://papers.nips.cc/paper/2007/hash/6e7b33fdea3adc80ebd648fffb665bb8-Abstract.html [abstract]
17. Johanson, M., Bowling, M. (2009). Data Biased Robust Counter Strategies. *AISTATS 2009*.
    https://mlanthology.org/aistats/2009/johanson2009aistats-data/ [abstract]
18. Ganzfried, S., Sandholm, T. (2011). Game theory-based opponent modeling in large
    imperfect-information games. *AAMAS 2011*. [citation]
19. Ganzfried, S., Sandholm, T. (2012). Safe Opponent Exploitation. *ACM EC 2012*.
    http://www.cs.cmu.edu/~sandholm/safeExploitation.ec12.pdf [abstract]
20. Burch, N., Schmid, M., Moravčík, M., Morrill, D., Bowling, M. (2018). AIVAT: A New Variance
    Reduction Technique for Agent Evaluation in Imperfect Information Games. *AAAI 2018*.
    https://ojs.aaai.org/index.php/AAAI/article/view/11481 [abstract]

**Determinization and search**

21. Frank, I., Basin, D. (1998). Search in games with incomplete information: a case study using
    Bridge card play. *Artificial Intelligence* 100:87–123. [citation]
22. Ginsberg, M. L. (2001). GIB: Imperfect Information in a Computationally Challenging Game. *JAIR*
    14:303–358. https://mlanthology.org/jair/2001/ginsberg2001jair-gib/ [abstract]
23. Long, J., Sturtevant, N., Buro, M., Furtak, T. (2010). Understanding the Success of Perfect
    Information Monte Carlo Sampling in Game Tree Search. *AAAI 2010*, 134–140.
    https://ojs.aaai.org/index.php/AAAI/article/view/7562 [read]
24. Cowling, P. I., Powley, E. J., Whitehouse, D. (2012). Information Set Monte Carlo Tree Search.
    *IEEE TCIAIG* 4(2):120–143. https://eprints.whiterose.ac.uk/id/eprint/75048/ [read]
25. Buro, M., Long, J. R., Furtak, T., Sturtevant, N. (2009). Improving State Evaluation, Inference,
    and Search in Trick-Based Card Games. *IJCAI 2009*, 1407–1413.
    https://www.ijcai.org/Proceedings/09/Papers/236.pdf [read]
26. Solinas, C., Rebstock, D., Buro, M. (2019). Improving Search with Supervised Learning in
    Trick-Based Card Games. *AAAI 2019*, 33(01):1158–1165. https://arxiv.org/abs/1903.09604 [read]
27. Rebstock, D., Solinas, C., Buro, M., Sturtevant, N. R. (2019). Policy Based Inference in
    Trick-Taking Card Games. *IEEE CoG 2019*. https://arxiv.org/abs/1905.10911 [read]
28. Solinas, C., Rebstock, D., Sturtevant, N. R., Buro, M. (2023). History Filtering in Imperfect
    Information Games: Algorithms and Complexity. *NeurIPS 2023*. [read, abstract]
29. Cazenave, T., Ventos, V. (2019/2021). The αμ Search Algorithm for the Game of Bridge.
    arXiv:1911.07960; *Monte Carlo Search (MCS 2020 workshop at IJCAI), Springer CCIS*, 2021.
    https://arxiv.org/abs/1911.07960 [read]

**Large card games**

30. Zha, D., Xie, J., Ma, W., Zhang, S., Lian, X., Hu, X., Liu, J. (2021). DouZero: Mastering
    DouDizhu with Self-Play Deep Reinforcement Learning. *ICML 2021*.
    https://arxiv.org/abs/2106.06135 [read]
31. Yang, G., Liu, M., Hong, W., Zhang, W., Fang, F., Zeng, G., Lin, Y. (2022). PerfectDou:
    Dominating DouDizhu with Perfect Information Distillation. *NeurIPS 2022*.
    https://arxiv.org/abs/2203.16406 [read]
32. Li, J., Koyamada, S., Ye, Q., Liu, G., Wang, C., Yang, R., Zhao, L., Qin, T., Liu, T.-Y., Hon,
    H.-W. (2020). Suphx: Mastering Mahjong with Deep Reinforcement Learning. arXiv:2003.13590.
    https://arxiv.org/abs/2003.13590 [read]
33. Lu, Y., Zhao, J., Zhao, Y., Zhou, W., Li, H. (2022). DanZero: Mastering GuanDan Game with
    Reinforcement Learning. arXiv:2210.17087. [abstract]
34. Kotnik, C., Kalita, J. (2003). The Significance of Temporal-Difference Learning in Self-Play
    Training: TD-Rummy versus EVO-rummy. *ICML 2003*, 369–375.
    https://mlanthology.org/icml/2003/kotnik2003icml-significance/ [abstract]
35. Neller, T. W. (organiser). EAAI-21 Gin Rummy Undergraduate Research Challenge (13 papers in
    *AAAI-21 / EAAI-21*). http://cs.gettysburg.edu/~tneller/games/ginrummy/eaai/ [read]
36. Mishra, B., Aggarwal, A. (2021). Opponent Hand Estimation in Gin Rummy Using Deep Neural
    Networks and Heuristic Strategies. *EAAI-21 (AAAI-21)*.
    https://cdn.aaai.org/ojs/17838/17838-13-21332-1-2-20210518.pdf [read]
37. Francis, P., Just, H. A., Neller, T. (2021). Opponent Hand Estimation in the Game of Gin Rummy.
    *EAAI-21*. [title verified]
38. Kelidari, N., Haghi, M., Salmani, M. (2026). A Gold-Standard Study of What Makes a Lightweight
    Game-Playing Agent Strong. arXiv:2607.06854 (AIIDE 2026 poster per the paper).
    https://arxiv.org/abs/2607.06854 [read]
39. Mohan, V. (2026). IRumAI: Reinforcement Learning for Indian Rummy. arXiv:2606.21975.
    https://arxiv.org/abs/2606.21975 [read]
40. Zha, D. et al. (2019). RLCard: A Toolkit for Reinforcement Learning in Card Games.
    arXiv:1910.04376. [citation]

**Belief modelling and cooperative play**

41. Foerster, J. N. et al. (2019). Bayesian Action Decoder for Deep Multi-Agent Reinforcement
    Learning. *ICML 2019*. https://arxiv.org/abs/1811.01458 [read, abstract]
42. Hu, H., Foerster, J. N. (2020). Simplified Action Decoder for Deep Multi-Agent Reinforcement
    Learning. *ICLR 2020*. https://arxiv.org/abs/1912.02288 [abstract]
43. Lerer, A., Hu, H., Foerster, J., Brown, N. (2020). Improving Policies via Search in Cooperative
    Partially Observable Games. *AAAI 2020*. https://arxiv.org/abs/1912.02318 [read, abstract]
44. Hu, H., Lerer, A., Brown, N., Foerster, J. (2021). Learned Belief Search: Efficiently Improving
    Policies in Partially Observable Settings. arXiv:2106.09086. [abstract]
45. Hu, H., Lerer, A., Cui, B., Pineda, L., Brown, N., Foerster, J. (2021). Off-Belief Learning.
    *ICML 2021*. https://proceedings.mlr.press/v139/hu21c.html [abstract]

**Training methods**

46. Vinyals, O. et al. (2019). Grandmaster level in StarCraft II using multi-agent reinforcement
    learning. *Nature* 575:350–354. https://www.nature.com/articles/s41586-019-1724-z [abstract]
47. Jaderberg, M. et al. (2017). Population Based Training of Neural Networks. arXiv:1711.09846.
    [abstract]
48. Jaderberg, M. et al. (2017). Reinforcement Learning with Unsupervised Auxiliary Tasks. *ICLR
    2017*. https://openreview.net/pdf?id=SJ6yPD5xg [abstract]
49. Jacob, A. P., Wu, D. J., Farina, G., Lerer, A., Hu, H., Bakhtin, A., Andreas, J., Brown, N.
    (2022). Modeling Strong and Human-Like Gameplay with KL-Regularized Search. *ICML 2022*.
    https://proceedings.mlr.press/v162/jacob22a.html [abstract]
50. Ng, A. Y., Harada, D., Russell, S. (1999). Policy invariance under reward transformations: theory
    and application to reward shaping. *ICML 1999*. [citation]

**Not found or not verified.** I found no peer-reviewed Canasta AI paper and no peer-reviewed
Žolíky AI paper. I found no peer-reviewed jam/fold equilibrium tables beyond three players. The
detail that SAD uses an auxiliary task of predicting the agent's own cards is from memory and was
not re-checked.
