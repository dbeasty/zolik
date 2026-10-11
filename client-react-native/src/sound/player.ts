import { createAudioPlayer, type AudioPlayer } from 'expo-audio';
import * as Speech from 'expo-speech';

import { getSoundPrefs } from '@/src/sound/prefs';

/**
 * Plays the board's sounds, honouring the player's choices.
 *
 * Every call checks the preferences at the moment it is made, so the quick
 * button takes effect on the very next card. Nothing here may throw into the
 * game: a browser that refuses audio before the first tap, or a device with no
 * speech voice, simply stays quiet.
 */
export type SoundName = 'card' | 'take' | 'turn' | 'round' | 'win' | 'lose';

const FILES: Record<SoundName, number> = {
  card: require('@/assets/sounds/card.wav'),
  take: require('@/assets/sounds/take.wav'),
  turn: require('@/assets/sounds/turn.wav'),
  round: require('@/assets/sounds/round.wav'),
  win: require('@/assets/sounds/win.wav'),
  lose: require('@/assets/sounds/lose.wav'),
};

const players = new Map<SoundName, AudioPlayer>();

export function playSound(name: SoundName) {
  if (!getSoundPrefs().enabled) return;
  try {
    let p = players.get(name);
    if (!p) {
      p = createAudioPlayer(FILES[name]);
      players.set(name, p);
    }
    void p.seekTo(0);
    p.play();
  } catch {
    // Quiet is a fine failure.
  }
}

/** Says `text` aloud in `language`, cutting off whatever the announcer was still saying. */
export function say(text: string, language?: string) {
  const prefs = getSoundPrefs();
  if (!prefs.enabled || !prefs.voice || !text) return;
  try {
    void Speech.stop();
    Speech.speak(text, { language, rate: 1.05 });
  } catch {
    // As above.
  }
}

/** Cuts off anything speaking — for when the player turns the sound off mid-sentence. */
export function hush() {
  try {
    void Speech.stop();
    players.forEach((p) => p.pause());
  } catch {
    // As above.
  }
}
