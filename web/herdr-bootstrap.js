const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-386-ea4a9595760773f9/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
