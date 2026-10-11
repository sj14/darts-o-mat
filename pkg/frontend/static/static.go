// Package static contains the files served under /static.
package static

import "embed"

// htmx.min.js: htmx 2.0.11 (https://htmx.org)
// htmx-ext-sse.min.js: htmx SSE extension 2.2.4 (https://htmx.org/extensions/sse/)
// bootstrap.min.css: Bootstrap 6.0.0-alpha.1 (https://getbootstrap.com), without the JavaScript part
//
//go:embed *.css *.js
var FS embed.FS
