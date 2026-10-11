// Sounds of the game page, synthesized with the Web Audio API (no audio files).
//
// The server marks the board with the kind of the last log entry (data-sound) and its
// id (data-sound-id). A sound plays when the board is swapped in with a newer entry,
// i.e. for darts entered on this device, on other devices or by the cameras. An older
// last entry means the last one was undone. Loading the page or correcting a dart
// (same entry) makes no sound.
(() => {
  "use strict";

  const storageKey = "darts-o-mat-sound";
  let ctx = null;
  let lastID = null;

  function enabled() {
    try {
      return localStorage.getItem(storageKey) !== "off";
    } catch {
      return true;
    }
  }

  function setEnabled(on) {
    try {
      localStorage.setItem(storageKey, on ? "on" : "off");
    } catch {}
  }

  // Browsers only allow audio after the user interacted with the page.
  function audio() {
    if (!ctx) {
      const AudioContext = window.AudioContext || window.webkitAudioContext;
      if (!AudioContext) return null;
      ctx = new AudioContext();
    }
    if (ctx.state === "suspended") ctx.resume();
    return ctx;
  }

  // tone plays an oscillator from start (seconds from now) for dur seconds.
  function tone(freq, start, dur, { type = "sine", gain = 0.2, slideTo = 0, attack = 0.005 } = {}) {
    const a = audio();
    if (!a) return;
    const t = a.currentTime + start;
    const osc = a.createOscillator();
    const amp = a.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(freq, t);
    if (slideTo) osc.frequency.exponentialRampToValueAtTime(slideTo, t + dur);
    amp.gain.setValueAtTime(0.0001, t);
    amp.gain.exponentialRampToValueAtTime(gain, t + attack);
    amp.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    osc.connect(amp).connect(a.destination);
    osc.start(t);
    osc.stop(t + dur + 0.05);
  }

  // noise plays filtered white noise, e.g. the thud of a dart.
  function noise(start, dur, { gain = 0.3, filter = "lowpass", freq = 900, sweepTo = 0 } = {}) {
    const a = audio();
    if (!a) return;
    const t = a.currentTime + start;
    const buffer = a.createBuffer(1, Math.ceil(a.sampleRate * dur), a.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1;
    const src = a.createBufferSource();
    src.buffer = buffer;
    const f = a.createBiquadFilter();
    f.type = filter;
    f.frequency.setValueAtTime(freq, t);
    if (sweepTo) f.frequency.exponentialRampToValueAtTime(sweepTo, t + dur);
    const amp = a.createGain();
    amp.gain.setValueAtTime(gain, t);
    amp.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    src.connect(f).connect(amp).connect(a.destination);
    src.start(t);
  }

  function thud() {
    noise(0, 0.07, { gain: 0.5, freq: 1200 });
    tone(150, 0, 0.12, { gain: 0.35, slideTo: 70 });
  }

  function arpeggio(notes, step, opts) {
    notes.forEach((f, i) => tone(f, i * step, step * 2.2, opts));
  }

  const C5 = 523.25, E5 = 659.25, G5 = 783.99, C6 = 1046.5, E6 = 1318.5;

  const sounds = {
    dart: thud,
    double() {
      thud();
      tone(E5, 0.05, 0.18, { type: "triangle", gain: 0.18 });
      tone(G5, 0.12, 0.22, { type: "triangle", gain: 0.18 });
    },
    triple() {
      thud();
      arpeggio([E5, G5, C6], 0.06, { type: "triangle", gain: 0.2 });
    },
    bull() {
      thud();
      tone(C6, 0.05, 0.4, { type: "sine", gain: 0.2 });
      tone(E6, 0.05, 0.4, { type: "sine", gain: 0.12 });
    },
    hit() {
      thud();
      tone(C6, 0.04, 0.25, { type: "sine", gain: 0.22 });
    },
    miss() {
      noise(0, 0.12, { gain: 0.25, freq: 400 });
      tone(220, 0, 0.35, { type: "triangle", gain: 0.25, slideTo: 90 });
    },
    bust() {
      // A sad trombone.
      [392, 370, 349].forEach((f, i) => tone(f, i * 0.32, 0.34, { type: "sawtooth", gain: 0.12, attack: 0.03 }));
      tone(330, 0.96, 0.9, { type: "sawtooth", gain: 0.12, slideTo: 300, attack: 0.03 });
    },
    180() {
      thud();
      arpeggio([C5, E5, G5, C6], 0.09, { type: "square", gain: 0.08 });
      [C5, E5, G5, C6].forEach((f) => tone(f, 0.45, 1.2, { type: "triangle", gain: 0.12, attack: 0.02 }));
    },
    win() {
      thud();
      arpeggio([C5, E5, G5, C6, E6], 0.11, { type: "square", gain: 0.08 });
      [C5, G5, C6, E6].forEach((f) => tone(f, 0.65, 1.6, { type: "triangle", gain: 0.12, attack: 0.02 }));
    },
    undo() {
      // The dart flies back out: a rising whoosh and a falling blip.
      noise(0, 0.25, { gain: 0.12, filter: "bandpass", freq: 400, sweepTo: 2500 });
      tone(660, 0.05, 0.2, { type: "triangle", gain: 0.15, slideTo: 330 });
    },
    takeout() {
      noise(0, 0.35, { gain: 0.12, filter: "bandpass", freq: 2500, sweepTo: 400 });
    },
  };

  function checkBoard() {
    const board = document.getElementById("board");
    if (!board || !board.dataset.soundId) return;
    const id = Number(board.dataset.soundId);
    if (lastID !== null && enabled()) {
      const play = id > lastID ? sounds[board.dataset.sound] : id < lastID ? sounds.undo : null;
      if (play) play();
    }
    lastID = id;
  }

  function updateToggle() {
    const toggle = document.getElementById("sound-toggle");
    if (!toggle) return;
    toggle.hidden = !document.getElementById("board");
    toggle.textContent = enabled() ? "🔊 Ton an" : "🔇 Ton aus";
    toggle.setAttribute("aria-pressed", enabled() ? "true" : "false");
  }

  document.addEventListener("click", (e) => {
    const toggle = e.target.closest("#sound-toggle");
    if (toggle) {
      setEnabled(!enabled());
      updateToggle();
      if (enabled()) sounds.hit();
      return;
    }
    if (enabled()) audio();
  });

  // htmx:load fires for the page and for everything htmx swaps in (darts, SSE pushes).
  document.addEventListener("htmx:load", () => {
    checkBoard();
    updateToggle();
  });
})();
