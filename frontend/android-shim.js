// Android bridge shim. On the desktop (Wails) this file does nothing.
// On Android, MainActivity exposes `window.NovelBridge`; this shim gives the
// page the same `window.go.main.App.*` and `window.runtime.EventsOn` API the
// desktop has, so app.js runs unchanged on both.
(function () {
  if (!window.NovelBridge || (window.go && window.go.main)) return;

  let nextId = 1;
  const pending = new Map();
  const listeners = new Map();

  // Called by Java with the result of a bridged call.
  window.__naResolve = function (id, ok, payload) {
    const p = pending.get(id);
    if (!p) return;
    pending.delete(id);
    if (ok) {
      try { p.resolve(payload === '' ? null : JSON.parse(payload)); }
      catch (e) { p.resolve(payload); }
    } else {
      p.reject(new Error(payload));
    }
  };

  // Called by Java for engine events (progress, done, error, ...).
  window.__naEmit = function (event, payload) {
    let data = payload;
    try { data = JSON.parse(payload); } catch (e) {}
    (listeners.get(event) || []).forEach(function (fn) {
      try { fn(data); } catch (e) { console.error(e); }
    });
  };

  function call(method, args) {
    return new Promise(function (resolve, reject) {
      const id = nextId++;
      pending.set(id, { resolve: resolve, reject: reject });
      window.NovelBridge.call(id, method, JSON.stringify(args));
    });
  }

  window.go = {
    main: {
      App: new Proxy({}, {
        get: function (_, method) {
          if (typeof method !== 'string') return undefined;
          return function () { return call(method, Array.prototype.slice.call(arguments)); };
        }
      })
    }
  };

  window.runtime = {
    EventsOn: function (event, fn) {
      if (!listeners.has(event)) listeners.set(event, []);
      listeners.get(event).push(fn);
      return function () {
        listeners.set(event, (listeners.get(event) || []).filter(function (f) { return f !== fn; }));
      };
    },
    EventsEmit: function () {},
    BrowserOpenURL: function (url) { window.location.href = url; }
  };
})();
