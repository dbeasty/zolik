"use strict";

// The console never builds markup from strings. Every value that reaches the
// page goes through textContent, because usernames and email addresses are
// attacker-controlled: anyone can register an account named like a <script>
// tag, and this page is viewed by someone holding an admin token.

const ACCESS_KEY = "zolik_admin_access";
const REFRESH_KEY = "zolik_admin_refresh";
const PAGE_SIZE = 25;

const el = (id) => document.getElementById(id);
const state = {
  search: "",
  skip: 0,
  total: 0,
  days: 30,
  // Feedback opens on the untriaged queue, because that is the only reason to
  // come looking at it.
  reportStatus: "new",
  reportKind: "",
  reportSkip: 0,
  reportTotal: 0,
};

const STATUS_FILTERS = [
  { value: "new", label: "New" },
  { value: "open", label: "Open" },
  { value: "resolved", label: "Resolved" },
  { value: "", label: "All" },
];

/* ------------------------------------------------------------------ tokens */

const tokens = {
  get access() {
    return localStorage.getItem(ACCESS_KEY) || "";
  },
  get refresh() {
    return localStorage.getItem(REFRESH_KEY) || "";
  },
  set(access, refresh) {
    localStorage.setItem(ACCESS_KEY, access);
    // A console sign-in has no refresh token, and any left over from a
    // previous email sign-in must go with it — otherwise a 401 on the console
    // token would refresh into a *player* token and quietly swap which
    // administrator this tab is.
    if (refresh) {
      localStorage.setItem(REFRESH_KEY, refresh);
    } else {
      localStorage.removeItem(REFRESH_KEY);
    }
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
  },
};

/* --------------------------------------------------------------------- api */

async function readError(res, fallback) {
  const text = (await res.text()).trim();
  return new Error(text || fallback || `Request failed (${res.status})`);
}

async function post(path, body) {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await readError(res);
  return res.json();
}

// Access tokens last 15 minutes, so an admin who leaves the tab open will hit
// an expiry mid-session. One retry behind a refresh keeps that invisible;
// a second failure means the refresh token is gone too, and the only honest
// answer is to sign in again.
async function api(path, options = {}, retry = true) {
  const res = await fetch(path, {
    ...options,
    headers: { ...(options.headers || {}), Authorization: `Bearer ${tokens.access}` },
  });
  if (res.status === 401 && retry && tokens.refresh) {
    try {
      const next = await post("/auth/refresh", { refreshToken: tokens.refresh });
      tokens.set(next.accessToken, next.refreshToken);
      return api(path, options, false);
    } catch {
      signOut();
      throw new Error("Session expired — sign in again.");
    }
  }
  if (res.status === 401 || res.status === 403) {
    // The guard answers both of these with a bare "forbidden", which tells an
    // operator nothing. Say what it actually means rather than passing the
    // wire text through.
    throw new Error("That account is not an administrator here.");
  }
  if (!res.ok) throw await readError(res);
  return res.json();
}

const count = (n, one, many) => `${n} ${n === 1 ? one : many}`;

function showMessage(message, kind) {
  const box = el("error");
  box.textContent = message;
  box.className = kind;
  box.hidden = false;
}

const showError = (message) => showMessage(message, "error");
// Confirmations share the banner but not its styling — reporting a successful
// revocation in the red error box reads as a failure.
const showNotice = (message) => showMessage(message, "notice");

function clearError() {
  el("error").hidden = true;
}

/* ------------------------------------------------------------------ sign in */

// showSignIn reveals only the doors this deployment actually has, so an
// operator is never shown a form that cannot possibly work.
async function showSignIn() {
  tokens.clear();
  el("console").hidden = true;
  el("signin").hidden = false;
  el("code-form").hidden = true;

  let methods = { password: false, email: true };
  try {
    methods = await (await fetch("/admin/api/methods")).json();
  } catch {
    // If even this cannot be reached the server is down, and the email form
    // is the better thing to be looking at than a blank card.
  }
  el("password-form").hidden = !methods.password;
  el("email-form").hidden = !methods.email;
  el("email-intro").hidden = !methods.email;
  el("signin-or").hidden = !(methods.password && methods.email);

  if (!methods.password && !methods.email) {
    signinError("No administrator is configured on this server.");
  }
}

