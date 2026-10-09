// The page's side of the Mac app's bridge (Sources/Jokerless/Bridge.swift).
//
// Runs before the web client's bundle. It replaces fetch and WebSocket for
// every http(s) and ws(s) address with versions that hand the request to the
// app, which makes it through the game core. The web view itself has no
// network: the page's content security policy allows connections to the
// app's own pages only, so anything that slips past these replacements is
// refused by the engine rather than sent.
//
// It also provides the zolik-nearby module (globalThis.ZolikNearbyDesktop),
// the same interface the iPhone and Android apps implement natively.
(function () {
  'use strict';
  if (window.__zolikDesktop) return;
  var handler = window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.zolik;
  if (!handler) return;

  function post(msg) {
    return handler.postMessage(msg);
  }

  // ---- base64 ---------------------------------------------------------------

  function bytesToBase64(bytes) {
    var s = '';
    for (var i = 0; i < bytes.length; i += 0x8000) {
      s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
    }
    return btoa(s);
  }

  function base64ToBytes(b64) {
    var bin = atob(b64 || '');
    var out = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  // ---- fetch ----------------------------------------------------------------

  var nativeFetch = window.fetch.bind(window);
  var nextId = 1;

  function isRemote(url) {
    return /^(https?|wss?):$/i.test(url.protocol);
  }

  window.fetch = function (input, init) {
    var request;
    try {
      request = new Request(input, init);
    } catch (e) {
      return Promise.reject(e);
    }
    var url = new URL(request.url, location.href);
    if (!isRemote(url)) return nativeFetch(input, init);

    var signal = request.signal;
    if (signal && signal.aborted) return Promise.reject(new DOMException('Aborted', 'AbortError'));

    var headers = {};
    request.headers.forEach(function (value, key) {
      headers[key] = value;
    });
    var hasBody = request.method !== 'GET' && request.method !== 'HEAD';
    var bodyPromise = hasBody ? request.arrayBuffer() : Promise.resolve(null);

    var answer = bodyPromise.then(function (buf) {
      return post({
        op: 'fetch',
        id: nextId++,
        method: request.method,
        url: url.href,
        headers: headers,
        body: buf && buf.byteLength ? bytesToBase64(new Uint8Array(buf)) : null,
      });
    });

    var result = answer.then(
      function (res) {
        var status = res.status;
        var nullBody = status === 101 || status === 204 || status === 205 || status === 304;
        var response = new Response(nullBody ? null : base64ToBytes(res.body), {
          status: status < 200 || status > 599 ? 502 : status,
          headers: res.headers || {},
        });
        try {
          Object.defineProperty(response, 'url', { value: url.href });
        } catch (e) {}
        return response;
      },
      function (err) {
        throw new TypeError('Load failed: ' + (err && err.message ? err.message : err));
      },
    );

    if (!signal) return result;
    return new Promise(function (resolve, reject) {
      signal.addEventListener('abort', function () {
        reject(new DOMException('Aborted', 'AbortError'));
      });
      result.then(resolve, reject);
    });
  };

  // ---- WebSocket ------------------------------------------------------------

  var sockets = {};

  function DesktopSocket(url, protocols) {
    var parsed = new URL(url, location.href);
    if (parsed.protocol === 'http:') parsed.protocol = 'ws:';
    if (parsed.protocol === 'https:') parsed.protocol = 'wss:';
    this.url = parsed.href;
    this.protocol = '';
    this.extensions = '';
    this.binaryType = 'blob';
    this.bufferedAmount = 0;
    this.readyState = 0;
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    this._listeners = {};
    this._id = nextId++;
    sockets[this._id] = this;
    var self = this;
    post({ op: 'ws.open', id: this._id, url: this.url }).catch(function () {
      self._closed(1006, '');
    });
  }

  DesktopSocket.CONNECTING = 0;
  DesktopSocket.OPEN = 1;
  DesktopSocket.CLOSING = 2;
  DesktopSocket.CLOSED = 3;
  DesktopSocket.prototype.CONNECTING = 0;
  DesktopSocket.prototype.OPEN = 1;
  DesktopSocket.prototype.CLOSING = 2;
  DesktopSocket.prototype.CLOSED = 3;

  DesktopSocket.prototype.addEventListener = function (type, fn) {
    (this._listeners[type] = this._listeners[type] || []).push(fn);
  };
  DesktopSocket.prototype.removeEventListener = function (type, fn) {
    var list = this._listeners[type] || [];
    var i = list.indexOf(fn);
    if (i >= 0) list.splice(i, 1);
  };
  DesktopSocket.prototype.dispatchEvent = function (ev) {
    var handler = this['on' + ev.type];
    if (typeof handler === 'function') handler.call(this, ev);
    (this._listeners[ev.type] || []).slice().forEach(function (fn) {
      fn.call(this, ev);
    }, this);
    return true;
  };
  DesktopSocket.prototype.send = function (data) {
    if (this.readyState === 0) throw new DOMException('Still in CONNECTING state.', 'InvalidStateError');
    if (this.readyState !== 1) return;
    var self = this;
    post({ op: 'ws.send', id: this._id, data: String(data) }).catch(function () {
      self._closed(1006, '');
    });
  };
  DesktopSocket.prototype.close = function (code, reason) {
    if (this.readyState >= 2) return;
    this.readyState = 2;
    post({ op: 'ws.close', id: this._id, code: code || 1000, reason: reason || '' });
  };
  DesktopSocket.prototype._opened = function () {
    if (this.readyState !== 0) return;
    this.readyState = 1;
    this.dispatchEvent(new Event('open'));
  };
  DesktopSocket.prototype._message = function (data) {
    if (this.readyState !== 1) return;
    this.dispatchEvent(new MessageEvent('message', { data: data, origin: new URL(this.url).origin }));
  };
  DesktopSocket.prototype._closed = function (code, reason) {
    if (this.readyState === 3) return;
    var wasClean = code === 1000 || this.readyState === 2;
    this.readyState = 3;
    delete sockets[this._id];
    if (!wasClean && code === 1006) this.dispatchEvent(new Event('error'));
    this.dispatchEvent(new CloseEvent('close', { code: code || 1006, reason: reason || '', wasClean: wasClean }));
  };

  var NativeWebSocket = window.WebSocket;
  window.WebSocket = function (url, protocols) {
    return new DesktopSocket(url, protocols);
  };
  window.WebSocket.prototype = DesktopSocket.prototype;
  ['CONNECTING', 'OPEN', 'CLOSING', 'CLOSED'].forEach(function (k) {
    window.WebSocket[k] = DesktopSocket[k];
  });
  void NativeWebSocket;

  // ---- the zolik-nearby module ---------------------------------------------

  var state = {
    host: null,
    relay: { status: 'off', code: '', url: '', guests: 0 },
    addresses: [],
    bleState: 'unknown',
    bleCodes: [],
    replicaReady: false,
    identity: null,
  };
  var listeners = {};
  applyState(window.__ZOLIK_DESKTOP_STATE__);

  function applyState(s) {
    if (s && typeof s === 'object') state = Object.assign({}, state, s);
  }

  function call(method, args) {
    return post({ op: 'nearby', method: method, args: args || [] }).then(function (res) {
      applyState(res && res.state);
      return res ? res.value : null;
    });
  }

  function asyncMethod(name) {
    return function () {
      return call(name, Array.prototype.slice.call(arguments));
    };
  }

  var nearby = {
    nodeIdentity: function () {
      return state.identity;
    },
    replicaReady: function () {
      return !!state.replicaReady;
    },
    hostStatus: function () {
      return state.host;
    },
    relayStatus: function () {
      return state.relay;
    },
    localAddresses: function () {
      return state.addresses || [];
    },
    bleState: function () {
      return state.bleState || 'unknown';
    },
    bleGuestCodes: function () {
      return state.bleCodes || [];
    },
    hostResumed: function () {
      void call('hostResumed');
    },
    randomBytes: function (count) {
      var n = Math.max(0, Math.min(count | 0, 1024));
      var b = new Uint8Array(n);
      crypto.getRandomValues(b);
      return bytesToBase64(b);
    },
    addListener: function (event, cb) {
      (listeners[event] = listeners[event] || []).push(cb);
      return {
        remove: function () {
          var list = listeners[event] || [];
          var i = list.indexOf(cb);
          if (i >= 0) list.splice(i, 1);
        },
      };
    },
  };
  [
    'startHost', 'startNode', 'syncNow', 'followMatch', 'stopHost', 'openRoom', 'closeRoom',
    'openRelay', 'closeRelay', 'startBrowsing', 'stopBrowsing', 'bleHostStart', 'bleHostStop',
    'bleScanStart', 'bleScanStop', 'bleConnect', 'bleSend', 'bleDisconnect', 'bleReady',
  ].forEach(function (name) {
    nearby[name] = asyncMethod(name);
  });
  Object.defineProperty(globalThis, 'ZolikNearbyDesktop', { value: Object.freeze(nearby) });

  // ---- events from the app --------------------------------------------------

  window.__zolikDesktop = Object.freeze({
    event: function (msg) {
      var name = msg && msg.name;
      var p = msg && msg.payload;
      if (name === 'ws') {
        var s = sockets[p.id];
        if (!s) return;
        if (p.type === 'open') s._opened();
        else if (p.type === 'message') s._message(p.data);
        else if (p.type === 'close') s._closed(p.code, p.reason);
      } else if (name === 'nearby') {
        (listeners[p.event] || []).slice().forEach(function (cb) {
          try {
            cb(p.payload);
          } catch (e) {
            console.error(e);
          }
        });
      } else if (name === 'state') {
        applyState(p);
      }
    },
  });
})();
