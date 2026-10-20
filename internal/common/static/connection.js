// Tells the player when the server can't be reached (it restarts, the network is gone),
// and brings the live part of the page up to date when the connection is back: what
// changed in between was pushed to nobody. htmx alone shows nothing when a request fails.
(function () {
  "use strict";

  function notice(show) {
    const el = document.getElementById("offline");
    if (el) el.hidden = !show;
  }

  // A request that didn't reach the server, or a proxy that has no server behind it.
  document.addEventListener("htmx:sendError", () => notice(true));
  document.addEventListener("htmx:responseError", (e) => {
    if ([502, 503, 504].includes(e.detail.xhr.status)) notice(true);
  });
  document.addEventListener("htmx:afterRequest", (e) => {
    if (e.detail.successful) notice(false);
  });

  // The live stream (data-refresh names the part it swaps): the first open is the page
  // loading, every later one a reconnect, after which that part is fetched again.
  document.addEventListener("htmx:sseError", () => notice(true));
  document.addEventListener("htmx:sseOpen", (e) => {
    notice(false);
    const el = e.target;
    if (!el.dataset.refresh) return;
    if (!el.dataset.opened) {
      el.dataset.opened = "1";
      return;
    }
    const target = el.dataset.refresh;
    htmx.ajax("GET", window.location.href, { target: target, select: target, swap: "outerHTML" });
  });
})();
