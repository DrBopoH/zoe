// Единая цветовая схема банков: и для плиток на карточке Лица (tile), и для панелей «Операционной» (panel).
const THEMES = {
  raif: {
    tile: 'bg-gradient-to-br from-amber-400/40 via-yellow-600/25 to-amber-950 border-amber-400/70 text-amber-100',
    panel: 'bg-gradient-to-br from-amber-400/30 via-yellow-600/20 to-amber-950/80 border-amber-400/60 text-amber-100',
    color: '#fbbf24', // фирменный цвет: маркеры на графике и т.п.
    tag: 'RAIF',
    tagStyle: 'bg-amber-400/20 text-amber-300 border-amber-400/40'
  },
  privat: {
    tile: 'bg-gradient-to-br from-emerald-600/30 via-emerald-900/30 to-slate-950 border-emerald-500/50 text-emerald-100',
    panel: 'bg-gradient-to-br from-emerald-600/30 via-emerald-900/20 to-slate-950 border-emerald-500/50 text-emerald-100',
    color: '#10b981', // фирменный цвет: маркеры на графике и т.п.
    tag: 'PRIVAT',
    tagStyle: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40'
  },
  fop: {
    tile: 'bg-gradient-to-br from-slate-600 via-slate-800 to-slate-950 border-slate-400/50 text-slate-100',
    panel: 'bg-gradient-to-br from-slate-600 via-slate-800 to-slate-950 border-slate-400/50 text-slate-100',
    color: '#94a3b8', // фирменный цвет: маркеры на графике и т.п.
    tag: 'FOP',
    tagStyle: 'bg-slate-500/20 text-slate-300 border-slate-400/40'
  },
  white: {
    tile: 'bg-gradient-to-br from-slate-100 via-slate-200 to-slate-400 border-slate-300 text-slate-900',
    panel: 'bg-gradient-to-br from-slate-100 via-slate-200 to-slate-400 border-slate-300 text-slate-900',
    color: '#e2e8f0', // фирменный цвет: маркеры на графике и т.п.
    tag: 'WHITE',
    tagStyle: 'bg-slate-900/30 text-slate-900 border-slate-400'
  },
  mono: {
    tile: 'bg-gradient-to-br from-zinc-700 via-zinc-950 to-black border-zinc-500/70 text-zinc-100',
    panel: 'bg-gradient-to-br from-zinc-700 via-zinc-900 to-black border-zinc-500/70 text-zinc-100',
    color: '#a1a1aa', // фирменный цвет: маркеры на графике и т.п.
    tag: 'MONO',
    tagStyle: 'bg-zinc-700/40 text-zinc-200 border-zinc-500/50'
  },
  icu: {
    tile: 'bg-gradient-to-br from-blue-700/40 via-blue-950/60 to-slate-950 border-blue-500/60 text-blue-100',
    panel: 'bg-gradient-to-br from-blue-700/40 via-blue-950/60 to-slate-950 border-blue-500/60 text-blue-100',
    color: '#93c5fd', // фирменный цвет: маркеры на графике и т.п.
    tag: 'OVDP',
    tagStyle: 'bg-blue-500/20 text-blue-300 border-blue-500/40'
  },
  binance: {
    tile: 'bg-gradient-to-br from-yellow-600/30 via-zinc-900 to-black border-yellow-500/60 text-yellow-100',
    panel: 'bg-gradient-to-br from-yellow-600/30 via-zinc-900 to-black border-yellow-500/60 text-yellow-100',
    color: '#F0B90B', // фирменный цвет: маркеры на графике и т.п.
    tag: 'BINANCE',
    tagStyle: 'bg-yellow-500/20 text-yellow-300 border-yellow-500/40'
  },
  other: {
    tile: 'bg-gradient-to-br from-slate-800 to-slate-950 border-slate-700 text-slate-300',
    panel: 'bg-slate-900/60 border-slate-800 text-slate-200',
    color: '#64748b', // фирменный цвет: маркеры на графике и т.п.
    tag: '',
    tagStyle: 'bg-slate-800 text-slate-300 border-slate-700'
  }
}

// bankKey: mono | raif | privat | other (см. detectBankKey), type — тип счёта Mono (black / white / fop …)
export const bankTheme = (bankKey, type = '') => {
  const t = String(type || '').toLowerCase()
  if (bankKey === 'raif') return THEMES.raif
  if (bankKey === 'privat') return THEMES.privat
  if (bankKey === 'icu') return THEMES.icu
  if (bankKey === 'binance') return THEMES.binance
  if (bankKey === 'mono') {
    if (t === 'fop') return THEMES.fop
    if (t === 'white') return THEMES.white
    return t ? { ...THEMES.mono, tag: t.toUpperCase() } : THEMES.mono
  }
  return THEMES.other
}
