const state = {
  token: localStorage.getItem("token") || "",
  user: null,
};

const el = {
  statusBadge: document.getElementById("statusBadge"),
  logOutput: document.getElementById("logOutput"),
  authPanel: document.getElementById("authPanel"),
  bookingPanel: document.getElementById("bookingPanel"),
  adminPanel: document.getElementById("adminPanel"),

  regName: document.getElementById("regName"),
  regEmail: document.getElementById("regEmail"),
  regPassword: document.getElementById("regPassword"),

  registerBtn: document.getElementById("registerBtn"),
  loginBtn: document.getElementById("loginBtn"),
  logoutBtn: document.getElementById("logoutBtn"),

  startAt: document.getElementById("startAt"),
  endAt: document.getElementById("endAt"),
  loadSeatsBtn: document.getElementById("loadSeatsBtn"),
  loadMyBookingsBtn: document.getElementById("loadMyBookingsBtn"),
  seatsTable: document.getElementById("seatsTable"),
  myBookingsTable: document.getElementById("myBookingsTable"),

  seatId: document.getElementById("seatId"),
  seatName: document.getElementById("seatName"),
  seatZone: document.getElementById("seatZone"),
  seatType: document.getElementById("seatType"),
  createSeatBtn: document.getElementById("createSeatBtn"),
  updateSeatBtn: document.getElementById("updateSeatBtn"),
  deleteSeatBtn: document.getElementById("deleteSeatBtn"),

  bookingLimit: document.getElementById("bookingLimit"),
  updateLimitBtn: document.getElementById("updateLimitBtn"),
  loadAllBookingsBtn: document.getElementById("loadAllBookingsBtn"),
  allBookingsTable: document.getElementById("allBookingsTable"),

  reportFrom: document.getElementById("reportFrom"),
  reportTo: document.getElementById("reportTo"),
  runReportBtn: document.getElementById("runReportBtn"),
  reportResult: document.getElementById("reportResult"),
};

function log(message) {
  const timestamp = new Date().toISOString();
  el.logOutput.textContent = `[${timestamp}] ${message}\n${el.logOutput.textContent}`.slice(0, 6000);
}

function toISO(value) {
  if (!value) return "";
  return new Date(value).toISOString();
}

function setDefaultWindow() {
  const now = new Date();
  const start = new Date(now.getTime() + 60 * 60 * 1000);
  const end = new Date(now.getTime() + 2 * 60 * 60 * 1000);
  el.startAt.value = toInputValue(start);
  el.endAt.value = toInputValue(end);
  el.reportFrom.value = toInputValue(now);
  el.reportTo.value = toInputValue(new Date(now.getTime() + 24 * 60 * 60 * 1000));
}

function toInputValue(date) {
  const pad = (n) => String(n).padStart(2, "0");
  const yyyy = date.getFullYear();
  const mm = pad(date.getMonth() + 1);
  const dd = pad(date.getDate());
  const hh = pad(date.getHours());
  const mi = pad(date.getMinutes());
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}`;
}

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (!headers["Content-Type"] && options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }
  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(data?.error || `HTTP ${response.status}`);
  }
  return data;
}

function updateAuthUI() {
  const signedIn = Boolean(state.token && state.user);
  el.logoutBtn.hidden = !signedIn;
  el.bookingPanel.hidden = !signedIn;
  el.adminPanel.hidden = !(signedIn && state.user.role === "admin");
  el.statusBadge.textContent = signedIn
    ? `${state.user.email} (${state.user.role})`
    : "Signed out";
}

async function loadMe() {
  if (!state.token) {
    state.user = null;
    updateAuthUI();
    return;
  }
  try {
    state.user = await api("/api/auth/me");
    updateAuthUI();
  } catch (err) {
    log(`Auth check failed: ${err.message}`);
    state.token = "";
    state.user = null;
    localStorage.removeItem("token");
    updateAuthUI();
  }
}

async function register() {
  const name = el.regName.value.trim();
  const email = el.regEmail.value.trim();
  const password = el.regPassword.value;
  await api("/api/auth/register", {
    method: "POST",
    body: { name, email, password },
  });
  log(`Registered user ${email}`);
}

async function login() {
  const email = el.regEmail.value.trim();
  const password = el.regPassword.value;
  const result = await api("/api/auth/login", {
    method: "POST",
    body: { email, password },
  });
  state.token = result.token;
  localStorage.setItem("token", state.token);
  await loadMe();
  await loadMyBookings();
  log(`Logged in as ${email}`);
}

function logout() {
  state.token = "";
  state.user = null;
  localStorage.removeItem("token");
  updateAuthUI();
  log("Logged out");
}

async function loadAvailableSeats() {
  const startAt = toISO(el.startAt.value);
  const endAt = toISO(el.endAt.value);
  const seats = await api(`/api/seats/available?start_at=${encodeURIComponent(startAt)}&end_at=${encodeURIComponent(endAt)}`);

  el.seatsTable.innerHTML = "";
  seats.forEach((seat) => {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${seat.id}</td>
      <td>${seat.name}</td>
      <td>${seat.zone}</td>
      <td>${seat.type}</td>
      <td><button data-seat-id="${seat.id}">Book</button></td>
    `;
    row.querySelector("button").addEventListener("click", () => createBooking(seat.id));
    el.seatsTable.appendChild(row);
  });
}

async function createBooking(seatId) {
  const startAt = toISO(el.startAt.value);
  const endAt = toISO(el.endAt.value);
  await api("/api/bookings", {
    method: "POST",
    body: { seat_id: seatId, start_at: startAt, end_at: endAt },
  });
  log(`Created booking for seat ${seatId}`);
  await loadMyBookings();
  await loadAvailableSeats();
}

