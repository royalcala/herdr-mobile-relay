const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-393-d6923e8323bae36c/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
