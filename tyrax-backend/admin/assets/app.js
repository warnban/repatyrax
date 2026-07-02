const API = window.location.origin + "/api/v1/admin";
const TOKEN_KEY = "tyrax_admin_token";

const $ = (sel) => document.querySelector(sel);
const loginScreen = $("#login-screen");
const mainScreen = $("#main-screen");
const loginForm = $("#login-form");
const loginError = $("#login-error");
const usersBody = $("#users-body");
const userModal = $("#user-modal");
const modalBody = $("#modal-body");
const modalTitle = $("#modal-title");
const ticketList = $("#ticket-list");
const ticketDetail = $("#ticket-detail");

let activeTicketId = null;

function token() { return localStorage.getItem(TOKEN_KEY); }
function setToken(t) { localStorage.setItem(TOKEN_KEY, t); }
function clearToken() { localStorage.removeItem(TOKEN_KEY); }

async function api(path, opts = {}) {
  const headers = { "Content-Type": "application/json", ...(opts.headers || {}) };
  if (token()) headers.Authorization = "Bearer " + token();
  const res = await fetch(API + path, { ...opts, headers });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) {
    clearToken();
    showLogin();
    throw new Error("SESSION EXPIRED");
  }
  if (!res.ok) throw new Error(data.message || "REQUEST FAILED");
  return data;
}

function showLogin() {
  loginScreen.classList.remove("hidden");
  mainScreen.classList.add("hidden");
}

function showMain() {
  loginScreen.classList.add("hidden");
  mainScreen.classList.remove("hidden");
  loadUsers();
  loadTickets();
}

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  loginError.classList.add("hidden");
  try {
    const res = await api("/auth/login", {
      method: "POST",
      body: JSON.stringify({
        username: $("#login-user").value.trim(),
        password: $("#login-pass").value,
      }),
    });
    setToken(res.data.token);
    showMain();
  } catch (err) {
    loginError.textContent = err.message;
    loginError.classList.remove("hidden");
  }
});

$("#logout-btn").addEventListener("click", () => {
  clearToken();
  showLogin();
});

document.querySelectorAll(".tab").forEach((tab) => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((t) => t.classList.remove("active"));
    tab.classList.add("active");
    document.querySelectorAll(".tab-panel").forEach((p) => p.classList.add("hidden"));
    $("#tab-" + tab.dataset.tab).classList.remove("hidden");
  });
});

$("#user-search-btn").addEventListener("click", loadUsers);
$("#user-search").addEventListener("keydown", (e) => { if (e.key === "Enter") loadUsers(); });

