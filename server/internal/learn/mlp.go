package learn

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

// The network a trained bot plays with, and the only place its arithmetic
// lives on the server.
//
// Deliberately small and deliberately hand-written. A few dense layers are a
// hundred lines of Go, and in exchange the server needs no cgo, no ONNX
// runtime in the image, and nothing whose float results could differ between
// the machine that tested a model and the one that serves it. The trainer
// writes the same layout (ml/export.py), and a parity test pins the two to the
// same numbers.
//
// Three stacks, all dense with ReLU between layers:
//
//	trunk   state            -> embedding   (ReLU after every layer)
//	scorer  embedding ++ cand -> one logit   (no activation on the last layer)
//	value   embedding         -> one number  (no activation on the last layer)

// Dense is one fully-connected layer. W is row-major, Out rows of In.
type Dense struct {
	In, Out int
	W, B    []float32
}

func (d *Dense) forward(x, out []float32, relu bool) {
	for o := 0; o < d.Out; o++ {
		sum := d.B[o]
		row := d.W[o*d.In : (o+1)*d.In]
		x := x[:len(row)] // one bounds check here rather than one per weight
		for i, w := range row {
			sum += w * x[i]
		}
		if relu && sum < 0 {
			sum = 0
		}
		out[o] = sum
	}
}

// Net is a loaded policy.
type Net struct {
	Game     string
	StateDim int
	CandDim  int
	Trunk    []Dense
	Scorer   []Dense
	Value    []Dense
}

func run(layers []Dense, x []float32, reluLast bool) []float32 {
	for i := range layers {
		out := make([]float32, layers[i].Out)
		layers[i].forward(x, out, reluLast || i < len(layers)-1)
		x = out
	}
	return x
}

// Embed runs the trunk.
func (n *Net) Embed(state []float32) []float32 { return run(n.Trunk, state, true) }

// Logits scores every candidate against one embedded state.
func (n *Net) Logits(emb []float32, cands [][]float32) []float32 {
	out := make([]float32, len(cands))
	x := make([]float32, len(emb)+n.CandDim)
	copy(x, emb)
	for i, c := range cands {
		copy(x[len(emb):], c)
		out[i] = run(n.Scorer, x, false)[0]
	}
	return out
}

// ValueOf is the value head's estimate for an embedded state.
func (n *Net) ValueOf(emb []float32) float32 { return run(n.Value, emb, false)[0] }

// --- the file format ----------------------------------------------------------
//
//	magic   "ZLNET1\n"
//	uint32  length of the JSON header, little-endian
//	header  {"game","stateDim","candDim","trunk":[[in,out]...],"scorer":...,"value":...}
//	float32 weights, little-endian: trunk, scorer, value; each layer W then B
//
// A header rather than a fixed layout so a model says what it was trained for,
// and a server refuses one that does not match the game's encoder instead of
// playing nonsense with it.

const magic = "ZLNET1\n"

type header struct {
	Game     string   `json:"game"`
	StateDim int      `json:"stateDim"`
	CandDim  int      `json:"candDim"`
	Trunk    [][2]int `json:"trunk"`
	Scorer   [][2]int `json:"scorer"`
	Value    [][2]int `json:"value"`
}

// LoadNet reads a model. It checks that the layers chain, not just that the
// bytes are there: a model that loads but cannot run is worse than one that
// does not load.
func LoadNet(b []byte) (*Net, error) {
	r := bytes.NewReader(b)
	m := make([]byte, len(magic))
	if _, err := io.ReadFull(r, m); err != nil || string(m) != magic {
		return nil, errors.New("learn: not a model file")
	}
	var hlen uint32
	if err := binary.Read(r, binary.LittleEndian, &hlen); err != nil {
		return nil, fmt.Errorf("learn: header length: %w", err)
	}
	hb := make([]byte, hlen)
	if _, err := io.ReadFull(r, hb); err != nil {
		return nil, fmt.Errorf("learn: header: %w", err)
	}
	var h header
	if err := json.Unmarshal(hb, &h); err != nil {
		return nil, fmt.Errorf("learn: header: %w", err)
	}
	n := &Net{Game: h.Game, StateDim: h.StateDim, CandDim: h.CandDim}
	var err error
	if n.Trunk, err = readStack(r, h.Trunk); err != nil {
		return nil, err
	}
	if n.Scorer, err = readStack(r, h.Scorer); err != nil {
		return nil, err
	}
	if n.Value, err = readStack(r, h.Value); err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("learn: %d trailing bytes", r.Len())
	}
	return n, n.check()
}

func readStack(r io.Reader, shape [][2]int) ([]Dense, error) {
	out := make([]Dense, len(shape))
	for i, s := range shape {
		d := Dense{In: s[0], Out: s[1], W: make([]float32, s[0]*s[1]), B: make([]float32, s[1])}
		if err := binary.Read(r, binary.LittleEndian, d.W); err != nil {
			return nil, fmt.Errorf("learn: weights: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, d.B); err != nil {
			return nil, fmt.Errorf("learn: biases: %w", err)
		}
		out[i] = d
	}
	return out, nil
}

func (n *Net) check() error {
	chain := func(name string, layers []Dense, in, out int) error {
		if len(layers) == 0 {
			return fmt.Errorf("learn: %s has no layers", name)
		}
		for _, l := range layers {
			if l.In != in {
				return fmt.Errorf("learn: %s layer takes %d, previous gives %d", name, l.In, in)
			}
			in = l.Out
		}
		if out > 0 && in != out {
			return fmt.Errorf("learn: %s ends in %d, want %d", name, in, out)
		}
		return nil
	}
	if err := chain("trunk", n.Trunk, n.StateDim, 0); err != nil {
		return err
	}
	emb := n.Trunk[len(n.Trunk)-1].Out
	if err := chain("scorer", n.Scorer, emb+n.CandDim, 1); err != nil {
		return err
	}
	return chain("value", n.Value, emb, 1)
}

// Marshal writes a model in the same format LoadNet reads. The trainer is the
// real writer; this exists for tests and for tools that build a model in Go.
func (n *Net) Marshal() ([]byte, error) {
	shape := func(ls []Dense) [][2]int {
		out := make([][2]int, len(ls))
		for i, l := range ls {
			out[i] = [2]int{l.In, l.Out}
		}
		return out
	}
	hb, err := json.Marshal(header{n.Game, n.StateDim, n.CandDim, shape(n.Trunk), shape(n.Scorer), shape(n.Value)})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(magic)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(hb)))
	buf.Write(hb)
	for _, stack := range [][]Dense{n.Trunk, n.Scorer, n.Value} {
		for _, l := range stack {
			_ = binary.Write(&buf, binary.LittleEndian, l.W)
			_ = binary.Write(&buf, binary.LittleEndian, l.B)
		}
	}
	return buf.Bytes(), nil
}

// softmax over logits at a temperature, stable against large logits.
func softmax(logits []float32, temperature float64) []float64 {
	p := make([]float64, len(logits))
	max := math.Inf(-1)
	for _, l := range logits {
		max = math.Max(max, float64(l))
	}
	sum := 0.0
	for i, l := range logits {
		p[i] = math.Exp((float64(l) - max) / temperature)
		sum += p[i]
	}
	for i := range p {
		p[i] /= sum
	}
	return p
}
