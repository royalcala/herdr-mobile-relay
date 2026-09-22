const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-387-48a9ef4c19d9b59c/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