async function loadUsers() {
  const q = $("#user-search").value.trim();
  const res = await api("/users?q=" + encodeURIComponent(q));
  usersBody.innerHTML = "";
  for (const u of res.data.users || []) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${identityLabel(u)}</td>
      <td><span class="badge ${u.effective_tier === "DOMINION" ? "dominion" : ""}">${u.effective_tier || u.subscription_tier}</span></td>
      <td>${u.device_count || 0}</td>
      <td><span class="badge ${u.is_online ? "online" : "offline"}">${u.is_online ? "ONLINE" : "OFFLINE"}</span></td>
      <td>${u.registration_ip || "—"}</td>
      <td>${formatDate(u.subscription_end)}</td>`;
    tr.addEventListener("click", () => openUser(u.id));
    usersBody.appendChild(tr);
  }
}

function identityLabel(u) {
  if (u.email) return u.email;
  if (u.username) return "@" + u.username;
  if (u.telegram_id) return "TG " + u.telegram_id;
  return u.id.slice(0, 8);
}

function formatDate(v) {
  if (!v) return "—";
  return new Date(v).toLocaleString("ru-RU");
}

async function openUser(id) {
  const res = await api("/users/" + id);
  const u = res.data;
  modalTitle.textContent = identityLabel(u);
  modalBody.innerHTML = `
    <div class="grid-2">
      <div><span class="muted">ID</span><br>${u.id}</div>
      <div><span class="muted">TIER (DB)</span><br>${u.subscription_tier}</div>
      <div><span class="muted">EFFECTIVE</span><br>${u.effective_tier}</div>
      <div><span class="muted">DEVICES</span><br>${u.device_count}</div>
      <div><span class="muted">STATUS</span><br>${u.is_online ? "ONLINE" : "OFFLINE"}</div>
      <div><span class="muted">REG IP</span><br>${u.registration_ip || "—"}</div>
      <div><span class="muted">SUB END</span><br>${formatDate(u.subscription_end)}</div>
      <div><span class="muted">CREATED</span><br>${formatDate(u.created_at)}</div>
    </div>
    <div class="section">
      <h3>GRANT TARIFF</h3>
      <div class="grant-row">
        <select id="grant-tier">
          <option value="CORE">CORE</option>
          <option value="SHADOW">SHADOW</option>
          <option value="DOMINION">DOMINION</option>
        </select>
        <select id="grant-period">
          <option value="7d">7 DAYS</option>
          <option value="14d">14 DAYS</option>
          <option value="1m">1 MONTH</option>
          <option value="3m">3 MONTHS</option>
          <option value="6m">6 MONTHS</option>
          <option value="12m">12 MONTHS</option>
        </select>
        <button class="btn btn-primary" id="grant-btn">GRANT</button>
        <button class="btn" id="revoke-btn">REVOKE → FREE</button>
      </div>
      <p id="grant-msg" class="muted"></p>
    </div>
    <div class="section">
      <h3>CONNECTION HISTORY</h3>
      ${renderConnections(u.connections || [])}
    </div>`;

  $("#grant-btn").onclick = async () => {
    try {
      await api(`/users/${u.id}/subscription`, {
        method: "POST",
        body: JSON.stringify({
          tier: $("#grant-tier").value,
          period: $("#grant-period").value,
        }),
      });
      $("#grant-msg").textContent = "ACCESS GRANTED";
      openUser(u.id);
      loadUsers();
    } catch (err) {
      $("#grant-msg").textContent = err.message;
    }
  };
  $("#revoke-btn").onclick = async () => {
    if (!confirm("Revoke paid tier and set FREE?")) return;
    await api(`/users/${u.id}/subscription`, { method: "DELETE" });
    openUser(u.id);
    loadUsers();
  };

  userModal.showModal();
}

function renderConnections(rows) {
  if (!rows.length) return "<p class='muted'>NO DATA</p>";
  return `<table><thead><tr><th>PROTOCOL</th><th>CONNECTED</th><th>DISCONNECTED</th></tr></thead><tbody>` +
    rows.map((r) => `<tr><td>${r.protocol}</td><td>${formatDate(r.connected_at)}</td><td>${formatDate(r.disconnected_at)}</td></tr>`).join("") +
    "</tbody></table>";
}

$("#modal-close").addEventListener("click", () => userModal.close());

$("#ticket-filter").addEventListener("change", loadTickets);
$("#ticket-refresh").addEventListener("click", loadTickets);

async function loadTickets() {
  const status = $("#ticket-filter").value;
  const res = await api("/support/tickets?status=" + encodeURIComponent(status));
  ticketList.innerHTML = "";
  for (const t of res.data.tickets || []) {
    const el = document.createElement("div");
    el.className = "ticket-item" +
      (t.id === activeTicketId ? " active" : "") +
      (t.status === "open" && t.subscription_tier === "DOMINION" ? " dominion" : "");
    el.innerHTML = `
      <div><strong>${escapeHtml(t.subject || "Без темы")}</strong></div>
      <div class="ticket-meta">${t.subscription_tier} · ${t.status.toUpperCase()} · ${formatDate(t.updated_at)}</div>
      <div class="ticket-meta">${t.telegram_username ? "@" + t.telegram_username : "TG " + t.telegram_id}</div>`;
    el.addEventListener("click", () => openTicket(t.id));
    ticketList.appendChild(el);
  }
}

async function openTicket(id) {
  activeTicketId = id;
  loadTickets();
  const res = await api("/support/tickets/" + id);
  const { ticket, messages } = res.data;
  ticketDetail.classList.remove("empty");
  ticketDetail.innerHTML = `
    <div>
      <strong>${escapeHtml(ticket.subject || "Тикет")}</strong>
      <div class="ticket-meta">${ticket.subscription_tier} · ${ticket.status.toUpperCase()}</div>
    </div>
    <div class="messages" id="messages">${messages.map(renderMsg).join("")}</div>
    ${ticket.status === "open" ? `
      <textarea id="reply-text" placeholder="ОТВЕТ ПОЛЬЗОВАТЕЛЮ"></textarea>
      <div class="grant-row">
        <button class="btn btn-primary" id="reply-btn">SEND</button>
        <button class="btn" id="close-btn">CLOSE TICKET</button>
      </div>` : "<p class='muted'>TICKET CLOSED</p>"}`;

  if (ticket.status === "open") {
    $("#reply-btn").onclick = async () => {
      const body = $("#reply-text").value.trim();
      if (!body) return;
      await api(`/support/tickets/${id}/reply`, { method: "POST", body: JSON.stringify({ body }) });
      openTicket(id);
    };
    $("#close-btn").onclick = async () => {
      await api(`/support/tickets/${id}/close`, { method: "POST" });
      activeTicketId = null;
      loadTickets();
      ticketDetail.classList.add("empty");
      ticketDetail.innerHTML = "<p class='muted'>ВЫБЕРИ ТИКЕТ</p>";
    };
  }
  const box = $("#messages");
  if (box) box.scrollTop = box.scrollHeight;
}

function renderMsg(m) {
  return `<div class="msg ${m.sender}">${escapeHtml(m.body)}<div class="ticket-meta">${formatDate(m.created_at)}</div></div>`;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

if (token()) showMain();
else showLogin();

setInterval(() => {
  if (!token() || mainScreen.classList.contains("hidden")) return;
  if (!$("#tab-support").classList.contains("hidden")) loadTickets();
}, 15000);
