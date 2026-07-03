const API = window.location.origin + "/api/v1/partner";
const TOKEN_KEY = "tyrax_partner_token";

const $ = (sel) => document.querySelector(sel);
const loginScreen = $("#login-screen");
const registerScreen = $("#register-screen");
const mainScreen = $("#main-screen");

function token() { return localStorage.getItem(TOKEN_KEY); }
function setToken(t) { localStorage.setItem(TOKEN_KEY, t); }
function clearToken() { localStorage.removeItem(TOKEN_KEY); }

function inviteTokenFromURL() {
  return new URLSearchParams(window.location.search).get("invite") || "";
}

function isRegisterRoute() {
  return window.location.pathname.includes("register") || inviteTokenFromURL();
}

async function api(path, opts = {}) {
  const headers = { "Content-Type": "application/json", ...(opts.headers || {}) };
  const tok = opts.skipAuth ? null : token();
  if (tok) headers.Authorization = "Bearer " + tok;
  const res = await fetch(API + path, { ...opts, headers });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) {
    if (!opts.skipAuth) {
      clearToken();
      showLogin();
    }
    throw new Error(data.message || "ACCESS DENIED");
  }
  if (!res.ok) throw new Error(data.message || "REQUEST FAILED");
  return data;
}

function showLogin() {
  loginScreen.classList.remove("hidden");
  registerScreen.classList.add("hidden");
  mainScreen.classList.add("hidden");
}

function showRegister() {
  registerScreen.classList.remove("hidden");
  loginScreen.classList.add("hidden");
  mainScreen.classList.add("hidden");
}

function showMain() {
  loginScreen.classList.add("hidden");
  registerScreen.classList.add("hidden");
  mainScreen.classList.remove("hidden");
  loadDashboard();
  loadPayouts();
}

function formatRub(n) {
  return Number(n || 0).toLocaleString("ru-RU", { maximumFractionDigits: 0 }) + " ₽";
}

function formatDate(v) {
  if (!v) return "—";
  return new Date(v).toLocaleString("ru-RU");
}

document.querySelectorAll(".tab").forEach((tab) => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((t) => t.classList.remove("active"));
    tab.classList.add("active");
    document.querySelectorAll(".tab-panel").forEach((p) => p.classList.add("hidden"));
    $("#tab-" + tab.dataset.tab).classList.remove("hidden");
  });
});

$("#logout-btn").addEventListener("click", () => {
  clearToken();
  showLogin();
});

$("#login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = $("#login-error");
  errEl.classList.add("hidden");
  try {
    const res = await api("/auth/login", {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({
        email: $("#login-email").value.trim(),
        password: $("#login-pass").value,
      }),
    });
    setToken(res.data.token);
    showMain();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove("hidden");
  }
});

async function initRegister() {
  const invite = inviteTokenFromURL();
  const status = $("#invite-status");
  const form = $("#register-form");
  if (!invite) {
    status.textContent = "НУЖНА ИНВАЙТ-ССЫЛКА ОТ АДМИНИСТРАТОРА";
    return;
  }
  try {
    await api("/invites/" + encodeURIComponent(invite), { skipAuth: true });
    status.textContent = "ИНВАЙТ ПОДТВЕРЖДЁН. СОЗДАЙ ДОСТУП.";
    form.classList.remove("hidden");
  } catch {
    status.textContent = "ИНВАЙТ НЕДЕЙСТВИТЕЛЕН ИЛИ УЖЕ ИСПОЛЬЗОВАН";
  }
}

$("#register-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errEl = $("#register-error");
  errEl.classList.add("hidden");
  try {
    const res = await api("/auth/register", {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({
        invite_token: inviteTokenFromURL(),
        display_name: $("#reg-name").value.trim(),
        email: $("#reg-email").value.trim(),
        password: $("#reg-pass").value,
      }),
    });
    setToken(res.data.token);
    history.replaceState({}, "", "/partner/");
    showMain();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove("hidden");
  }
});

async function loadDashboard() {
  const res = await api("/dashboard");
  const p = res.data.partner;
  const s = res.data.stats;
  $("#stat-regs").textContent = s.registrations;
  $("#stat-active").textContent = s.active_users;
  $("#stat-conv").textContent = s.conversions;
  $("#ref-link").value = p.ref_link || "";
  $("#bal-available").textContent = formatRub(p.balance_available);
  $("#bal-hold").textContent = formatRub(p.balance_hold);
  $("#bal-paid").textContent = formatRub(p.total_paid_out);
  if (p.payout_method === "mir" && p.payout_mir_card) {
    $("#payout-method").value = "mir";
    $("#payout-mir").value = p.payout_mir_card;
  } else if (p.payout_method === "usdt") {
    $("#payout-method").value = "usdt";
    $("#payout-usdt-addr").value = p.payout_usdt_address || "";
    $("#payout-usdt-net").value = p.payout_usdt_network || "TRC20";
  }
  togglePayoutFields();
}

async function loadPayouts() {
  const res = await api("/payouts");
  const body = $("#payouts-body");
  body.innerHTML = "";
  for (const row of res.data.payouts || []) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${formatDate(row.created_at)}</td><td>${formatRub(row.amount_rub)}</td><td>${escapeHtml(row.note || "—")}</td>`;
    body.appendChild(tr);
  }
  if (!body.children.length) {
    body.innerHTML = "<tr><td colspan='3' class='muted'>ВЫПЛАТ ПОКА НЕТ</td></tr>";
  }
}

$("#copy-ref").addEventListener("click", async () => {
  const val = $("#ref-link").value;
  try {
    await navigator.clipboard.writeText(val);
    $("#copy-ref").textContent = "СКОПИРОВАНО";
    setTimeout(() => { $("#copy-ref").textContent = "КОПИРОВАТЬ"; }, 1500);
  } catch {
    $("#ref-link").select();
    document.execCommand("copy");
  }
});

function togglePayoutFields() {
  const method = $("#payout-method").value;
  $("#mir-fields").classList.toggle("hidden", method !== "mir");
  $("#usdt-fields").classList.toggle("hidden", method !== "usdt");
}

$("#payout-method").addEventListener("change", togglePayoutFields);

$("#payout-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = $("#payout-msg");
  const method = $("#payout-method").value;
  const body = { method };
  if (method === "mir") body.mir_card = $("#payout-mir").value.trim();
  else {
    body.usdt_address = $("#payout-usdt-addr").value.trim();
    body.usdt_network = $("#payout-usdt-net").value;
  }
  try {
    await api("/payout-details", { method: "PUT", body: JSON.stringify(body) });
    msg.textContent = "РЕКВИЗИТЫ СОХРАНЕНЫ";
  } catch (err) {
    msg.textContent = err.message;
  }
});

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

if (token()) {
  showMain();
} else if (isRegisterRoute()) {
  showRegister();
  initRegister();
} else {
  showLogin();
}
