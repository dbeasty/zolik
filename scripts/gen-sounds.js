#!/usr/bin/env node
/**
 * Synthesises the board's sound effects into client-react-native/assets/sounds.
 *
 * Generated rather than recorded so there is no licence to carry and the
 * sounds can be retuned here: `node scripts/gen-sounds.js`. 16-bit mono PCM at
 * 22.05 kHz — a few KB each, which is why they are committed.
 */
const fs = require('fs');
const path = require('path');

const RATE = 22050;
const out = path.join(__dirname, '..', 'client-react-native', 'assets', 'sounds');
fs.mkdirSync(out, { recursive: true });

/** Sum of notes: { f, at, dur, gain, type, slide }. Each has a fast attack and an exponential decay. */
function render(notes, tail = 0.05) {
  const total = Math.max(...notes.map((n) => n.at + n.dur)) + tail;
  const buf = new Float32Array(Math.ceil(total * RATE));
  let seed = 12345;
  const rand = () => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff) * 2 - 1;
  for (const n of notes) {
    const start = Math.floor(n.at * RATE);
    const len = Math.floor(n.dur * RATE);
    let phase = 0;
    let lp = 0;
    for (let i = 0; i < len && start + i < buf.length; i++) {
      const t = i / RATE;
      const f = n.slide ? n.f + (n.slide - n.f) * (i / len) : n.f;
      phase += (2 * Math.PI * f) / RATE;
      const env = Math.min(1, t / 0.004) * Math.exp((-5 * i) / len) * (1 - i / len);
      let s;
      if (n.type === 'noise') {
        lp += 0.14 * (rand() - lp); // low cutoff: a dull rustle, not a hiss
        s = lp;
      } else if (n.type === 'tri') s = (2 / Math.PI) * Math.asin(Math.sin(phase));
      else s = Math.sin(phase) + 0.25 * Math.sin(2 * phase);
      buf[start + i] += s * env * (n.gain ?? 0.5);
    }
  }
  return buf;
}

function wav(samples) {
  const data = Buffer.alloc(samples.length * 2);
  samples.forEach((s, i) => data.writeInt16LE(Math.round(Math.max(-1, Math.min(1, s)) * 32000), i * 2));
  const h = Buffer.alloc(44);
  h.write('RIFF', 0);
  h.writeUInt32LE(36 + data.length, 4);
  h.write('WAVEfmt ', 8);
  h.writeUInt32LE(16, 16);
  h.writeUInt16LE(1, 20);
  h.writeUInt16LE(1, 22);
  h.writeUInt32LE(RATE, 24);
  h.writeUInt32LE(RATE * 2, 28);
  h.writeUInt16LE(2, 32);
  h.writeUInt16LE(16, 34);
  h.write('data', 36);
  h.writeUInt32LE(data.length, 40);
  return Buffer.concat([h, data]);
}

const note = (f, at, dur, extra = {}) => ({ f, at, dur, ...extra });
// An octave below concert pitch: the board's sounds are deliberately deep.
const C5 = 261.63, E5 = 329.63, G5 = 392, C6 = 523.25, A4 = 220, F4 = 174.61, D4 = 146.83;

const sounds = {
  // A card laid on felt: a short noise slap over a low thump.
  card: [note(0, 0, 0.07, { type: 'noise', gain: 1.1 }), note(95, 0, 0.11, { slide: 55, gain: 0.7 })],
  // A pile gathered up: a rising rustle and two soft ticks.
  take: [note(0, 0, 0.22, { type: 'noise', gain: 0.8 }), note(165, 0.02, 0.12, { slide: 260, type: 'tri', gain: 0.45 }), note(260, 0.12, 0.14, { type: 'tri', gain: 0.4 })],
  // One card lifted off the discards: a quick flick and a bright upward chirp,
  // about half the length of a whole pile.
  pick: [note(0, 0, 0.05, { type: 'noise', gain: 0.7 }), note(330, 0.01, 0.1, { slide: 620, type: 'tri', gain: 0.5 })],
  // Your turn: one soft bell.
  turn: [note(E5, 0, 0.35, { gain: 0.4 }), note(E5 * 2, 0, 0.2, { gain: 0.12 })],
  // A round settles: a falling-then-rising pair.
  round: [note(G5, 0, 0.22, { gain: 0.45 }), note(C5, 0.18, 0.4, { gain: 0.5 })],
  // The match is won: a rising arpeggio.
  win: [note(C5, 0, 0.25), note(E5, 0.14, 0.25), note(G5, 0.28, 0.25), note(C6, 0.42, 0.7, { gain: 0.55 })],
  // The match is lost: a gentle fall.
  lose: [note(A4, 0, 0.3, { type: 'tri' }), note(F4, 0.22, 0.3, { type: 'tri' }), note(D4, 0.44, 0.7, { type: 'tri' })],
};

for (const [name, notes] of Object.entries(sounds)) {
  fs.writeFileSync(path.join(out, `${name}.wav`), wav(render(notes)));
}
console.log(`wrote ${Object.keys(sounds).length} sounds to ${out}`);