function formatDT(value) {
  return new Date(value).toLocaleString();
}

async function loadMyBookings() {
  const bookings = await api("/api/bookings/me");
  el.myBookingsTable.innerHTML = "";
  bookings.forEach((booking) => {
    const canCancel = booking.status === "confirmed";
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${booking.id}</td>
      <td>${booking.seat_id}</td>
      <td>${formatDT(booking.start_at)}</td>
      <td>${formatDT(booking.end_at)}</td>
      <td>${booking.status}</td>
      <td>${canCancel ? `<button data-id="${booking.id}" class="danger">Cancel</button>` : ""}</td>
    `;
    const btn = row.querySelector("button");
    if (btn) {
      btn.addEventListener("click", async () => {
        await api(`/api/bookings/${booking.id}`, { method: "DELETE" });
        log(`Canceled booking ${booking.id}`);
        await loadMyBookings();
        await loadAvailableSeats();
      });
    }
    el.myBookingsTable.appendChild(row);
  });
}

async function createSeat() {
  await api("/api/admin/seats", {
    method: "POST",
    body: {
      name: el.seatName.value.trim(),
      zone: el.seatZone.value.trim(),
      type: el.seatType.value.trim(),
      active: true,
    },
  });
  log("Seat created");
}

async function updateSeat() {
  const seatId = Number(el.seatId.value);
  await api(`/api/admin/seats/${seatId}`, {
    method: "PUT",
    body: {
      name: el.seatName.value.trim(),
      zone: el.seatZone.value.trim(),
      type: el.seatType.value.trim(),
      active: true,
    },
  });
  log(`Seat ${seatId} updated`);
}

async function deleteSeat() {
  const seatId = Number(el.seatId.value);
  await api(`/api/admin/seats/${seatId}`, { method: "DELETE" });
  log(`Seat ${seatId} deleted`);
}

async function updateLimit() {
  const limit = Number(el.bookingLimit.value);
  await api("/api/admin/settings/limit", {
    method: "PUT",
    body: { limit },
  });
  log(`Booking limit updated to ${limit}`);
}

async function loadAllBookings() {
  const bookings = await api("/api/admin/bookings");
  el.allBookingsTable.innerHTML = "";
  bookings.forEach((booking) => {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${booking.id}</td>
      <td>${booking.user_id}</td>
      <td>${booking.seat_id}</td>
      <td>${formatDT(booking.start_at)}</td>
      <td>${formatDT(booking.end_at)}</td>
      <td>
        <select data-id="${booking.id}">
          <option value="confirmed" ${booking.status === "confirmed" ? "selected" : ""}>confirmed</option>
          <option value="canceled" ${booking.status === "canceled" ? "selected" : ""}>canceled</option>
          <option value="completed" ${booking.status === "completed" ? "selected" : ""}>completed</option>
        </select>
      </td>
      <td><button data-id="${booking.id}" class="secondary">Apply</button></td>
    `;
    const button = row.querySelector("button");
    const select = row.querySelector("select");
    button.addEventListener("click", async () => {
      await api(`/api/admin/bookings/${booking.id}`, {
        method: "PATCH",
        body: { status: select.value },
      });
      log(`Booking ${booking.id} updated to ${select.value}`);
      await loadAllBookings();
      await loadMyBookings();
    });
    el.allBookingsTable.appendChild(row);
  });
}

async function runReport() {
  const from = toISO(el.reportFrom.value);
  const to = toISO(el.reportTo.value);
  const report = await api(`/api/admin/reports?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`);
  el.reportResult.textContent = JSON.stringify(report, null, 2);
}

function initTelegram() {
  const tg = window.Telegram?.WebApp;
  if (!tg) return;
  tg.ready();
  tg.expand();
  const user = tg.initDataUnsafe?.user;
  if (user) {
    if (!el.regName.value) {
      el.regName.value = [user.first_name, user.last_name].filter(Boolean).join(" ").trim() || `tg-${user.id}`;
    }
    if (!el.regEmail.value) {
      el.regEmail.value = `${user.id}@tg.local`;
    }
  }
}

function bindActions() {
  el.registerBtn.addEventListener("click", () => runAction(register));
  el.loginBtn.addEventListener("click", () => runAction(login));
  el.logoutBtn.addEventListener("click", logout);
  el.loadSeatsBtn.addEventListener("click", () => runAction(loadAvailableSeats));
  el.loadMyBookingsBtn.addEventListener("click", () => runAction(loadMyBookings));

  el.createSeatBtn.addEventListener("click", () => runAction(createSeat));
  el.updateSeatBtn.addEventListener("click", () => runAction(updateSeat));
  el.deleteSeatBtn.addEventListener("click", () => runAction(deleteSeat));
  el.updateLimitBtn.addEventListener("click", () => runAction(updateLimit));
  el.loadAllBookingsBtn.addEventListener("click", () => runAction(loadAllBookings));
  el.runReportBtn.addEventListener("click", () => runAction(runReport));
}

async function runAction(fn) {
  try {
    await fn();
  } catch (err) {
    log(`Error: ${err.message}`);
  }
}

async function bootstrap() {
  setDefaultWindow();
  initTelegram();
  bindActions();
  await loadMe();
  if (state.user) {
    await runAction(loadMyBookings);
    await runAction(loadAvailableSeats);
    if (state.user.role === "admin") {
      await runAction(loadAllBookings);
    }
  }
}

bootstrap();
