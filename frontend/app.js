// Coworking Seat Booking — frontend.
//
// The app has three screens, gated by the actor (per the spec):
//   * landing  — "Кто вы?" choice between Student and Admin.
//   * student  — anonymous, device-bound: no login, name typed per booking,
//                only the student use cases (UC-2/3/4) are reachable.
//   * admin    — email+password login (UC-1) followed by the admin use cases
//                (UC-5/6/7/8) only.

const SCREEN = Object.freeze({ LANDING: "landing", STUDENT: "student", ADMIN: "admin" });

const state = {
  screen: SCREEN.LANDING,
  // Bearer token issued for the current screen. Tokens are kept separately per
  // screen so that switching role doesn't carry credentials over.
  studentToken: localStorage.getItem("studentToken") || "",
  adminToken: localStorage.getItem("adminToken") || "",
  user: null,
  deviceId: localStorage.getItem("deviceId") || "",
};

const el = {
  screenTitle: document.getElementById("screenTitle"),
  statusBadge: document.getElementById("statusBadge"),
  backHomeBtn: document.getElementById("backHomeBtn"),
  logOutput: document.getElementById("logOutput"),

  landingPanel: document.getElementById("landingPanel"),
  chooseStudentBtn: document.getElementById("chooseStudentBtn"),
  chooseAdminBtn: document.getElementById("chooseAdminBtn"),

  studentBookingPanel: document.getElementById("studentBookingPanel"),
  studentName: document.getElementById("studentName"),
  studentStartAt: document.getElementById("studentStartAt"),
  studentEndAt: document.getElementById("studentEndAt"),
  studentLoadSeatsBtn: document.getElementById("studentLoadSeatsBtn"),
  studentRefreshBtn: document.getElementById("studentRefreshBtn"),
  studentSeatsTable: document.getElementById("studentSeatsTable"),
  studentBookingsTable: document.getElementById("studentBookingsTable"),

  adminLoginPanel: document.getElementById("adminLoginPanel"),
  adminEmail: document.getElementById("adminEmail"),
  adminPassword: document.getElementById("adminPassword"),
  adminLoginBtn: document.getElementById("adminLoginBtn"),

  adminPanel: document.getElementById("adminPanel"),
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
  const ts = new Date().toISOString();
  el.logOutput.textContent = `[${ts}] ${message}\n${el.logOutput.textContent}`.slice(0, 6000);
}

function fmtError(err) {
  const message = (err && err.message) ? err.message : String(err);
  log(`Ошибка: ${message}`);
  alert(message);
}

function toInputValue(date) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toISO(value) {
  if (!value) return "";
  return new Date(value).toISOString();
}

function fmtDt(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function setDefaultStudentWindow() {
  const now = new Date();
  const start = new Date(now.getTime() + 2 * 60 * 60 * 1000);
  const end = new Date(now.getTime() + 3 * 60 * 60 * 1000);
  if (!el.studentStartAt.value) el.studentStartAt.value = toInputValue(start);
  if (!el.studentEndAt.value) el.studentEndAt.value = toInputValue(end);
}

function setDefaultReportWindow() {
  const now = new Date();
  if (!el.reportFrom.value) el.reportFrom.value = toInputValue(now);
  if (!el.reportTo.value) el.reportTo.value = toInputValue(new Date(now.getTime() + 24 * 60 * 60 * 1000));
}

function ensureDeviceID() {
  if (!state.deviceId) {
    const uuid = (crypto && crypto.randomUUID)
      ? crypto.randomUUID()
      : `dev-${Math.random().toString(36).slice(2)}-${Date.now()}`;
    state.deviceId = uuid;
    localStorage.setItem("deviceId", uuid);
  }
  return state.deviceId;
}

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (!headers["Content-Type"] && options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  const token = options.token !== undefined ? options.token : tokenForCurrentScreen();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error((data && data.error) || `HTTP ${response.status}`);
  }
  return data;
}

function tokenForCurrentScreen() {
  if (state.screen === SCREEN.STUDENT) return state.studentToken;
  if (state.screen === SCREEN.ADMIN) return state.adminToken;
  return "";
}