function signOut() {
  showSignIn().catch(() => {});
}

function signinError(message) {
  const box = el("signin-error");
  box.textContent = message;
  box.hidden = false;
}

el("password-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  el("signin-error").hidden = true;
  try {
    const res = await post("/admin/api/login", {
      username: el("admin-username").value,
      password: el("admin-password").value,
    });
    // No refresh token: a console sign-in is a single long-lived token, so
    // there is nothing to rotate. See consoleTokenTTL on the server.
    tokens.set(res.token, "");
    el("admin-password").value = "";
    await start(true);
  } catch (err) {
    signinError(err.message);
  }
});

el("email-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  el("signin-error").hidden = true;
  const email = el("email").value.trim();
  try {
    await post("/auth/email/start", { email });
    el("code-target").textContent = email;
    el("email-form").hidden = true;
    el("code-form").hidden = false;
    el("code").focus();
  } catch (err) {
    signinError(err.message);
  }
});

el("code-back").addEventListener("click", () => {
  el("code-form").hidden = true;
  el("email-form").hidden = false;
  el("signin-error").hidden = true;
});

el("code-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  el("signin-error").hidden = true;
  try {
    const res = await post("/auth/email/verify", {
      email: el("email").value.trim(),
      code: el("code").value.trim(),
    });
    tokens.set(res.accessToken, res.refreshToken);
    await start(true);
  } catch (err) {
    signinError(err.message);
  }
});

el("signout").addEventListener("click", signOut);

/* ------------------------------------------------------------------ overview */

function tile(value, label) {
  const box = document.createElement("div");
  box.className = "tile";
  const v = document.createElement("div");
  v.className = "tile-value";
  v.textContent = String(value);
  const l = document.createElement("div");
  l.className = "tile-label";
  l.textContent = label;
  box.append(v, l);
  return box;
}

function renderChart(byDay) {
  const chart = el("chart");
  chart.replaceChildren();
  const peak = Math.max(1, ...byDay.map((d) => d.matches));
  for (const day of byDay) {
    const col = document.createElement("div");
    col.className = "chart-col";
    col.title = `${day.date}: ${day.matches} matches`;
    const bar = document.createElement("div");
    bar.className = "chart-bar";
    // Scaled against the peak rather than a fixed ceiling, so a quiet period
    // still shows shape instead of a flat row of stubs.
    bar.style.height = `${(day.matches / peak) * 100}%`;
    col.append(bar);
    chart.append(col);
  }
}

async function loadUsage() {
  const usage = await api(`/admin/api/usage?days=${state.days}`);
  el("tiles").replaceChildren(
    tile(usage.users.total, "Accounts"),
    tile(usage.users.activeDay, "Active today"),
    tile(usage.users.activeWeek, "Active this week"),
    tile(usage.users.openSessions, "Open sessions"),
    tile(usage.guests.openSessions, "Guest sessions"),
    tile(usage.live.instanceConnections, "Connected now (this instance)"),
    tile(usage.live.instanceMatches, "Live matches (this instance)"),
    tile(usage.live.instanceWaiting, "In the waiting room"),
    tile(usage.matches.totalMatches, "Matches all time"),
    tile(usage.matches.recentMatches, `Matches in ${state.days} days`)
  );
  renderChart(usage.matches.byDay);

  const modules = el("modules");
  modules.replaceChildren();
  for (const m of usage.matches.byModule) {
    const chip = document.createElement("span");
    chip.className = "chip";
    const mins = Math.round(m.avgDurationSeconds / 60);
    chip.textContent = `${m.moduleId}: ${count(m.matches, "match", "matches")} · ~${mins} min avg`;
    modules.append(chip);
  }
}

/* --------------------------------------------------------------------- users */

const fmtDate = (iso) => {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) || d.getFullYear() < 1971 ? "—" : d.toLocaleDateString();
};

