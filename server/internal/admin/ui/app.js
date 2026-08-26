"use strict";

// The console never builds markup from strings. Every value that reaches the
// page goes through textContent, because usernames and email addresses are
// attacker-controlled: anyone can register an account named like a <script>
// tag, and this page is viewed by someone holding an admin token.

const ACCESS_KEY = "zolik_admin_access";
const REFRESH_KEY = "zolik_admin_refresh";
const PAGE_SIZE = 25;

const el = (id) => document.getElementById(id);
const state = { search: "", skip: 0, total: 0, days: 30 };

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
    if (refresh) localStorage.setItem(REFRESH_KEY, refresh);
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
    // 403 is the guard's answer for "signed in, but not an administrator". It
    // is not a session problem, so it must not silently bounce to sign-in.
    throw await readError(res, "That account is not an administrator.");
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

function signOut() {
  tokens.clear();
  el("console").hidden = true;
  el("signin").hidden = false;
  el("code-form").hidden = true;
  el("email-form").hidden = false;
}

function signinError(message) {
  const box = el("signin-error");
  box.textContent = message;
  box.hidden = false;
}

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
    await start();
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

/* --------------------------------------------------------------------- boot */

async function refresh() {
  clearError();
  await Promise.all([loadUsage(), loadUsers()]);
}

async function start() {
  if (!tokens.access) {
    signOut();
    return;
  }
  try {
    const me = await api("/admin/api/session");
    el("whoami").textContent = me.email || me.username;
    el("signin").hidden = true;
    el("console").hidden = false;
    await refresh();
  } catch (err) {
    // Reaching /session at all is what proves the token belongs to an
    // administrator, so any failure here belongs on the sign-in screen.
    signOut();
    signinError(err.message);
  }
}

start();