// ----- screen routing -----------------------------------------------------

function showScreen(screen) {
  state.screen = screen;
  el.landingPanel.hidden = screen !== SCREEN.LANDING;
  el.studentBookingPanel.hidden = screen !== SCREEN.STUDENT;
  el.adminLoginPanel.hidden = !(screen === SCREEN.ADMIN && !state.adminToken);
  el.adminPanel.hidden = !(screen === SCREEN.ADMIN && state.adminToken && state.user && state.user.role === "admin");
  el.backHomeBtn.hidden = screen === SCREEN.LANDING;

  if (screen === SCREEN.LANDING) {
    el.screenTitle.textContent = "Бронирование мест";
    el.statusBadge.textContent = "Не выбрана роль";
  } else if (screen === SCREEN.STUDENT) {
    el.screenTitle.textContent = "Студент / сотрудник";
    el.statusBadge.textContent = `Устройство: ${state.deviceId.slice(0, 8)}…`;
  } else if (screen === SCREEN.ADMIN) {
    el.screenTitle.textContent = "Администратор";
    el.statusBadge.textContent = state.adminToken && state.user
      ? `${state.user.email} (admin)`
      : "Не авторизован";
  }
}

function goHome() {
  // Drop tokens silently on going back so a shared device can switch roles.
  state.studentToken = "";
  state.adminToken = "";
  state.user = null;
  localStorage.removeItem("studentToken");
  localStorage.removeItem("adminToken");
  showScreen(SCREEN.LANDING);
}

async function chooseStudent() {
  try {
    ensureDeviceID();
    if (!state.studentToken) {
      const result = await api("/api/auth/device", {
        method: "POST",
        body: { device_id: state.deviceId },
        token: "",
      });
      state.studentToken = result.token;
      localStorage.setItem("studentToken", state.studentToken);
    }
    setDefaultStudentWindow();
    showScreen(SCREEN.STUDENT);
    log(`Студенческая сессия по устройству ${state.deviceId.slice(0, 8)}…`);
    await loadStudentBookings();
  } catch (err) {
    fmtError(err);
  }
}

function chooseAdmin() {
  showScreen(SCREEN.ADMIN);
}

// ----- student flow (UC-2, UC-3, UC-4) ------------------------------------