function userCell(user) {
  const td = document.createElement("td");
  const name = document.createElement("div");
  name.className = "name";
  name.textContent = user.username;
  if (user.isAdmin) {
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = "admin";
    name.append(badge);
  }
  const sub = document.createElement("div");
  sub.className = "sub";
  sub.textContent = user.email ? `${user.email}${user.emailVerified ? "" : " (unverified)"}` : "no email";
  td.append(name, sub);
  return td;
}

function actionButton(label, className, onClick) {
  const b = document.createElement("button");
  b.textContent = label;
  if (className) b.className = className;
  b.addEventListener("click", onClick);
  return b;
}

function renderUsers(rows) {
  const body = el("users").querySelector("tbody");
  body.replaceChildren();

  if (rows.length === 0) {
    const tr = document.createElement("tr");
    const td = document.createElement("td");
    td.colSpan = 5;
    td.className = "muted";
    td.textContent = "No accounts match.";
    tr.append(td);
    body.append(tr);
    return;
  }

  for (const user of rows) {
    const tr = document.createElement("tr");

    const signin = document.createElement("td");
    const methods = [...user.identities];
    if (user.hasPassword) methods.push("password");
    signin.textContent = methods.length ? methods.join(", ") : user.authProvider || "—";

    const created = document.createElement("td");
    created.textContent = fmtDate(user.createdAt);

    const seen = document.createElement("td");
    seen.textContent = fmtDate(user.lastSeenAt);

    const actions = document.createElement("td");
    actions.className = "actions";
    actions.append(
      actionButton("Reset password", "", () => resetPassword(user)),
      actionButton("Revoke sessions", "", () => revokeSessions(user)),
      actionButton("Delete", "danger", () => deleteUser(user))
    );

    tr.append(userCell(user), signin, created, seen, actions);
    body.append(tr);
  }
}

async function loadUsers() {
  const params = new URLSearchParams({
    limit: String(PAGE_SIZE),
    skip: String(state.skip),
  });
  if (state.search) params.set("search", state.search);
  const res = await api(`/admin/api/users?${params}`);
  state.total = res.total;
  renderUsers(res.users);

  const from = res.total === 0 ? 0 : state.skip + 1;
  const to = Math.min(state.skip + PAGE_SIZE, res.total);
  el("page-info").textContent = `${from}–${to} of ${res.total}`;
  el("prev").disabled = state.skip === 0;
  el("next").disabled = state.skip + PAGE_SIZE >= res.total;
}

/* ----------------------------------------------------------------- feedback */

const fmtWhen = (iso) => {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString();
};

function tag(text) {
  const t = document.createElement("span");
  t.className = "tag";
  t.textContent = text;
  return t;
}

function renderStatusFilters(counts) {
  const box = el("status-filters");
  box.replaceChildren();
  for (const filter of STATUS_FILTERS) {
    const b = document.createElement("button");
    const n = filter.value ? counts?.[filter.value] : undefined;
    b.textContent = n === undefined ? filter.label : `${filter.label} ${n}`;
    // aria-pressed rather than a class, so the selected filter is announced
    // rather than only coloured.
    b.setAttribute("aria-pressed", String(state.reportStatus === filter.value));
    b.addEventListener("click", () => {
      state.reportStatus = filter.value;
      state.reportSkip = 0;
      loadReports().catch((err) => showError(err.message));
    });
    box.append(b);
  }
}

