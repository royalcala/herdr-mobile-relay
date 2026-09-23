const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-392-2622dcb6cf7d0bdd/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
