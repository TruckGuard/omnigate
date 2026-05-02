// Categorical colors for gate badges — 8 hues cycling by gate index
const COLORS = [
  { bg: '#eef2ff', fg: '#3730a3', dot: '#4f46e5' }, // indigo
  { bg: '#e0f2fe', fg: '#075985', dot: '#0284c7' }, // sky
  { bg: '#ccfbf1', fg: '#115e59', dot: '#0d9488' }, // teal
  { bg: '#dcfce7', fg: '#166534', dot: '#16a34a' }, // emerald
  { bg: '#fef3c7', fg: '#92400e', dot: '#d97706' }, // amber
  { bg: '#ffe4e6', fg: '#9f1239', dot: '#e11d48' }, // rose
  { bg: '#ede9fe', fg: '#5b21b6', dot: '#7c3aed' }, // violet
  { bg: '#f1f5f9', fg: '#334155', dot: '#64748b' }, // slate
];

/** Return bg/fg/dot colors for a 1-based gate index (cycles every 8). */
export function gateColor(idx: number) {
  return COLORS[((idx - 1) % 8 + 8) % 8];
}

/** Deterministic index from a gate_id string. */
export function gateIndex(gateId: string): number {
  let h = 0;
  for (let i = 0; i < gateId.length; i++) h = (h * 31 + gateId.charCodeAt(i)) >>> 0;
  return (h % 8) + 1;
}
