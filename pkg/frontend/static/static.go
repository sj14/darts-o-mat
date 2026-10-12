// Package static contains the files served under /static.
package static

import "embed"

// htmx.min.js: htmx 2.0.11 (https://htmx.org)
// htmx-ext-sse.min.js: htmx SSE extension 2.2.4 (https://htmx.org/extensions/sse/)
// tailwind.css: built from pkg/frontend/view/tailwind.css with Tailwind CSS (see go:generate below)
// barlow-condensed-*.woff2: Barlow Condensed (https://github.com/jpt/barlow), SIL Open Font License, see barlow-condensed-OFL.txt
// favicon.svg: the board logo of the page header; apple-touch-icon.png is it as 180×180 PNG for iOS
//
//go:embed *.css *.js *.woff2 *.txt *.svg *.png
var FS embed.FS

//go:generate go run ../../../tools/tailwind --input ../view/tailwind.css --output tailwind.css --minify