function reportCard(report) {
  const card = document.createElement("div");
  card.className = "report";
  card.dataset.status = report.status;

  const head = document.createElement("div");
  head.className = "report-head";

  const left = document.createElement("div");
  const kind = document.createElement("span");
  kind.className = "kind";
  kind.dataset.kind = report.kind;
  kind.textContent = report.kind;
  const who = document.createElement("span");
  who.className = "report-who";
  const name = report.username || (report.guestId ? "a guest" : "someone signed out");
  who.textContent = ` ${name} · ${fmtWhen(report.createdAt)}`;
  left.append(kind, who);

  const status = document.createElement("span");
  status.className = "report-who";
  status.textContent = report.status;
  head.append(left, status);

  const message = document.createElement("p");
  message.className = "report-message";
  message.textContent = report.message;

  const context = document.createElement("div");
  context.className = "report-context";
  if (report.appVersion) context.append(tag(`v${report.appVersion}`));
  if (report.platform) context.append(tag(report.platform));
  if (report.matchId) context.append(tag(`match ${report.matchId}`));
  if (report.contactEmail) context.append(tag(`reply to ${report.contactEmail}`));

  const actions = document.createElement("div");
  actions.className = "report-actions";

  const note = document.createElement("input");
  note.className = "report-note";
  note.placeholder = "Note to self";
  note.value = report.note || "";
  note.setAttribute("aria-label", "Note");

  const save = async (patch) => {
    await api(`/admin/api/feedback/${report.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    });
    await loadReports();
  };

  actions.append(note);
  if (report.status !== "open") {
    actions.append(actionButton("Open", "", () => save({ status: "open", note: note.value })));
  }
  if (report.status !== "resolved") {
    actions.append(actionButton("Resolve", "", () => save({ status: "resolved", note: note.value })));
  } else {
    actions.append(actionButton("Reopen", "", () => save({ status: "new", note: note.value })));
  }
  actions.append(actionButton("Save note", "", () => save({ note: note.value })));
  actions.append(
    actionButton("Delete", "danger", () =>
      openModal({
        title: "Delete report",
        body: "Delete this report permanently? Use this for spam — resolving is what you want for a report you have dealt with.",
        confirmLabel: "Delete permanently",
        danger: true,
        onOk: async () => {
          await api(`/admin/api/feedback/${report.id}`, { method: "DELETE" });
          closeModal();
          await loadReports();
        },
      })
    )
  );

  card.append(head, message);
  if (context.childElementCount) card.append(context);
  card.append(actions);
  return card;
}

async function loadReports() {
  const params = new URLSearchParams({
    limit: String(PAGE_SIZE),
    skip: String(state.reportSkip),
  });
  if (state.reportStatus) params.set("status", state.reportStatus);
  if (state.reportKind) params.set("kind", state.reportKind);

  const res = await api(`/admin/api/feedback?${params}`);
  state.reportTotal = res.total;
  renderStatusFilters(res.counts);

  const box = el("reports");
  box.replaceChildren();
  if (res.reports.length === 0) {
    const empty = document.createElement("p");
    empty.className = "muted";
    empty.textContent = "Nothing here.";
    box.append(empty);
  } else {
    for (const report of res.reports) box.append(reportCard(report));
  }

  const from = res.total === 0 ? 0 : state.reportSkip + 1;
  const to = Math.min(state.reportSkip + PAGE_SIZE, res.total);
  el("reports-page-info").textContent = `${from}–${to} of ${res.total}`;
  el("reports-prev").disabled = state.reportSkip === 0;
  el("reports-next").disabled = state.reportSkip + PAGE_SIZE >= res.total;
}

/* -------------------------------------------------------------------- modal */

let onConfirm = null;

function openModal({ title, body, confirmLabel, danger, onOk }) {
  el("modal-title").textContent = title;
  el("modal-body").textContent = body;
  el("modal-extra").replaceChildren();
  const confirm = el("modal-confirm");
  confirm.textContent = confirmLabel;
  confirm.className = danger ? "danger" : "";
  confirm.hidden = false;
  el("modal-cancel").textContent = "Cancel";
  onConfirm = onOk;
  el("modal").hidden = false;
}

function closeModal() {
  el("modal").hidden = true;
  onConfirm = null;
}

el("modal-cancel").addEventListener("click", closeModal);
el("modal-confirm").addEventListener("click", async () => {
  const action = onConfirm;
  if (!action) return;
  el("modal-confirm").disabled = true;
  try {
    await action();
  } catch (err) {
    showError(err.message);
    closeModal();
  } finally {
    el("modal-confirm").disabled = false;
  }
});

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !el("modal").hidden) closeModal();
});

/* ------------------------------------------------------------------ actions */

function resetPassword(user) {
  openModal({
    title: "Reset password",
    body: `Generate a new password for ${user.username}? This signs them out everywhere. The password is shown once and cannot be recovered afterwards.`,
    confirmLabel: "Reset password",
    danger: false,
    onOk: async () => {
      const res = await api(`/admin/api/users/${user.id}/password`, { method: "POST" });
      el("modal-title").textContent = "New password";
      el("modal-body").textContent = `Give this to ${user.username} over a channel you trust. It is not stored anywhere and will not be shown again.`;
      const secret = document.createElement("div");
      secret.className = "secret";
      secret.textContent = res.password;
      el("modal-extra").replaceChildren(secret);
      el("modal-confirm").hidden = true;
      el("modal-cancel").textContent = "Done";
      onConfirm = null;
      await loadUsers();
    },
  });
}

function revokeSessions(user) {
  openModal({
    title: "Revoke sessions",
    body: `Sign ${user.username} out of every device? They can sign in again straight away.`,
    confirmLabel: "Revoke",
    danger: false,
    onOk: async () => {
      const res = await api(`/admin/api/users/${user.id}/revoke-sessions`, { method: "POST" });
      closeModal();
      showNotice(`Revoked ${count(res.revokedSessions, "session", "sessions")} for ${user.username}.`);
    },
  });
}

function deleteUser(user) {
  openModal({
    title: "Delete account",
    body: `Permanently delete ${user.username}? Their sign-in methods and sessions go with them. Match history is kept, because other players' statistics are derived from it. This cannot be undone.`,
    confirmLabel: "Delete permanently",
    danger: true,
    onOk: async () => {
      await api(`/admin/api/users/${user.id}`, { method: "DELETE" });
      closeModal();
      // A deletion can empty the last page, which would otherwise strand the
      // pager past the end of the list.
      if (state.skip >= state.total - 1) state.skip = Math.max(0, state.skip - PAGE_SIZE);
      await refresh();
    },
  });
}

