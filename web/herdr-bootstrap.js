const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-390-bdf6c28fe870d809/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
