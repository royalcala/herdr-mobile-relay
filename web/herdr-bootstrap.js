const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.5-384-c9e7cd1a80d39ff4/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