/* ------------------------------------------------------------------ controls */

let searchTimer = null;
el("search").addEventListener("input", (e) => {
  clearTimeout(searchTimer);
  const value = e.target.value.trim();
  searchTimer = setTimeout(() => {
    state.search = value;
    state.skip = 0;
    loadUsers().catch((err) => showError(err.message));
  }, 250);
});

el("days").addEventListener("change", (e) => {
  state.days = Number(e.target.value);
  loadUsage().catch((err) => showError(err.message));
});

el("prev").addEventListener("click", () => {
  state.skip = Math.max(0, state.skip - PAGE_SIZE);
  loadUsers().catch((err) => showError(err.message));
});

el("next").addEventListener("click", () => {
  state.skip += PAGE_SIZE;
  loadUsers().catch((err) => showError(err.message));
});

el("kind-filter").addEventListener("change", (e) => {
  state.reportKind = e.target.value;
  state.reportSkip = 0;
  loadReports().catch((err) => showError(err.message));
});

el("reports-prev").addEventListener("click", () => {
  state.reportSkip = Math.max(0, state.reportSkip - PAGE_SIZE);
  loadReports().catch((err) => showError(err.message));
});

el("reports-next").addEventListener("click", () => {
  state.reportSkip += PAGE_SIZE;
  loadReports().catch((err) => showError(err.message));
});

/* --------------------------------------------------------------------- boot */

async function refresh() {
  clearError();
  await Promise.all([loadUsage(), loadReports(), loadUsers()]);
}

// start proves the stored token belongs to an administrator by using it.
//
// justSignedIn separates the two ways of getting here. Arriving with a token
// that has simply expired is ordinary, and greeting an operator with a red
// error for it would be noise — they just need the form. A token that was
// rejected moments after they typed a password is something they have to be
// told about.
async function start(justSignedIn = false) {
  if (!tokens.access) {
    await showSignIn();
    return;
  }
  try {
    const me = await api("/admin/api/session");
    el("whoami").textContent = me.email || me.username;
    el("signin").hidden = true;
    el("console").hidden = false;
    await refresh();
  } catch (err) {
    await showSignIn();
    if (justSignedIn) signinError(err.message);
  }
}

start();
