const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-386-c7e1f8d43c713d22/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
