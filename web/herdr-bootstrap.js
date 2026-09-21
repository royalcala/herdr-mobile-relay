const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.4-383-d89055611f7b2746/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
