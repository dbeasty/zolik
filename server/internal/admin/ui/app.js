/* The operator console.
 *
 * No framework and no build step: it is one screen, and shipping it as source
 * inside the binary means there is no second artefact that can be a different
 * version from the API it talks to.
 *
 * No inline script or style anywhere, which is what lets ui.go serve the
 * strictest CSP there is (`script-src 'self'`). That is worth more here than
 * on most pages: this console is reached over an SSH tunnel by somebody
 * holding an admin token. */
(function () {
  'use strict';

  var TOKEN_KEY = 'zolik.admin.token';
  var token = null;
  try { token = localStorage.getItem(TOKEN_KEY); } catch (e) { token = null; }

  var $ = function (id) { return document.getElementById(id); };

  function show(el, on) { el.hidden = !on; }

  function fail(el, message) {
    el.textContent = message;
    show(el, !!message);
  }

  /* ------------------------------------------------------------------ api */

  function api(path, options) {
    options = options || {};
    var headers = options.headers || {};
    if (token) headers['Authorization'] = 'Bearer ' + token;
    return fetch('/admin/api' + path, {
      method: options.method || 'GET',
      headers: headers,
      body: options.body
    }).then(function (res) {
      if (res.status === 401 || res.status === 403) {
        // The token expired or the password changed underneath us — changing
        // the password invalidates every issued token by design, since the
        // signing key is derived from the hash.
        signOut();
        throw new Error('Signed out. Sign in again.');
      }
      if (!res.ok) {
        return res.text().then(function (t) {
          throw new Error(t.trim() || ('Request failed (' + res.status + ')'));
        });
      }
      return res.json();
    });
  }

  /* -------------------------------------------------------------- sign in */

  function signOut() {
    token = null;
    try { localStorage.removeItem(TOKEN_KEY); } catch (e) { /* private mode */ }
    show($('console'), false);
    show($('signin'), true);
  }

  function signedIn(newToken) {
    token = newToken;
    try { localStorage.setItem(TOKEN_KEY, token); } catch (e) { /* private mode */ }
    show($('signin'), false);
    show($('console'), true);
    start();
  }

  $('password-form').addEventListener('submit', function (ev) {
    ev.preventDefault();
    fail($('signin-error'), '');
    fetch('/admin/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: $('username').value,
        password: $('password').value
      })
    }).then(function (res) {
      if (!res.ok) {
        return res.text().then(function (t) {
          throw new Error(t.trim() || 'Sign-in failed');
        });
      }
      return res.json();
    }).then(function (body) {
      $('password').value = '';
      signedIn(body.token);
    }).catch(function (err) {
      fail($('signin-error'), err.message);
    });
  });

  $('signout').addEventListener('click', signOut);

  /* --------------------------------------------------------------- render */

  function tile(parent, label, value, sub) {
    var el = document.createElement('div');
    el.className = 'tile';
    var v = document.createElement('div');
    v.className = 'tile-value';
    v.textContent = value;
    var l = document.createElement('div');
    l.className = 'tile-label';
    l.textContent = label;
    el.appendChild(v);
    el.appendChild(l);
    if (sub) {
      var s = document.createElement('div');
      s.className = 'sub muted';
      s.textContent = sub;
      el.appendChild(s);
    }
    parent.appendChild(el);
    return el;
  }

  function chip(parent, label, value) {
    var el = document.createElement('span');
    el.className = 'chip';
    el.textContent = label + ' · ' + value;
    parent.appendChild(el);
  }

  function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); }

  function num(n) { return (n || 0).toLocaleString(); }

  /* Empty buckets are off by default. A month-long range over a quiet week is
   * mostly rows of zeroes, and a table you scroll past nothing to read is a
   * table nobody reads. lastReport lets the toggle redraw without refetching:
   * the rows are already here, only the filter changed. */
  var showQuiet = false;
  var lastReport = null;

  function percent(rate) {
    return Math.round((rate || 0) * 100) + '%';
  }

  function renderStatus(s) {
    var el = $('live-tiles');
    clear(el);
    $('version').textContent = s.version ? 'build ' + s.version : '';
    if (s.live) {
      tile(el, 'matches in progress', num(s.live.matches));
      tile(el, 'connections', num(s.live.connections));
      tile(el, 'in the waiting room', num(s.live.waiting));
    }
    if (s.capacity) {
      var c = s.capacity;
      var sub = c.maxConnections ? 'ceiling ' + num(c.maxConnections) : '';
      tile(el, 'held slots', num(c.live), sub);
      if (c.memoryFraction) {
        tile(el, 'memory used', Math.round(c.memoryFraction * 100) + '%',
          c.accepting ? 'accepting' : 'refusing new sockets');
      }
      if (!c.waitingRoomOpen) {
        tile(el, 'waiting room', 'closed', 'under pressure');
      }
    }
  }

  function renderReport(rep) {
    $('range-note').textContent =
      rep.from + ' to ' + rep.to + ', by ' + rep.bucket + ' — all days are UTC';

    var t = rep.totals;
    var el = $('totals');
    clear(el);
    tile(el, 'games played', num(t.matches.completed));
    tile(el, 'people', num(t.players.distinct),
      t.players.isFloor ? 'at least — the daily cap was reached' : '');
    tile(el, 'new accounts', num(t.users.registered));
    tile(el, 'new guests', num(t.users.guests));
    tile(el, 'unfinished', num(t.matches.abandoned),
      'walked away from · ' + num(t.matches.neverStarted) + ' never started');
    tile(el, 'finished', percent(t.matches.completionRate),
      'of games that started');
    tile(el, 'turned away', num(t.admission.total),
      num(t.admission.matchStartDenied) + ' refused a new game');
    tile(el, 'crashes', num(t.ops.uncleanBoots),
      num(t.ops.boots) + ' starts');

    renderChart(rep.buckets);

    var mods = $('modules');
    clear(mods);
    if (!t.matches.byModule.length) {
      mods.appendChild(document.createTextNode('Nothing was played in this range.'));
    }
    t.matches.byModule.forEach(function (m) { chip(mods, m.moduleId, num(m.matches)); });

    var refs = $('refusals');
    clear(refs);
    if (!t.admission.byReason.length) {
      refs.appendChild(document.createTextNode('Nobody was turned away.'));
      $('refusal-note').textContent = '';
    } else {
      t.admission.byReason.forEach(function (r) { chip(refs, r.reason, num(r.count)); });
      $('refusal-note').textContent =
        'memory_pressure is the server protecting itself from an OOM kill; ' +
        'waiting_room_closed is it declining to grow while still serving every game in progress.';
    }

    lastReport = rep;
    renderTable(rep.buckets, rep.bucket);
  }

  /* A bar per bucket, drawn with divs rather than a charting library: the CSP
   * forbids a CDN, and at one series it would be more code to configure one
   * than to draw this. */
  function renderChart(buckets) {
    var el = $('chart');
    clear(el);
    var peak = buckets.reduce(function (m, b) {
      return Math.max(m, b.matches.completed + b.matches.abandoned);
    }, 0);


    buckets.forEach(function (b) {
      var col = document.createElement('div');
      col.className = 'chart-col';
      var done = b.matches.completed;
      var gone = b.matches.abandoned;
      // A range with nothing in it draws a row of empty columns rather than
      // a message: the bars are zero, and zero is the answer.
      var height = peak ? Math.round(((done + gone) / peak) * 100) : 0;

      var bar = document.createElement('div');
      bar.className = 'chart-bar';
      bar.style.height = height + '%';
      if (done + gone === 0) bar.setAttribute('data-empty', '1');
      // Abandoned games are shaded into the same bar rather than given their
      // own: they are part of how much was played, and a second series would
      // invite reading them as extra activity.
      if (done + gone > 0) {
        bar.style.background =
          'linear-gradient(to top, var(--bar) ' +
          Math.round((done / (done + gone)) * 100) + '%, var(--danger) 0)';
      }
      bar.title = b.label + ': ' + done + ' finished, ' + gone + ' abandoned' +
        (b.partial ? ' (partial bucket)' : '');
      col.appendChild(bar);

      var label = document.createElement('div');
      label.className = 'muted';
      // Only the day part, and only on some columns: thirty full dates side
      // by side is an unreadable smear.
      label.textContent = shortLabel(b.label);
      if (b.partial) label.textContent += '*';
      col.appendChild(label);

      el.appendChild(col);
    });
  }

  function shortLabel(label) {
    var m = /^\d{4}-(\d{2})-(\d{2})$/.exec(label);
    if (m) return m[1] + '/' + m[2];
    return label.replace(/^\d{4}-/, '');
  }

  /* One row's cells, in the order the header names them.
   *
   * Emptiness is decided from this same list rather than from a second copy
   * of the field names, because the two would drift: a column added to the
   * table and forgotten in the filter would start hiding rows that have a
   * number in them, which is the one failure this feature must not have. */
  function rowCells(b) {
    return [
      { text: b.label + (b.partial ? ' *' : '') },
      { n: b.matches.created },
      { n: b.matches.started },
      { n: b.matches.completed },
      { n: b.matches.abandoned },
      { n: b.matches.neverStarted },
      { n: b.players.distinct, suffix: b.players.isFloor ? '+' : '' },
      { n: b.users.registered },
      { n: b.users.guests },
      { n: b.admission.total },
      { n: b.ops.uncleanBoots }
    ];
  }

  /* A bucket in which nothing at all happened: every number the table would
   * print is zero. Not the same as a bucket with no *matches* — a day that
   * turned somebody away or booted uncleanly is a day worth reading. */
  function isQuiet(b) {
    return rowCells(b).every(function (c) {
      return c.text !== undefined || (c.n || 0) === 0;
    });
  }

  function renderTable(buckets, noun) {
    var table = $('table');
    clear(table);
    var head = ['bucket', 'created', 'started', 'finished', 'abandoned',
      'never started', 'people', 'accounts', 'guests', 'turned away', 'crashes'];
    var thead = document.createElement('thead');
    var hr = document.createElement('tr');
    head.forEach(function (h) {
      var th = document.createElement('th');
      th.textContent = h;
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    table.appendChild(thead);

    var quiet = 0;
    var tbody = document.createElement('tbody');
    buckets.forEach(function (b) {
      var isEmpty = isQuiet(b);
      if (isEmpty) {
        quiet++;
        if (!showQuiet) return;
      }
      var tr = document.createElement('tr');
      // Kept legible but visibly not the thing to read, so that asking for
      // them back does not undo the reason they were hidden.
      if (isEmpty) tr.className = 'quiet';
      rowCells(b).forEach(function (c) {
        var td = document.createElement('td');
        td.textContent = c.text !== undefined ? c.text : num(c.n) + (c.suffix || '');
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);

    renderQuietNote(quiet, noun);
  }

  /* Say what was left out, and offer it back.
   *
   * A table that silently drops rows is one an operator misreads as a shorter
   * range than they asked for — "we only have four days of data" is a much
   * worse conclusion than the empty rows were a nuisance. The chart above
   * still draws every bucket, so the shape of a quiet stretch stays visible
   * even while its rows are not. */
  function renderQuietNote(quiet, noun) {
    var note = $('empty-note');
    clear(note);
    show(note, quiet > 0);
    if (!quiet) return;

    var plural = quiet === 1 ? '' : 's';
    note.appendChild(document.createTextNode(
      (showQuiet ? 'Showing ' : 'Hiding ') + quiet + ' empty ' + noun + plural +
      ' — nothing was created, played, refused or crashed. '));

    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'link';
    btn.textContent = showQuiet ? 'Hide them' : 'Show them';
    btn.addEventListener('click', function () {
      showQuiet = !showQuiet;
      if (lastReport) renderTable(lastReport.buckets, lastReport.bucket);
    });
    note.appendChild(btn);
  }

  /* ----------------------------------------------------------------- load */

  function query() {
    var q = '?bucket=' + encodeURIComponent($('bucket').value);
    if ($('from').value) q += '&from=' + encodeURIComponent($('from').value);
    if ($('to').value) q += '&to=' + encodeURIComponent($('to').value);
    return q;
  }

  function load() {
    fail($('error'), '');
    api('/status').then(renderStatus).catch(function (err) {
      fail($('error'), err.message);
    });
    api('/report' + query()).then(renderReport).catch(function (err) {
      fail($('error'), err.message);
    });
  }

  $('refresh').addEventListener('click', load);
  $('bucket').addEventListener('change', load);

  $('csv').addEventListener('click', function () {
    // The download has to carry the Authorization header, so it is fetched and
    // handed to the browser as a blob rather than being a plain link — a link
    // would arrive unauthenticated and 401.
    fetch('/admin/api/report' + query() + '&format=csv', {
      headers: { 'Authorization': 'Bearer ' + token }
    }).then(function (res) {
      if (!res.ok) throw new Error('Could not export (' + res.status + ')');
      return res.blob();
    }).then(function (blob) {
      var url = URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = 'zolik-report.csv';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    }).catch(function (err) {
      fail($('error'), err.message);
    });
  });

  function start() {
    api('/session').then(function (s) {
      $('who').textContent = s.username + (s.viaPassword ? '' : ' · ' + s.email);
      load();
    }).catch(function (err) {
      fail($('error'), err.message);
    });
  }

  /* ----------------------------------------------------------------- boot */

  // Default range: the last thirty days, which is the question somebody
  // opening this without choosing anything is asking.
  var today = new Date();
  var thirty = new Date(today.getTime() - 29 * 86400000);
  $('to').value = today.toISOString().slice(0, 10);
  $('from').value = thirty.toISOString().slice(0, 10);

  fetch('/admin/api/methods').then(function (res) { return res.json(); })
    .then(function (m) {
      show($('password-form'), m.password);
      show($('no-doors'), !m.password && !m.allowList);
      if (!m.password && m.allowList) {
        $('signin-intro').textContent =
          'This console admits allow-listed accounts. Sign in to the game first, then return here.';
      }
    });

  if (token) {
    show($('console'), true);
    start();
  } else {
    show($('signin'), true);
  }
})();
