// Contrast gate for the Fog Desktop palette ("Quiet" direction).
//
// Run with `npm run contrast`. Exits non-zero if any token pair drops below its
// WCAG 2.1 floor, so a palette edit can't silently ship an unreadable pair.
// The values here must mirror src/tokens.css — if you change a token there,
// change it here and re-run.
//
// This gate is not decoration: it caught four real failures when the palette
// was derived from the design mock, including control borders at 1.32:1.
//
// OKLCH -> sRGB -> WCAG 2.1 contrast ratio.

function oklchToSrgb(L, C, Hdeg) {
  const h = (Hdeg * Math.PI) / 180;
  const a = C * Math.cos(h);
  const b = C * Math.sin(h);

  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;

  const l = l_ ** 3, m = m_ ** 3, s = s_ ** 3;

  const lr = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const lg = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const lb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;

  const enc = (v) =>
    v <= 0.0031308 ? 12.92 * v : 1.055 * Math.sign(v) * Math.abs(v) ** (1 / 2.4) - 0.055;

  return [enc(lr), enc(lg), enc(lb)].map((v) => Math.min(1, Math.max(0, v)));
}

function luminance([r, g, b]) {
  const lin = (v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function ratio(c1, c2) {
  const a = luminance(oklchToSrgb(...c1));
  const b = luminance(oklchToSrgb(...c2));
  const [hi, lo] = a > b ? [a, b] : [b, a];
  return (hi + 0.05) / (lo + 0.05);
}

// Mirrors src/tokens.css.
const T = {
  paper:      [0.168, 0.004, 264],
  paper2:     [0.191, 0.006, 271],
  paper3:     [0.218, 0.008, 275],
  paper4:     [0.232, 0.010, 277],
  rule:       [0.264, 0.008, 264],
  rule2:      [0.310, 0.011, 271],
  field:      [0.520, 0.012, 273],
  ink:        [0.937, 0.003, 265],
  ink2:       [0.710, 0.010, 273],
  ink3:       [0.620, 0.013, 275],
  accent:     [0.861, 0.173, 92],
  accentDim:  [0.795, 0.162, 86],
  accentInk:  [0.197, 0.022, 104],
  add:        [0.800, 0.182, 152],
  del:        [0.711, 0.166, 22],
  warn:       [0.837, 0.164, 84],
  info:       [0.828, 0.101, 230],
};

// Surfaces a given foreground actually appears on, worst case last.
const checks = [
  ["ink on paper",             T.ink,       T.paper,  4.5],
  ["ink on paper-2",           T.ink,       T.paper2, 4.5],
  ["ink on paper-4",           T.ink,       T.paper4, 4.5],
  ["ink-2 on paper",           T.ink2,      T.paper,  4.5],
  ["ink-2 on paper-3",         T.ink2,      T.paper3, 4.5],
  ["ink-2 on paper-4",         T.ink2,      T.paper4, 4.5],
  ["ink-3 on paper",           T.ink3,      T.paper,  4.5],
  ["ink-3 on paper-2",         T.ink3,      T.paper2, 4.5],
  ["ink-3 on paper-3",         T.ink3,      T.paper3, 4.5],
  ["ink-3 on paper-4",         T.ink3,      T.paper4, 4.5],
  ["accent on paper",          T.accent,    T.paper,  4.5],
  ["accent on paper-2",        T.accent,    T.paper2, 4.5],
  ["accent-ink ON accent",     T.accentInk, T.accent, 4.5],
  ["accent-ink ON accent-dim", T.accentInk, T.accentDim, 4.5],
  ["signal-add on paper",      T.add,       T.paper,  4.5],
  ["signal-add on paper-2",    T.add,       T.paper2, 4.5],
  ["signal-del on paper",      T.del,       T.paper,  4.5],
  ["signal-del on paper-2",    T.del,       T.paper2, 4.5],
  ["signal-warn on paper",     T.warn,      T.paper,  4.5],
  ["signal-info on paper",     T.info,      T.paper,  4.5],
  // Control boundaries: WCAG 1.4.11 wants 3:1. --color-rule is exempt by
  // design (decorative separators only); --color-field is not.
  ["field border on paper",    T.field,     T.paper,  3.0],
  ["field border on paper-2",  T.field,     T.paper2, 3.0],
  ["field border on paper-4",  T.field,     T.paper4, 3.0],
  ["focus ring on paper",      T.accent,    T.paper,  3.0],
  ["focus ring on paper-4",    T.accent,    T.paper4, 3.0],
];

let fails = 0;
console.log("pair                          ratio   floor  result");
console.log("─".repeat(57));
for (const [name, fg, bg, floor] of checks) {
  const r = ratio(fg, bg);
  const ok = r >= floor;
  if (!ok) fails++;
  console.log(
    `${name.padEnd(29)} ${r.toFixed(2).padStart(5)}   ${floor.toFixed(1)}    ${ok ? "PASS" : "*** FAIL ***"}`
  );
}
console.log("─".repeat(57));
console.log(fails === 0 ? "All pairs pass." : `${fails} FAILING PAIR(S)`);

if (fails > 0) process.exit(1);