async function loadStudentSeats() {
  try {
    const startISO = toISO(el.studentStartAt.value);
    const endISO = toISO(el.studentEndAt.value);
    if (!startISO || !endISO) { fmtError(new Error("Укажите начало и конец интервала.")); return; }
    const params = new URLSearchParams({ start_at: startISO, end_at: endISO });
    const seats = await api(`/api/seats/available?${params.toString()}`);
    el.studentSeatsTable.innerHTML = "";
    if (!seats.length) {
      el.studentSeatsTable.innerHTML = `<tr><td colspan="5" class="hint">Свободных мест нет.</td></tr>`;
      log("Свободных мест нет на выбранный интервал");
      return;
    }
    for (const s of seats) {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${s.id}</td>
        <td>${s.name}</td>
        <td>${s.zone}</td>
        <td>${s.type}</td>
        <td><button data-seat="${s.id}">Забронировать</button></td>`;
      tr.querySelector("button").addEventListener("click", () => bookSeat(s.id));
      el.studentSeatsTable.appendChild(tr);
    }
    log(`Загружено свободных мест: ${seats.length}`);
  } catch (err) {
    fmtError(err);
  }
}

async function bookSeat(seatID) {
  const name = (el.studentName.value || "").trim();
  if (!name) { fmtError(new Error("Введите имя — оно сохранится в брони.")); return; }
  try {
    const result = await api("/api/bookings", {
      method: "POST",
      body: {
        seat_id: seatID,
        start_at: toISO(el.studentStartAt.value),
        end_at: toISO(el.studentEndAt.value),
        display_name: name,
      },
    });
    log(`Бронь #${result.id} создана для «${result.display_name || name}»`);
    await Promise.all([loadStudentSeats(), loadStudentBookings()]);
  } catch (err) {
    fmtError(err);
  }
}

async function loadStudentBookings() {
  if (!state.studentToken) return;
  try {
    const bookings = await api("/api/bookings/me");
    el.studentBookingsTable.innerHTML = "";
    if (!bookings.length) {
      el.studentBookingsTable.innerHTML = `<tr><td colspan="7" class="hint">Пока нет броней.</td></tr>`;
      return;
    }
    for (const b of bookings) {
      const tr = document.createElement("tr");
      const canCancel = b.status === "confirmed";
      tr.innerHTML = `
        <td>${b.id}</td>
        <td>${b.seat_id}</td>
        <td>${b.display_name || ""}</td>
        <td>${fmtDt(b.start_at)}</td>
        <td>${fmtDt(b.end_at)}</td>
        <td>${b.status}</td>
        <td>${canCancel ? `<button data-id="${b.id}" class="danger">Отменить</button>` : ""}</td>`;
      const cancelBtn = tr.querySelector("button");
      if (cancelBtn) cancelBtn.addEventListener("click", () => cancelStudentBooking(b.id));
      el.studentBookingsTable.appendChild(tr);
    }
  } catch (err) {
    fmtError(err);
  }
}

async function cancelStudentBooking(id) {
  try {
    await api(`/api/bookings/${id}`, { method: "DELETE" });
    log(`Бронь #${id} отменена`);
    await Promise.all([loadStudentSeats(), loadStudentBookings()]);
  } catch (err) {
    fmtError(err);
  }
}

// ----- admin flow (UC-1 then UC-5/6/7/8) ----------------------------------

async function adminLogin() {
  try {
    const email = el.adminEmail.value.trim();
    const password = el.adminPassword.value;
    if (!email || !password) { fmtError(new Error("Введите email и пароль.")); return; }
    const result = await api("/api/auth/login", {
      method: "POST",
      body: { email, password },
      token: "",
    });
    state.adminToken = result.token;
    const me = await api("/api/auth/me", { token: state.adminToken });
    if (me.role !== "admin") {
      state.adminToken = "";
      throw new Error("Эта учётная запись не админская. Используйте студенческий вход.");
    }
    state.user = me;
    localStorage.setItem("adminToken", state.adminToken);
    setDefaultReportWindow();
    showScreen(SCREEN.ADMIN);
    log(`Админ-вход: ${me.email}`);
  } catch (err) {
    state.adminToken = "";
    state.user = null;
    localStorage.removeItem("adminToken");
    fmtError(err);
  }
}

async function createSeat() {
  try {
    const seat = await api("/api/admin/seats", {
      method: "POST",
      body: {
        name: el.seatName.value.trim(),
        zone: el.seatZone.value.trim(),
        type: el.seatType.value.trim(),
        active: true,
      },
    });
    log(`Создано место #${seat.id} «${seat.name}»`);
  } catch (err) { fmtError(err); }
}

async function updateSeat() {
  try {
    const id = parseInt(el.seatId.value, 10);
    if (!id) throw new Error("Укажите ID места.");
    const seat = await api(`/api/admin/seats/${id}`, {
      method: "PUT",
      body: {
        name: el.seatName.value.trim(),
        zone: el.seatZone.value.trim(),
        type: el.seatType.value.trim(),
        active: true,
      },
    });
    log(`Обновлено место #${seat.id} «${seat.name}»`);
  } catch (err) { fmtError(err); }
}

async function deleteSeat() {
  try {
    const id = parseInt(el.seatId.value, 10);
    if (!id) throw new Error("Укажите ID места.");
    await api(`/api/admin/seats/${id}`, { method: "DELETE" });
    log(`Удалено место #${id}`);
  } catch (err) { fmtError(err); }
}

async function loadAllBookings() {
  try {
    const bookings = await api("/api/admin/bookings");
    el.allBookingsTable.innerHTML = "";
    if (!bookings.length) {
      el.allBookingsTable.innerHTML = `<tr><td colspan="7" class="hint">Бронирований пока нет.</td></tr>`;
      return;
    }
    for (const b of bookings) {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${b.id}</td>
        <td>${b.display_name || `user#${b.user_id}`}</td>
        <td>${b.seat_id}</td>
        <td>${fmtDt(b.start_at)}</td>
        <td>${fmtDt(b.end_at)}</td>
        <td>${b.status}</td>
        <td>${b.status === "confirmed" ? `<button data-id="${b.id}" class="danger">Отменить</button>` : ""}</td>`;
      const btn = tr.querySelector("button");
      if (btn) btn.addEventListener("click", () => adminCancelBooking(b.id));
      el.allBookingsTable.appendChild(tr);
    }
    log(`Загружено броней: ${bookings.length}`);
  } catch (err) { fmtError(err); }
}

async function adminCancelBooking(id) {
  try {
    await api(`/api/admin/bookings/${id}`, {
      method: "PATCH",
      body: { status: "canceled" },
    });
    log(`Админ отменил бронь #${id}`);
    await loadAllBookings();
  } catch (err) { fmtError(err); }
}

async function updateLimit() {
  try {
    const limit = parseInt(el.bookingLimit.value, 10);
    if (!limit) throw new Error("Укажите положительное число.");
    await api("/api/admin/settings/limit", {
      method: "PUT",
      body: { limit },
    });
    log(`Лимит активных броней обновлён: ${limit}`);
  } catch (err) { fmtError(err); }
}

async function runReport() {
  try {
    const params = new URLSearchParams({
      from: toISO(el.reportFrom.value),
      to: toISO(el.reportTo.value),
    });
    const result = await api(`/api/admin/reports?${params.toString()}`);
    el.reportResult.textContent = JSON.stringify(result, null, 2);
    log(`Отчёт: всего ${result.total_bookings}, отменено ${result.canceled_bookings}`);
  } catch (err) { fmtError(err); }
}

// ----- bootstrap ----------------------------------------------------------

function bindEvents() {
  el.chooseStudentBtn.addEventListener("click", chooseStudent);
  el.chooseAdminBtn.addEventListener("click", chooseAdmin);
  el.backHomeBtn.addEventListener("click", goHome);

  el.studentLoadSeatsBtn.addEventListener("click", loadStudentSeats);
  el.studentRefreshBtn.addEventListener("click", loadStudentBookings);

  el.adminLoginBtn.addEventListener("click", adminLogin);
  el.adminPassword.addEventListener("keydown", (e) => { if (e.key === "Enter") adminLogin(); });

  el.createSeatBtn.addEventListener("click", createSeat);
  el.updateSeatBtn.addEventListener("click", updateSeat);
  el.deleteSeatBtn.addEventListener("click", deleteSeat);
  el.loadAllBookingsBtn.addEventListener("click", loadAllBookings);
  el.updateLimitBtn.addEventListener("click", updateLimit);
  el.runReportBtn.addEventListener("click", runReport);
}

async function init() {
  bindEvents();
  if (window.Telegram && window.Telegram.WebApp) {
    try { window.Telegram.WebApp.expand(); } catch (_) {}
  }

  // Try to recover an existing screen state silently.
  if (state.adminToken) {
    try {
      const me = await api("/api/auth/me", { token: state.adminToken });
      if (me.role === "admin") {
        state.user = me;
        setDefaultReportWindow();
        showScreen(SCREEN.ADMIN);
        return;
      }
    } catch (_) { /* fallthrough to landing */ }
    state.adminToken = "";
    localStorage.removeItem("adminToken");
  }
  if (state.studentToken && state.deviceId) {
    try {
      await api("/api/auth/me", { token: state.studentToken });
      setDefaultStudentWindow();
      showScreen(SCREEN.STUDENT);
      await loadStudentBookings();
      return;
    } catch (_) { /* fallthrough */ }
    state.studentToken = "";
    localStorage.removeItem("studentToken");
  }
  showScreen(SCREEN.LANDING);
}

init();
