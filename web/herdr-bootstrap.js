const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-391-765c466086224f55/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
