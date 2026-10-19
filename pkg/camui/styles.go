package camui

// Tailwind classes shared by the pages, written out in full so Tailwind finds them
// (see pkg/frontend/view/tailwind.css, which also scans this package).
const (
	// stickyBar stays at the top while the camera image is scrolled: the instruction of the
	// calibration says which point to click next, and every click loads the page again.
	stickyBar    = "sticky top-0 z-10 -mx-4 mb-4 bg-pub-950 px-4 pb-3 pt-2"
	btnBase      = "inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 font-semibold transition active:scale-95 disabled:opacity-40"
	btnPrimary   = btnBase + " bg-board-green text-white shadow-lg shadow-board-green/25 hover:brightness-110"
	btnSecondary = btnBase + " border border-white/15 text-white/80 hover:bg-white/10"
	btnDanger    = btnBase + " border border-board-red/50 text-red-300 hover:bg-board-red/20"
	btnSmall     = "px-3 py-1.5 text-sm"
	linkClass    = "font-semibold text-emerald-300 underline-offset-4 hover:underline"
	inputClass   = "w-full rounded-xl border border-white/10 bg-pub-950/70 px-4 py-2.5 text-white placeholder-white/30 outline-none transition focus:border-board-green focus:ring-2 focus:ring-board-green/40"
)
