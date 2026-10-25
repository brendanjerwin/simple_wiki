/**
 * @Beep audio feedback for page apps: a short synthesized tone via the Web
 * Audio API — no external assets (the wiki ships no audio files; page apps
 * must not load from CDNs).
 *
 * Autoplay policy: browsers only let audio sound after a user gesture
 * unlocks the origin — which only gesture-driven action events (a real
 * Button click) can do. `ensureAudioUnlocked()` runs on every page app
 * action event: it creates/resumes the context while a click's call stack
 * is live, and is a no-op for non-gesture triggers (a Countdown tick is
 * not a gesture). A `@Beep` from a Countdown the user themselves started
 * then plays through that earlier click's unlock; audio on a page where
 * no interaction ever happened is silently skipped — audio is feedback,
 * never control flow.
 */

/** One shared AudioContext across all @Beep steps. */
let sharedAudioContext: AudioContext | undefined;

/**
 * Create (and unlock) the shared AudioContext if needed. Call while a
 * user gesture's call stack is live. No-op where there is no Web Audio.
 */
export function ensureAudioUnlocked(): void {
 try {
  const AudioContextCtor = globalThis.AudioContext;
  if (AudioContextCtor == null) return;
  sharedAudioContext ??= new AudioContextCtor();
  if (sharedAudioContext.state === 'suspended') {
   void sharedAudioContext.resume().catch(() => { });
  }
 } catch {
  // Unlock is best-effort; a blocked Web Audio API never blocks the plan.
 }
}

/**
 * Play a short synthesized tone. `double` plays two quick beeps, `low` a
 * lower single tone, `beep` (default) one 880 Hz tone. Silent no-op while
 * the context does not exist or is not running.
 */
export function playBeepSound(kind: string = 'beep'): void {
 try {
  if (sharedAudioContext == null) return;
  const audio = sharedAudioContext;
  if (audio.state !== 'running') return;
  const toneCount = kind === 'double' ? 2 : 1;
  const frequencyHz = kind === 'low' ? 440 : 880;
  for (let toneIndex = 0; toneIndex < toneCount; toneIndex++) {
   const startAt = audio.currentTime + toneIndex * 0.2;
   const oscillator = audio.createOscillator();
   const gain = audio.createGain();
   oscillator.type = 'sine';
   oscillator.frequency.value = frequencyHz;
   gain.gain.setValueAtTime(0.0001, startAt);
   gain.gain.exponentialRampToValueAtTime(0.25, startAt + 0.01);
   gain.gain.exponentialRampToValueAtTime(0.0001, startAt + 0.15);
   oscillator.connect(gain).connect(audio.destination);
   oscillator.start(startAt);
   oscillator.stop(startAt + 0.16);
  }
 } catch {
  // A missing/blocked Web Audio API never blocks the action plan's other steps.
 }
}

/** Test seam: forget the shared AudioContext between isolated tests. */
export function resetAudioContextForTest(): void {
 sharedAudioContext = undefined;
}