// Mockup helper, not part of the product: ?theme=light|dark|auto|high-contrast sets DuVay's theme,
// and ?pane=list|main|thread picks the visible pane on narrow screens.
(function applyMockupOptions() {
  var params = new URLSearchParams(window.location.search);
  var theme = params.get("theme");
  if (theme) document.documentElement.setAttribute("w-theme", theme);
  var pane = params.get("pane");
  var app = document.querySelector(".ms-app");
  if (pane && app) app.setAttribute("data-pane", pane);
})();
