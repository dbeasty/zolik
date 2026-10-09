// Helpers for the end-to-end runner (e2e/macos), injected only into a debug
// build started with ZOLIK_E2E_CONTROL. Commands the runner sends call these
// as window.e2e.*.
(function () {
  'use strict';
  var post = function (line) {
    try {
      window.webkit.messageHandlers.zolik.postMessage({ op: 'log', line: line });
    } catch (e) {}
  };
  ['log', 'info', 'warn', 'error'].forEach(function (level) {
    var orig = console[level].bind(console);
    console[level] = function () {
      var parts = Array.prototype.map.call(arguments, function (a) {
        if (a instanceof Error) return a.stack || String(a);
        if (typeof a === 'object') {
          try {
            return JSON.stringify(a);
          } catch (e) {}
        }
        return String(a);
      });
      post(level + ': ' + parts.join(' ').slice(0, 2000));
      orig.apply(null, arguments);
    };
  });
  window.addEventListener('error', function (e) {
    post('pageerror: ' + (e.message || '') + ' ' + (e.filename || '') + ':' + (e.lineno || ''));
  });
  window.addEventListener('unhandledrejection', function (e) {
    post('unhandledrejection: ' + (e.reason && (e.reason.stack || e.reason.message) || e.reason));
  });
  document.addEventListener('securitypolicyviolation', function (e) {
    window.__cspViolations = (window.__cspViolations || 0) + 1;
    post('csp: blocked ' + e.blockedURI + ' (' + e.violatedDirective + ')');
  });

  function visible(el) {
    if (!el || !el.getClientRects().length) return false;
    var s = getComputedStyle(el);
    return s.visibility !== 'hidden' && s.display !== 'none' && Number(s.opacity) !== 0;
  }

  // Whether the element is the one a click at its centre would reach. A
  // screen the stack keeps mounted underneath is "visible" by every style
  // measure, but never on top.
  function onTop(el) {
    var r = el.getBoundingClientRect();
    var x = r.left + r.width / 2;
    var y = r.top + r.height / 2;
    if (x < 0 || y < 0 || x > innerWidth || y > innerHeight) return null;
    var hit = document.elementFromPoint(x, y);
    return !!hit && (hit === el || el.contains(hit));
  }

  // Of several candidates, the first one on top; failing that (it may be
  // scrolled out of view), the first one at all.
  function best(list) {
    for (var i = 0; i < list.length; i++) if (onTop(list[i]) === true) return list[i];
    for (var j = 0; j < list.length; j++) if (onTop(list[j]) === null) return list[j];
    return list[0] || null;
  }

  function byText(text, exact) {
    var want = String(text).trim();
    var all = document.querySelectorAll('[role="button"],button,a,[role="link"],[role="tab"],[role="menuitem"],[role="checkbox"],[role="switch"],[tabindex]');
    var hits = [];
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      var t = (el.innerText || el.getAttribute('aria-label') || '').trim();
      if (exact ? t === want : t.indexOf(want) >= 0) hits.push(el);
    }
    // The innermost match: a button inside a card both read the same text.
    hits = hits.filter(function (el) {
      return visible(el) && !hits.some(function (o) { return o !== el && el.contains(o) && visible(o); });
    });
    return best(hits);
  }

  function sleep(ms) {
    return new Promise(function (r) { setTimeout(r, ms); });
  }

  async function waitFor(fn, what, timeout) {
    var end = Date.now() + (timeout || 15000);
    var last;
    while (Date.now() < end) {
      try {
        last = fn();
        if (last) return last;
      } catch (e) {}
      await sleep(100);
    }
    throw new Error('timed out waiting for ' + what);
  }

  function hasText(text) {
    return document.body && document.body.innerText.indexOf(text) >= 0;
  }

  // React Native Web's Pressable listens to pointer events, not a bare
  // click, so a press is pointerdown + pointerup + click on the element.
  function press(el) {
    el.scrollIntoView({ block: 'center' });
    var r = el.getBoundingClientRect();
    var opts = { bubbles: true, cancelable: true, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2, pointerId: 1, pointerType: 'mouse', isPrimary: true, button: 0, buttons: 1 };
    el.dispatchEvent(new PointerEvent('pointerdown', opts));
    el.dispatchEvent(new MouseEvent('mousedown', opts));
    opts.buttons = 0;
    el.dispatchEvent(new PointerEvent('pointerup', opts));
    el.dispatchEvent(new MouseEvent('mouseup', opts));
    el.dispatchEvent(new MouseEvent('click', opts));
  }

  function setValue(input, value) {
    var proto = input.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  }

  window.e2e = {
    sleep: sleep,
    waitFor: waitFor,
    hasText: hasText,
    waitForText: function (text, timeout) {
      return waitFor(function () { return hasText(text); }, 'text "' + text + '"', timeout);
    },
    waitForGone: function (text, timeout) {
      return waitFor(function () { return !hasText(text); }, 'text "' + text + '" to go', timeout);
    },
    click: async function (text, opts) {
      opts = opts || {};
      var el = await waitFor(function () { return byText(text, opts.exact); }, 'a control "' + text + '"', opts.timeout);
      // After the command has answered: a press that navigates would
      // otherwise take the command's page away before it could reply.
      setTimeout(function () { press(el); }, 0);
      await sleep(50);
      return true;
    },
    clickTestId: async function (id, timeout) {
      var el = await waitFor(function () {
        var e = document.querySelector('[data-testid="' + id + '"]');
        return e && visible(e) ? e : null;
      }, 'testID ' + id, timeout);
      setTimeout(function () { press(el); }, 0);
      await sleep(50);
      return true;
    },
    fill: async function (placeholderOrLabel, value, timeout) {
      var input = await waitFor(function () {
        var all = document.querySelectorAll('input,textarea');
        var hits = [];
        for (var i = 0; i < all.length; i++) {
          var el = all[i];
          if (!visible(el)) continue;
          // Never a field on a screen the stack keeps underneath.
          if (onTop(el) === false) continue;
          if (el.placeholder === placeholderOrLabel || el.getAttribute('aria-label') === placeholderOrLabel || el.dataset.testid === placeholderOrLabel) hits.push(el);
        }
        return best(hits);
      }, 'a field "' + placeholderOrLabel + '"', timeout);
      input.focus();
      setValue(input, value);
      return true;
    },
    text: function () {
      return document.body ? document.body.innerText : '';
    },
    buttons: function () {
      return Array.prototype.map.call(document.querySelectorAll('[role="button"],button'), function (b) {
        return (b.innerText || b.getAttribute('aria-label') || '').trim();
      }).filter(Boolean);
    },
  };
})();
