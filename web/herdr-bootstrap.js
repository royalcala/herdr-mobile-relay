const e = new URL(window.__HERDR_ENTRY__ || "/builds/0.21.6-385-e3d0fee5ec31edb2/index.html", location);
  e.search = location.search;
  e.hash = location.hash;
  location.replace(e);
