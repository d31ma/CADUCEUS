// Mockup helper, not part of the product. URL options:
//   ?theme=light|dark|auto|high-contrast   DuVay theme (w-theme)
//   ?os=macos|windows|linux|ios|android    DuVay platform skin (w-os); Mesail sets it per Tac target
//   ?density=comfortable|compact           DuVay density (w-density)
//   ?pane=list|main|thread                 which pane shows on screens under 600 px
(function applyMockupOptions() {
  var params = new URLSearchParams(window.location.search);
  var root = document.documentElement;
  var map = { theme: "w-theme", os: "w-os", density: "w-density" };
  Object.keys(map).forEach(function (key) {
    var value = params.get(key);
    if (value) root.setAttribute(map[key], value);
  });
  var pane = params.get("pane");
  var app = document.querySelector("[data-pane]");
  if (pane && app) app.setAttribute("data-pane", pane);
})();
