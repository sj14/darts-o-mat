package view

// Tailwind classes shared by the pages. They are written out in full here, so Tailwind
// finds them when scanning the Go files (see tailwind.css).
const (
	btnBase      = "inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 font-semibold transition active:scale-95 disabled:opacity-40"
	btnPrimary   = btnBase + " bg-board-green text-white shadow-lg shadow-board-green/25 hover:brightness-110"
	btnAccent    = btnBase + " bg-amber-400 text-pub-950 shadow-lg shadow-amber-400/20 hover:bg-amber-300"
	btnSecondary = btnBase + " border border-white/15 text-white/80 hover:bg-white/10"
	btnDanger    = btnBase + " border border-board-red/50 text-red-300 hover:bg-board-red/20"
	btnSmall     = "px-3 py-1.5 text-sm"

	inputClass = "w-full rounded-xl border border-white/10 bg-pub-950/70 px-4 py-2.5 text-white placeholder-white/30 outline-none transition focus:border-board-green focus:ring-2 focus:ring-board-green/40"
	labelClass = "mb-2 block text-xs font-semibold uppercase tracking-widest text-white/50"
	hintClass  = "mt-1.5 text-sm text-white/50"
	linkClass  = "font-semibold text-emerald-300 underline-offset-4 hover:underline"
)

// Alert is an error message, with a link to the game or tournament it is about if there
// is one.
type Alert struct {
	Text       string
	Game       int64
	Tournament int64
}
