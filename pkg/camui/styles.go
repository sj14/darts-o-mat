package camui

// Tailwind classes shared by the pages, written out in full so Tailwind finds them
// (see pkg/frontend/view/tailwind.css, which also scans this package).
const (
	btnBase      = "inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 font-semibold transition active:scale-95 disabled:opacity-40"
	btnPrimary   = btnBase + " bg-board-green text-white shadow-lg shadow-board-green/25 hover:brightness-110"
	btnSecondary = btnBase + " border border-white/15 text-white/80 hover:bg-white/10"
	btnDanger    = btnBase + " border border-board-red/50 text-red-300 hover:bg-board-red/20"
	btnSmall     = "px-3 py-1.5 text-sm"
	linkClass    = "font-semibold text-emerald-300 underline-offset-4 hover:underline"
)
