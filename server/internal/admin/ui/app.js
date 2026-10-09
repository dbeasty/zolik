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
    if (options.body) headers['Content-Type'] = 'application/json';
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

  /* ----------------------------------------------------------------- bots */

  /* One row per game that ships a trained model. The switch reaches live
   * tables at their next bot move, so every change asks first, and the card
   * is redrawn from what the server answers rather than from what was asked
   * for. */
  function when(iso) {
    if (!iso) return '';
    var d = new Date(iso);
    return isNaN(d) ? iso : d.toISOString().replace('T', ' ').slice(0, 16) + ' UTC';
  }

  function cell(tr, className) {
    var td = document.createElement('td');
    if (className) td.className = className;
    tr.appendChild(td);
    return td;
  }

  function line(parent, text, className) {
    var el = document.createElement('div');
    if (className) el.className = className;
    el.textContent = text;
    parent.appendChild(el);
    return el;
  }

  /* The governor panel: the mode as three pressed-or-not buttons, where the
   * mode came from, and what the governor and the monitor are doing now. */
  var MODES = [
    { id: 'off', label: 'Off' },
    { id: 'observe', label: 'Observe' },
    { id: 'enforce', label: 'Enforce' }
  ];

  function renderGovernor(g) {
    var panel = $('governor');
    show(panel, !!g);
    if (!g) return;

    var modes = $('governor-modes');
    clear(modes);
    MODES.forEach(function (m) {
      var b = document.createElement('button');
      b.type = 'button';
      b.textContent = m.label;
      b.setAttribute('data-mode', m.id);
      b.setAttribute('aria-pressed', g.mode === m.id ? 'true' : 'false');
      b.addEventListener('click', function () { setGovernor(g, m.id, modes); });
      modes.appendChild(b);
    });

    var src = $('governor-source');
    src.textContent = g.source === 'console'
      ? 'Set here by ' + (g.updatedBy || 'unknown') + ', ' + when(g.updatedAt) +
        '. The server\'s BOT_GOVERNOR (' + g.envDefault + ') no longer applies.'
      : 'From the server\'s BOT_GOVERNOR setting (' + g.envDefault + '); never changed here.';

    var stats = $('governor-stats');
    clear(stats);
    var mon = g.monitor || {};
    var gov = g.governor || {};
    var level = document.createElement('span');
    level.className = 'chip level-' + (mon.level || 'green');
    level.textContent = 'level · ' + (mon.level || 'green') +
      (mon.reason && mon.level !== 'green' ? ' (' + mon.reason.replace('_', ' ') + ')' : '');
    stats.appendChild(level);
    chip(stats, 'CPU quota', (mon.cpuQuota || 0).toLocaleString(undefined, { maximumFractionDigits: 2 }) + ' cores');
    chip(stats, 'leased', (gov.leasedCores || 0).toFixed(2) + ' of ' + (gov.capacityCores || 0).toFixed(2) + ' cores');
    chip(stats, 'bot seats tracked', num(gov.seats));
    chip(stats, g.mode === 'observe' ? 'would simplify' : 'simplified', num(gov.reducedSeats));
    if (gov.revocationsTotal) chip(stats, 'downgrades since start', num(gov.revocationsTotal));
  }

  function setGovernor(g, to, group) {
    if (to === g.mode) return;
    var question = {
      off: 'Turn the bot governor off?\n\nBots play at their seated strength whatever the load.',
      observe: 'Set the bot governor to observe?\n\nIt keeps counting what it would do, and changes no move. ' +
        'Simplified seats go back to full strength at their next turn.',
      enforce: 'Let the bot governor enforce?\n\nWhen this server is short of CPU, the dearest bot seats ' +
        'play a cheaper version from their next turn, and go back up at the next round when there is room.'
    }[to];
    if (!window.confirm(question)) return;
    Array.prototype.forEach.call(group.querySelectorAll('button'), function (b) { b.disabled = true; });
    fail($('bots-error'), '');
    api('/governor', {
      method: 'PUT',
      body: JSON.stringify({ mode: to })
    }).then(renderBots).catch(function (err) {
      fail($('bots-error'), err.message);
      loadBots();
    });
  }

  function renderBots(body) {
    renderGovernor(body.governor);
    var table = $('bots');
    clear(table);
    var thead = document.createElement('thead');
    var hr = document.createElement('tr');
    ['game', 'model', 'AI seats play', 'last changed'].forEach(function (h) {
      var th = document.createElement('th');
      th.textContent = h;
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    table.appendChild(thead);

    var tbody = document.createElement('tbody');
    (body.games || []).forEach(function (g) {
      var m = g.model || {};
      var tr = document.createElement('tr');

      var name = cell(tr);
      line(name, g.title || g.game, 'name');
      line(name, g.game, 'sub');

      var model = cell(tr, 'bot-model');
      if (!g.embedded) {
        line(model, 'No model in this build.', 'sub');
      } else {
        line(model, m.benchmark || '');
        line(model, 'trained ' + (m.trained || '?') + ' · ' + (m.sourceRun || ''), 'sub');
        line(model, (m.encoder || '') + ' · ' +
          Math.round((m.bytes || 0) / 1024).toLocaleString() + ' KiB · sha256 ' +
          (m.sha256 || '').slice(0, 12), 'sub');
      }
      if (!g.fits) {
        line(model, 'Cannot be turned on: ' + (g.problem || 'the model does not fit this game.'), 'bot-warn');
      }
      if (g.envOverride) {
        line(model, 'Overridden: this server\'s environment names ' + g.envOverride +
          ', which AI seats play whatever this switch says.', 'bot-warn');
      }

      var state = cell(tr, 'bot-switch');
      var label = document.createElement('span');
      label.className = 'bot-state';
      label.setAttribute('data-on', g.enabled ? '1' : '0');
      label.textContent = g.enabled ? 'Trained model' : 'Heuristic';
      state.appendChild(label);
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.setAttribute('aria-pressed', g.enabled ? 'true' : 'false');
      btn.className = g.enabled ? 'danger' : '';
      btn.textContent = g.enabled ? 'Turn off' : 'Turn on';
      btn.disabled = !g.enabled && !g.fits;
      btn.addEventListener('click', function () { toggleBot(g, btn); });
      state.appendChild(btn);

      var changed = cell(tr);
      if (g.updatedAt) {
        line(changed, when(g.updatedAt));
        line(changed, 'by ' + (g.updatedBy || 'unknown'), 'sub');
      } else {
        line(changed, 'never — off by default', 'sub');
      }

      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
  }

  function toggleBot(g, btn) {
    var to = !g.enabled;
    var title = g.title || g.game;
    var question = to
      ? 'Put the trained model in every ' + title + ' AI seat?\n\n' +
        'Live tables switch at each seat\'s next turn. Easy, Medium and Hard seats are unchanged.'
      : 'Put the heuristic back in every ' + title + ' AI seat?\n\n' +
        'Live tables switch at each seat\'s next turn.';
    if (!window.confirm(question)) return;
    btn.disabled = true;
    fail($('bots-error'), '');
    api('/bots/' + encodeURIComponent(g.game), {
      method: 'PUT',
      body: JSON.stringify({ enabled: to })
    }).then(renderBots).catch(function (err) {
      fail($('bots-error'), err.message);
      loadBots();
    });
  }

  function loadBots() {
    api('/bots').then(renderBots).catch(function (err) {
      fail($('bots-error'), err.message);
    });
  }

  /* ------------------------------------------------------------- accounts */

  /* Country names come from the browser rather than a table shipped here: the
   * server only knows ISO codes, and Intl already knows every name. */
  var regionNames = null;
  try { regionNames = new Intl.DisplayNames(['en'], { type: 'region' }); } catch (e) { regionNames = null; }

  function regionName(code) {
    if (!code) return 'unknown';
    try { return (regionNames && regionNames.of(code)) || code; } catch (e) { return code; }
  }

  function flag(code) {
    if (!code || code.length !== 2) return '';
    return String.fromCodePoint.apply(null, code.toUpperCase().split('').map(function (c) {
      return 0x1f1e6 + c.charCodeAt(0) - 65;
    })) + ' ';
  }

  function day(iso) {
    if (!iso) return '—';
    var d = new Date(iso);
    return isNaN(d) ? iso : d.toISOString().slice(0, 10);
  }

  /* "3 days ago" for anything recent, the date for anything older: the
   * question is usually "are they still here", which a date makes you work out. */
  function ago(iso) {
    if (!iso) return '—';
    var d = new Date(iso);
    if (isNaN(d)) return iso;
    var mins = Math.round((Date.now() - d.getTime()) / 60000);
    if (mins < 2) return 'just now';
    if (mins < 60) return mins + ' min ago';
    var hours = Math.round(mins / 60);
    if (hours < 36) return hours + ' h ago';
    var days = Math.round(hours / 24);
    if (days < 45) return days + ' days ago';
    return day(iso);
  }

  function hbars(el, rows, total, label) {
    clear(el);
    var peak = rows.reduce(function (m, r) { return Math.max(m, r.count); }, 0);
    if (!rows.length) {
      el.appendChild(document.createTextNode('No accounts yet.'));
      return;
    }
    rows.forEach(function (r) {
      var row = document.createElement('div');
      row.className = 'hbar';
      if (!r.key) row.setAttribute('data-unknown', '1');
      var name = document.createElement('div');
      name.className = 'hbar-label';
      name.textContent = label(r.key);
      name.title = r.key || 'no time zone reported yet';
      var track = document.createElement('div');
      track.className = 'hbar-track';
      var fill = document.createElement('div');
      fill.className = 'hbar-fill';
      fill.style.width = (peak ? Math.round((r.count / peak) * 100) : 0) + '%';
      track.appendChild(fill);
      var n = document.createElement('div');
      n.className = 'hbar-n';
      n.textContent = num(r.count);
      n.title = total ? Math.round((r.count / total) * 100) + '% of accounts' : '';
      row.appendChild(name);
      row.appendChild(track);
      row.appendChild(n);
      el.appendChild(row);
    });
  }

  function renderSignups(rows) {
    var el = $('signups');
    clear(el);
    var peak = rows.reduce(function (m, r) { return Math.max(m, r.count); }, 0);
    rows.forEach(function (r) {
      var col = document.createElement('div');
      col.className = 'chart-col';
      var bar = document.createElement('div');
      bar.className = 'chart-bar';
      bar.style.height = (peak ? Math.round((r.count / peak) * 100) : 0) + '%';
      if (!r.count) bar.setAttribute('data-empty', '1');
      bar.title = r.key + ': ' + num(r.count) + ' new accounts';
      col.appendChild(bar);
      var label = document.createElement('div');
      label.className = 'muted';
      label.textContent = r.key.slice(5);
      col.appendChild(label);
      el.appendChild(col);
    });
  }

  function renderAccounts(rep) {
    var t = rep.totals;
    $('accounts-note').textContent = 'active = played on that UTC day; window ' + rep.windowDays + ' days';

    var el = $('account-tiles');
    clear(el);
    tile(el, 'accounts', num(t.accounts),
      num(t.verifiedEmail) + ' with a verified email');
    tile(el, 'have played', num(t.played),
      num(t.neverPlayed) + ' never finished a match');
    tile(el, 'active today', num(t.active1), num(t.guests1) + ' guests too');
    tile(el, 'active 7 days', num(t.active7), num(t.guests7) + ' guests too');
    tile(el, 'active 30 days', num(t.active30), num(t.guests30) + ' guests too');
    tile(el, 'returning', num(t.returning30), 'active, joined before the window');
    tile(el, 'new 7 days', num(t.new7), num(t.new30) + ' in 30 days');

    hbars($('regions'), rep.byRegion, t.accounts, function (code) {
      return code ? flag(code) + regionName(code) : 'unknown';
    });
    $('region-note').textContent =
      'From the time zone each device reports when it refreshes its session — never from an IP. ' +
      num(t.regionKnown) + ' of ' + num(t.accounts) + ' accounts have reported one so far.';

    renderSignups(rep.signups || []);

    var games = $('account-games');
    clear(games);
    if (!rep.byGame.length) games.appendChild(document.createTextNode('No account has finished a match yet.'));
    rep.byGame.forEach(function (g) {
      chip(games, g.moduleId, num(g.accounts) + ' players · ' + num(g.matches) + ' matches');
    });

    var eng = $('engagement');
    clear(eng);
    rep.engagement.forEach(function (b) { chip(eng, b.key, num(b.count) + ' accounts'); });

    var prov = $('providers');
    clear(prov);
    rep.byProvider.forEach(function (c) { chip(prov, c.key, num(c.count)); });

    var lang = $('languages');
    clear(lang);
    rep.byLanguage.forEach(function (c) { chip(lang, c.key, num(c.count)); });

    renderPlayers(rep);
  }

  function renderPlayers(rep) {
    var shown = rep.users.length;
    $('players-note').textContent = rep.matched === shown
      ? num(shown) + ' accounts'
      : 'Top ' + num(shown) + ' of ' + num(rep.matched) + ' accounts';

    var table = $('players');
    clear(table);
    var thead = document.createElement('thead');
    var hr = document.createElement('tr');
    ['#', 'player', 'where', 'joined', 'last seen', 'last match', 'matches', 'won',
      'vs people', 'vs bots', 'favourite game', 'active days'].forEach(function (h) {
      var th = document.createElement('th');
      th.textContent = h;
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    table.appendChild(thead);

    var tbody = document.createElement('tbody');
    rep.users.forEach(function (u, i) {
      var tr = document.createElement('tr');
      cell(tr).textContent = String(i + 1);
      var who = cell(tr, 'player');
      line(who, u.username, 'name');
      line(who, (u.email || 'no email') + (u.email && !u.emailVerified ? ' (unverified)' : '') +
        ' · ' + (u.provider || 'unknown'), 'sub');
      var where = cell(tr);
      where.textContent = u.region ? flag(u.region) + regionName(u.region) : '—';
      if (u.timeZone) where.title = u.timeZone + (u.language ? ' · ' + u.language : '');
      cell(tr).textContent = day(u.createdAt);
      var seen = cell(tr);
      seen.textContent = ago(u.lastSeenAt);
      seen.title = when(u.lastSeenAt);
      var last = cell(tr);
      last.textContent = ago(u.lastMatchAt);
      last.title = when(u.lastMatchAt);
      cell(tr).textContent = num(u.matches);
      cell(tr).textContent = u.matches ? percent(u.winRate) : '—';
      cell(tr).textContent = num(u.vsHumans);
      cell(tr).textContent = num(u.vsAI);
      var fav = cell(tr);
      fav.textContent = u.favouriteGame || '—';
      if (u.games > 1) fav.textContent += ' +' + (u.games - 1);
      cell(tr).textContent = u.activeDays + ' / ' + rep.windowDays;
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
  }

  function loadAccounts() {
    fail($('accounts-error'), '');
    var q = '?sort=' + encodeURIComponent($('players-sort').value) +
      '&limit=' + encodeURIComponent($('players-limit').value);
    var search = $('players-search').value.trim();
    if (search) q += '&q=' + encodeURIComponent(search);
    api('/accounts' + q).then(renderAccounts).catch(function (err) {
      fail($('accounts-error'), err.message);
    });
  }

  $('players-form').addEventListener('submit', function (ev) {
    ev.preventDefault();
    loadAccounts();
  });
  $('players-sort').addEventListener('change', loadAccounts);
  $('players-limit').addEventListener('change', loadAccounts);

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
    loadBots();
    loadAccounts();
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
