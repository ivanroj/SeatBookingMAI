// Coworking Seat Booking — frontend (vanilla JS, no build step).
//
// Three top-level screens:
//   landing  → choose role (student / admin)
//   student  → list coworkings → grid map (clickable squares) → modal book
//   admin    → email/password login → tabbed panel (coworkings & seat editor,
//              bookings, limit, report, logs)
//
// All UX wiring happens after DOMContentLoaded so the file can be loaded
// regardless of script ordering. Errors surface as toasts; raw JSON, table
// IDs and developer-only controls are hidden from the student.

const SCREEN = Object.freeze({
  LANDING: "landing",
  STUDENT_COWORKINGS: "student_coworkings",
  STUDENT_MAP: "student_map",
  ADMIN_LOGIN: "admin_login",
  ADMIN: "admin",
});

const SEAT_TYPES = {
  desk: "Стандартное",
  pc: "С ПК",
  meeting: "Переговорная",
  quiet: "Тихая зона",
};

const GRID_COLS = 8;
const GRID_ROWS = 6;

const state = {
  screen: SCREEN.LANDING,
  studentToken: localStorage.getItem("studentToken") || "",
  adminToken: localStorage.getItem("adminToken") || "",
  user: null,
  deviceId: localStorage.getItem("deviceId") || "",
  // Student
  coworkings: [],
  selectedCoworkingId: null,
  studentMapSeats: [], // [{id, coworking_id, name, grid_x, grid_y, type, active, is_busy}]
  studentBookings: [],
  // Admin
  adminCoworkings: [],
  adminSelectedCoworkingId: null,
  adminMapSeats: [],
  // caches for admin all-bookings rendering
  seatNameCache: {}, // seatId → seatName
};

const el = {};

function $(id) { return document.getElementById(id); }

function bindElements() {
  [
    "screenTitle", "statusBadge", "backHomeBtn",
    "landingPanel", "chooseStudentBtn", "chooseAdminBtn",
    "studentCoworkingsPanel", "studentCoworkingList",
    "studentMapPanel", "studentMapTitle",
    "studentBackToCoworkingsBtn",
    "studentName", "studentStartAt", "studentEndAt",
    "studentReloadMapBtn", "studentMap", "studentBookingsList",
    "adminLoginPanel", "adminEmail", "adminPassword", "adminLoginBtn",
    "adminPanel",
    "adminNewCoworkingBtn", "adminCoworkingList",
    "adminGridSection", "adminGridTitle", "adminMap",
    "adminEditCoworkingBtn", "adminDeleteCoworkingBtn",
    "loadAllBookingsBtn", "allBookingsTable",
    "bookingLimit", "updateLimitBtn",
    "reportFrom", "reportTo", "runReportBtn",
    "reportKpis", "kpiTotal", "kpiCanceled", "kpiWindow",
    "reportSeatsWrap", "reportSeatsTable",
    "reloadLogsBtn", "logList",
    "bookingDialog", "bookingDialogSeat", "bookingDialogWindow", "bookingDialogConfirm",
    "seatDialog", "seatDialogTitle", "seatDialogPos", "seatDialogName",
    "seatDialogZone", "seatDialogType", "seatDialogActive",
    "seatDialogDelete", "seatDialogSave",
    "coworkingDialog", "coworkingDialogTitle", "coworkingDialogName", "coworkingDialogCapacity", "coworkingDialogSave",
    "toastStack",
  ].forEach((k) => { el[k] = $(k); });
}

// ----- toast / error helpers -------------------------------------------------

function toast(message, kind = "info") {
  if (!el.toastStack) { console.log(message); return; }
  const node = document.createElement("div");
  node.className = `toast toast-${kind}`;
  node.textContent = message;
  el.toastStack.appendChild(node);
  setTimeout(() => { node.classList.add("toast-out"); }, 3500);
  setTimeout(() => { node.remove(); }, 4000);
}

function showError(err) {
  const message = (err && err.message) ? err.message : String(err || "Ошибка");
  toast(message, "error");
}

// ----- date helpers ----------------------------------------------------------

function pad(n) { return String(n).padStart(2, "0"); }

function toInputValue(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toISO(value) {
  if (!value) return "";
  return new Date(value).toISOString();
}

function fmtDt(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
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
  if (!el.reportFrom.value) el.reportFrom.value = toInputValue(new Date(now.getTime() - 7 * 24 * 3600 * 1000));
  if (!el.reportTo.value) el.reportTo.value = toInputValue(new Date(now.getTime() + 24 * 3600 * 1000));
}

// ----- device id ------------------------------------------------------------

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

// ----- API client -----------------------------------------------------------

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (!headers["Content-Type"] && options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  const token = options.token !== undefined
    ? options.token
    : (state.screen.startsWith("admin") ? state.adminToken : state.studentToken);
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const url = path.startsWith("http") ? path : `/api${path}`;
  const resp = await fetch(url, { ...options, headers });
  if (resp.status === 204) return null;
  let body = null;
  const contentType = resp.headers.get("Content-Type") || "";
  if (contentType.includes("application/json")) {
    body = await resp.json().catch(() => null);
  } else {
    body = await resp.text();
  }
  if (!resp.ok) {
    const message = (body && body.error) ? body.error : (typeof body === "string" && body) || `Ошибка ${resp.status}`;
    const err = new Error(message);
    err.status = resp.status;
    err.body = body;
    throw err;
  }
  return body;
}

// ----- screen routing -------------------------------------------------------

function setScreen(screen) {
  state.screen = screen;
  el.landingPanel.hidden = screen !== SCREEN.LANDING;
  el.studentCoworkingsPanel.hidden = screen !== SCREEN.STUDENT_COWORKINGS;
  el.studentMapPanel.hidden = screen !== SCREEN.STUDENT_MAP;
  el.adminLoginPanel.hidden = screen !== SCREEN.ADMIN_LOGIN;
  el.adminPanel.hidden = screen !== SCREEN.ADMIN;
  el.backHomeBtn.hidden = screen === SCREEN.LANDING;

  switch (screen) {
    case SCREEN.LANDING:
      el.screenTitle.textContent = "Коворкинг";
      el.statusBadge.hidden = true;
      break;
    case SCREEN.STUDENT_COWORKINGS:
      el.screenTitle.textContent = "Бронирование";
      el.statusBadge.hidden = false;
      el.statusBadge.textContent = "Студент";
      el.statusBadge.className = "badge badge-student";
      break;
    case SCREEN.STUDENT_MAP:
      el.screenTitle.textContent = "Бронирование";
      el.statusBadge.hidden = false;
      el.statusBadge.textContent = "Студент";
      el.statusBadge.className = "badge badge-student";
      break;
    case SCREEN.ADMIN_LOGIN:
      el.screenTitle.textContent = "Администратор";
      el.statusBadge.hidden = true;
      break;
    case SCREEN.ADMIN:
      el.screenTitle.textContent = "Админ-панель";
      el.statusBadge.hidden = false;
      el.statusBadge.textContent = state.user ? state.user.email : "Администратор";
      el.statusBadge.className = "badge badge-admin";
      break;
  }
}

// ----- landing --------------------------------------------------------------

async function chooseStudent() {
  try {
    const deviceID = ensureDeviceID();
    const resp = await api("/auth/device", {
      method: "POST",
      body: JSON.stringify({ device_id: deviceID }),
      token: "",
    });
    state.studentToken = resp.token;
    localStorage.setItem("studentToken", resp.token);
    setScreen(SCREEN.STUDENT_COWORKINGS);
    await loadStudentCoworkings();
  } catch (err) {
    showError(err);
  }
}

function chooseAdmin() {
  setScreen(SCREEN.ADMIN_LOGIN);
}

function backHome() {
  setScreen(SCREEN.LANDING);
}

// ----- student: coworking list ----------------------------------------------

async function loadStudentCoworkings() {
  el.studentCoworkingList.innerHTML = '<p class="hint">Загрузка…</p>';
  try {
    const list = await api("/coworkings", { token: "" });
    state.coworkings = list || [];
    if (!state.coworkings.length) {
      el.studentCoworkingList.innerHTML = '<p class="hint">Пока нет коворкингов. Свяжитесь с администратором.</p>';
      return;
    }
    el.studentCoworkingList.innerHTML = "";
    for (const cw of state.coworkings) {
      const card = document.createElement("button");
      card.className = "coworking-card";
      card.type = "button";
      card.innerHTML = `
        <span class="coworking-card-name"></span>
        <span class="coworking-card-meta">Мест: <b></b></span>
        <span class="coworking-card-cta">Выбрать места →</span>`;
      card.querySelector(".coworking-card-name").textContent = cw.name;
      card.querySelector(".coworking-card-meta b").textContent = cw.capacity;
      card.addEventListener("click", () => openStudentMap(cw));
      el.studentCoworkingList.appendChild(card);
    }
  } catch (err) {
    el.studentCoworkingList.innerHTML = '<p class="hint">Не удалось загрузить коворкинги.</p>';
    showError(err);
  }
}

async function openStudentMap(cw) {
  state.selectedCoworkingId = cw.id;
  el.studentMapTitle.textContent = cw.name;
  setScreen(SCREEN.STUDENT_MAP);
  setDefaultStudentWindow();
  await reloadStudentMap();
  await reloadStudentBookings();
}

async function reloadStudentMap() {
  if (!state.selectedCoworkingId) return;
  const start = el.studentStartAt.value;
  const end = el.studentEndAt.value;
  if (!start || !end) return;
  el.studentMap.innerHTML = '<p class="hint">Загрузка схемы…</p>';
  try {
    const seats = await api(`/coworkings/${state.selectedCoworkingId}/map?start_at=${encodeURIComponent(toISO(start))}&end_at=${encodeURIComponent(toISO(end))}`);
    state.studentMapSeats = seats || [];
    renderStudentMap();
  } catch (err) {
    el.studentMap.innerHTML = '<p class="hint">Не удалось загрузить схему.</p>';
    showError(err);
  }
}

function renderStudentMap() {
  const seats = state.studentMapSeats;
  el.studentMap.innerHTML = "";
  el.studentMap.style.gridTemplateColumns = `repeat(${GRID_COLS}, minmax(56px, 1fr))`;
  el.studentMap.style.gridAutoRows = "56px";

  if (!seats.length) {
    const p = document.createElement("p");
    p.className = "hint";
    p.textContent = "В этом коворкинге пока нет мест.";
    el.studentMap.appendChild(p);
    return;
  }

  // Render seats based on their grid_x/grid_y.
  for (const seat of seats) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "seat";
    btn.style.gridColumn = String((seat.grid_x || 0) + 1);
    btn.style.gridRow = String((seat.grid_y || 0) + 1);
    btn.dataset.seatId = String(seat.id);
    const isUsable = seat.active && !seat.is_busy;
    if (!seat.active) {
      btn.classList.add("seat-inactive");
      btn.disabled = true;
      btn.title = "Не используется";
    } else if (seat.is_busy) {
      btn.classList.add("seat-busy");
      btn.disabled = true;
      btn.title = "Занято";
    } else {
      btn.classList.add("seat-free");
    }
    const label = document.createElement("span");
    label.className = "seat-label";
    label.textContent = seat.label || seat.name || `${seat.grid_x},${seat.grid_y}`;
    btn.appendChild(label);
    if (isUsable) {
      btn.addEventListener("click", () => openBookingDialog(seat));
    }
    el.studentMap.appendChild(btn);
  }
}

async function openBookingDialog(seat) {
  const name = (el.studentName.value || "").trim();
  if (!name) {
    showError(new Error("Введите имя — оно появится у администратора."));
    el.studentName.focus();
    return;
  }
  const start = el.studentStartAt.value;
  const end = el.studentEndAt.value;
  if (!start || !end) {
    showError(new Error("Заполните период."));
    return;
  }
  el.bookingDialogSeat.textContent = `${seat.name}${seat.label ? " — " + seat.label : ""} (${SEAT_TYPES[seat.type] || seat.type || "место"})`;
  el.bookingDialogWindow.textContent = `${fmtDt(toISO(start))} — ${fmtDt(toISO(end))}`;
  el.bookingDialog.dataset.seatId = String(seat.id);
  el.bookingDialog.showModal();
}

async function confirmBooking(seatId) {
  try {
    await api("/bookings", {
      method: "POST",
      body: JSON.stringify({
        seat_id: seatId,
        start_at: toISO(el.studentStartAt.value),
        end_at: toISO(el.studentEndAt.value),
        display_name: el.studentName.value.trim(),
      }),
    });
    toast("Место забронировано.", "success");
    await reloadStudentMap();
    await reloadStudentBookings();
  } catch (err) {
    showError(err);
  }
}

async function reloadStudentBookings() {
  try {
    const bookings = await api("/bookings/me");
    state.studentBookings = bookings || [];
    renderStudentBookings();
  } catch (err) {
    showError(err);
  }
}

function renderStudentBookings() {
  const items = state.studentBookings;
  el.studentBookingsList.innerHTML = "";
  if (!items.length) {
    const li = document.createElement("li");
    li.className = "hint";
    li.textContent = "Бронирований пока нет.";
    el.studentBookingsList.appendChild(li);
    return;
  }
  const seatById = Object.fromEntries(state.studentMapSeats.map((s) => [s.id, s]));
  for (const b of items) {
    const seat = seatById[b.seat_id];
    const li = document.createElement("li");
    li.className = `booking-row booking-${b.status}`;
    const seatTxt = seat ? `${seat.name}` : "—";
    li.innerHTML = `
      <span class="booking-row-seat"></span>
      <span class="booking-row-when"></span>
      <span class="booking-row-status"></span>`;
    li.querySelector(".booking-row-seat").textContent = seatTxt;
    li.querySelector(".booking-row-when").textContent = `${fmtDt(b.start_at)} → ${fmtDt(b.end_at)}`;
    const statusEl = li.querySelector(".booking-row-status");
    statusEl.textContent = b.status === "confirmed" ? "подтверждено" : b.status === "canceled" ? "отменено" : "завершено";
    if (b.status === "confirmed") {
      const cancelBtn = document.createElement("button");
      cancelBtn.className = "ghost ghost-small";
      cancelBtn.textContent = "Отменить";
      cancelBtn.addEventListener("click", async () => {
        try {
          await api(`/bookings/${b.id}`, { method: "DELETE" });
          toast("Бронь отменена.", "success");
          await reloadStudentMap();
          await reloadStudentBookings();
        } catch (err) {
          showError(err);
        }
      });
      li.appendChild(cancelBtn);
    }
    el.studentBookingsList.appendChild(li);
  }
}

// ----- admin: login ---------------------------------------------------------

async function adminLogin() {
  const email = (el.adminEmail.value || "").trim();
  const password = el.adminPassword.value || "";
  if (!email || !password) {
    showError(new Error("Введите email и пароль."));
    return;
  }
  try {
    const resp = await api("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
      token: "",
    });
    state.adminToken = resp.token;
    localStorage.setItem("adminToken", resp.token);
    const me = await api("/auth/me", { token: resp.token });
    if (me.role !== "admin") {
      throw new Error("Эта учётка не админская.");
    }
    state.user = me;
    setScreen(SCREEN.ADMIN);
    await loadAdminCoworkings();
    setDefaultReportWindow();
  } catch (err) {
    showError(err);
  }
}

// ----- admin: tabs ----------------------------------------------------------

function bindAdminTabs() {
  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      const target = tab.dataset.tab;
      document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("is-active", t === tab));
      document.querySelectorAll(".tab-panel").forEach((p) => {
        p.hidden = p.dataset.tab !== target;
      });
      if (target === "logs") loadAdminLogs();
      if (target === "settings") loadSettings();
    });
  });
}

// ----- admin: coworkings + grid editor --------------------------------------

async function loadAdminCoworkings() {
  el.adminCoworkingList.innerHTML = '<p class="hint">Загрузка…</p>';
  try {
    const list = await api("/admin/coworkings");
    state.adminCoworkings = list || [];
    el.adminCoworkingList.innerHTML = "";
    if (!state.adminCoworkings.length) {
      el.adminCoworkingList.innerHTML = '<p class="hint">Пока нет коворкингов.</p>';
      el.adminGridSection.hidden = true;
      return;
    }
    for (const cw of state.adminCoworkings) {
      const card = document.createElement("button");
      card.className = "coworking-card";
      card.type = "button";
      card.innerHTML = `
        <span class="coworking-card-name"></span>
        <span class="coworking-card-meta">Мест: <b></b></span>`;
      card.querySelector(".coworking-card-name").textContent = cw.name;
      card.querySelector(".coworking-card-meta b").textContent = cw.capacity;
      if (cw.id === state.adminSelectedCoworkingId) card.classList.add("is-selected");
      card.addEventListener("click", () => openAdminGrid(cw));
      el.adminCoworkingList.appendChild(card);
    }
    if (state.adminSelectedCoworkingId) {
      const stillThere = state.adminCoworkings.find((c) => c.id === state.adminSelectedCoworkingId);
      if (stillThere) {
        await openAdminGrid(stillThere);
      } else {
        state.adminSelectedCoworkingId = null;
        el.adminGridSection.hidden = true;
      }
    }
  } catch (err) {
    el.adminCoworkingList.innerHTML = '<p class="hint">Не удалось загрузить коворкинги.</p>';
    showError(err);
  }
}

async function openAdminGrid(cw) {
  state.adminSelectedCoworkingId = cw.id;
  el.adminGridSection.hidden = false;
  el.adminGridTitle.textContent = `Схема: ${cw.name}`;
  document.querySelectorAll("#adminCoworkingList .coworking-card").forEach((node, idx) => {
    node.classList.toggle("is-selected", state.adminCoworkings[idx] && state.adminCoworkings[idx].id === cw.id);
  });
  await reloadAdminMap();
}

async function reloadAdminMap() {
  try {
    const seats = await api(`/admin/coworkings/${state.adminSelectedCoworkingId}/seats`);
    state.adminMapSeats = seats || [];
    state.seatNameCache = {};
    for (const s of state.adminMapSeats) state.seatNameCache[s.id] = s.name;
    renderAdminMap();
  } catch (err) {
    showError(err);
  }
}

function renderAdminMap() {
  const seats = state.adminMapSeats;
  const byPos = {};
  for (const s of seats) byPos[`${s.grid_x},${s.grid_y}`] = s;

  el.adminMap.innerHTML = "";
  el.adminMap.style.gridTemplateColumns = `repeat(${GRID_COLS}, minmax(56px, 1fr))`;
  el.adminMap.style.gridAutoRows = "56px";

  for (let y = 0; y < GRID_ROWS; y++) {
    for (let x = 0; x < GRID_COLS; x++) {
      const seat = byPos[`${x},${y}`];
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "seat";
      btn.style.gridColumn = String(x + 1);
      btn.style.gridRow = String(y + 1);
      if (seat) {
        btn.classList.add(seat.active ? "seat-free" : "seat-inactive");
        const label = document.createElement("span");
        label.className = "seat-label";
        label.textContent = seat.label || seat.name;
        btn.appendChild(label);
        btn.addEventListener("click", () => openSeatDialog(seat, x, y));
      } else {
        btn.classList.add("seat-empty");
        btn.textContent = "+";
        btn.title = `Создать место в (${x},${y})`;
        btn.addEventListener("click", () => openSeatDialog(null, x, y));
      }
      el.adminMap.appendChild(btn);
    }
  }
}

function openSeatDialog(seat, gridX, gridY) {
  el.seatDialog.dataset.seatId = seat ? String(seat.id) : "";
  el.seatDialog.dataset.gridX = String(gridX);
  el.seatDialog.dataset.gridY = String(gridY);
  el.seatDialogTitle.textContent = seat ? `Место ${seat.name}` : "Новое место";
  el.seatDialogPos.textContent = `Позиция: (${gridX}, ${gridY})`;
  el.seatDialogName.value = seat ? seat.name : suggestNextSeatName();
  el.seatDialogZone.value = seat ? (seat.zone || "") : "";
  el.seatDialogType.value = seat && seat.type ? seat.type : "desk";
  el.seatDialogActive.checked = seat ? seat.active : true;
  el.seatDialogDelete.hidden = !seat;
  el.seatDialog.showModal();
}

function suggestNextSeatName() {
  // Find next free letter-prefix name based on row letter A-Z.
  const used = new Set(state.adminMapSeats.map((s) => s.name));
  for (let i = 1; i < 200; i++) {
    const candidate = `S-${String(i).padStart(2, "0")}`;
    if (!used.has(candidate)) return candidate;
  }
  return "";
}

async function saveSeatFromDialog() {
  const seatId = el.seatDialog.dataset.seatId;
  const gridX = parseInt(el.seatDialog.dataset.gridX || "0", 10);
  const gridY = parseInt(el.seatDialog.dataset.gridY || "0", 10);
  const payload = {
    coworking_id: state.adminSelectedCoworkingId,
    name: (el.seatDialogName.value || "").trim(),
    zone: (el.seatDialogZone.value || "").trim(),
    type: el.seatDialogType.value,
    grid_x: gridX,
    grid_y: gridY,
    active: el.seatDialogActive.checked,
  };
  if (!payload.name) {
    showError(new Error("Укажите номер места."));
    return;
  }
  try {
    if (seatId) {
      await api(`/admin/seats/${seatId}`, { method: "PUT", body: JSON.stringify(payload) });
      toast("Место обновлено.", "success");
    } else {
      await api("/admin/seats", { method: "POST", body: JSON.stringify(payload) });
      toast("Место создано.", "success");
    }
    await reloadAdminMap();
  } catch (err) {
    showError(err);
  }
}

async function deleteSeatFromDialog() {
  const seatId = el.seatDialog.dataset.seatId;
  if (!seatId) return;
  if (!confirm("Удалить это место?")) return;
  try {
    await api(`/admin/seats/${seatId}`, { method: "DELETE" });
    toast("Место удалено.", "success");
    await reloadAdminMap();
    el.seatDialog.close();
  } catch (err) {
    showError(err);
  }
}

function openCoworkingDialog(cw) {
  el.coworkingDialog.dataset.cwId = cw ? String(cw.id) : "";
  el.coworkingDialogTitle.textContent = cw ? "Изменить коворкинг" : "Новый коворкинг";
  el.coworkingDialogName.value = cw ? cw.name : "";
  el.coworkingDialogCapacity.value = cw ? cw.capacity : 20;
  el.coworkingDialog.showModal();
}

async function saveCoworkingFromDialog() {
  const cwId = el.coworkingDialog.dataset.cwId;
  const payload = {
    name: (el.coworkingDialogName.value || "").trim(),
    capacity: parseInt(el.coworkingDialogCapacity.value || "0", 10),
  };
  if (!payload.name) { showError(new Error("Укажите название.")); return; }
  if (!Number.isFinite(payload.capacity) || payload.capacity <= 0) {
    showError(new Error("Укажите количество мест.")); return;
  }
  try {
    if (cwId) {
      await api(`/admin/coworkings/${cwId}`, { method: "PATCH", body: JSON.stringify(payload) });
      toast("Коворкинг обновлён.", "success");
    } else {
      const created = await api("/admin/coworkings", { method: "POST", body: JSON.stringify(payload) });
      state.adminSelectedCoworkingId = created && created.id;
      toast("Коворкинг создан.", "success");
    }
    await loadAdminCoworkings();
  } catch (err) {
    showError(err);
  }
}

async function deleteSelectedCoworking() {
  const id = state.adminSelectedCoworkingId;
  if (!id) return;
  if (!confirm("Удалить этот коворкинг и все его места?")) return;
  try {
    await api(`/admin/coworkings/${id}`, { method: "DELETE" });
    toast("Коворкинг удалён.", "success");
    state.adminSelectedCoworkingId = null;
    el.adminGridSection.hidden = true;
    await loadAdminCoworkings();
  } catch (err) {
    showError(err);
  }
}

// ----- admin: bookings ------------------------------------------------------

async function loadAllBookings() {
  el.allBookingsTable.innerHTML = '<tr><td colspan="6" class="hint">Загрузка…</td></tr>';
  try {
    const items = await api("/admin/bookings");
    if (!items || !items.length) {
      el.allBookingsTable.innerHTML = '<tr><td colspan="6" class="hint">Нет броней.</td></tr>';
      return;
    }
    // ensure seat name cache for any seat we don't yet know
    const unknownSeats = items.map((b) => b.seat_id).filter((id) => !state.seatNameCache[id]);
    if (unknownSeats.length && state.adminCoworkings.length) {
      // load seats for all coworkings (cheap; usually 1-3 of them)
      for (const cw of state.adminCoworkings) {
        try {
          const seats = await api(`/admin/coworkings/${cw.id}/seats`);
          for (const s of seats || []) state.seatNameCache[s.id] = s.name;
        } catch (e) { /* ignore */ }
      }
    }
    el.allBookingsTable.innerHTML = "";
    for (const b of items) {
      const tr = document.createElement("tr");
      const cells = [
        b.display_name || "—",
        state.seatNameCache[b.seat_id] || `#${b.seat_id}`,
        fmtDt(b.start_at),
        fmtDt(b.end_at),
        b.status === "confirmed" ? "подтверждено" : b.status === "canceled" ? "отменено" : "завершено",
      ];
      for (const c of cells) {
        const td = document.createElement("td");
        td.textContent = c;
        tr.appendChild(td);
      }
      const actionTd = document.createElement("td");
      if (b.status === "confirmed") {
        const cancelBtn = document.createElement("button");
        cancelBtn.className = "ghost ghost-small";
        cancelBtn.textContent = "Отменить";
        cancelBtn.addEventListener("click", async () => {
          try {
            await api(`/admin/bookings/${b.id}`, {
              method: "PATCH",
              body: JSON.stringify({ status: "canceled" }),
            });
            toast("Бронь отменена.", "success");
            await loadAllBookings();
          } catch (err) {
            showError(err);
          }
        });
        actionTd.appendChild(cancelBtn);
      } else {
        actionTd.textContent = "—";
      }
      tr.appendChild(actionTd);
      el.allBookingsTable.appendChild(tr);
    }
  } catch (err) {
    el.allBookingsTable.innerHTML = '<tr><td colspan="6" class="hint">Не удалось загрузить.</td></tr>';
    showError(err);
  }
}

// ----- admin: limit ---------------------------------------------------------

async function loadSettings() {
  // Backend doesn't expose GET settings explicitly — rely on what we know.
  // (The PUT endpoint is sufficient for the requirement.)
}

async function updateLimit() {
  const limit = parseInt(el.bookingLimit.value || "0", 10);
  if (!Number.isFinite(limit) || limit <= 0) { showError(new Error("Лимит должен быть положительным.")); return; }
  try {
    await api("/admin/settings/limit", {
      method: "PUT",
      body: JSON.stringify({ limit }),
    });
    toast("Лимит сохранён.", "success");
  } catch (err) {
    showError(err);
  }
}

// ----- admin: report --------------------------------------------------------

async function runReport() {
  const from = el.reportFrom.value;
  const to = el.reportTo.value;
  if (!from || !to) { showError(new Error("Заполните период.")); return; }
  try {
    const data = await api(`/admin/reports?from=${encodeURIComponent(toISO(from))}&to=${encodeURIComponent(toISO(to))}`);
    el.reportKpis.hidden = false;
    el.reportSeatsWrap.hidden = false;
    el.kpiTotal.textContent = data.total_bookings ?? 0;
    el.kpiCanceled.textContent = data.canceled_bookings ?? 0;
    el.kpiWindow.textContent = `${fmtDt(toISO(from))} — ${fmtDt(toISO(to))}`;
    el.reportSeatsTable.innerHTML = "";
    const bySeat = data.by_seat || {};
    const ids = Object.keys(bySeat);
    if (!ids.length) {
      const tr = document.createElement("tr");
      tr.innerHTML = '<td colspan="2" class="hint">Нет данных за указанный период.</td>';
      el.reportSeatsTable.appendChild(tr);
    } else {
      for (const id of ids) {
        const tr = document.createElement("tr");
        const tdName = document.createElement("td");
        tdName.textContent = state.seatNameCache[id] || `#${id}`;
        const tdCount = document.createElement("td");
        tdCount.textContent = bySeat[id];
        tr.appendChild(tdName); tr.appendChild(tdCount);
        el.reportSeatsTable.appendChild(tr);
      }
    }
  } catch (err) {
    showError(err);
  }
}

// ----- admin: logs ----------------------------------------------------------

async function loadAdminLogs() {
  el.logList.innerHTML = '<li class="hint">Загрузка…</li>';
  try {
    const items = await api("/admin/logs");
    if (!items || !items.length) {
      el.logList.innerHTML = '<li class="hint">Журнал пуст.</li>';
      return;
    }
    el.logList.innerHTML = "";
    for (const e of items) {
      const li = document.createElement("li");
      li.className = "log-row";
      const ts = document.createElement("span");
      ts.className = "log-row-ts";
      ts.textContent = fmtDt(e.at);
      const ev = document.createElement("span");
      ev.className = "log-row-event";
      ev.textContent = e.event;
      const det = document.createElement("span");
      det.className = "log-row-fields";
      det.textContent = formatLogFields(e.fields);
      li.appendChild(ts);
      li.appendChild(ev);
      li.appendChild(det);
      el.logList.appendChild(li);
    }
  } catch (err) {
    el.logList.innerHTML = '<li class="hint">Не удалось загрузить журнал.</li>';
    showError(err);
  }
}

function formatLogFields(fields) {
  if (!fields) return "";
  return Object.entries(fields)
    .filter(([k]) => k !== "device_id" && k !== "id")
    .map(([k, v]) => `${k}=${v}`)
    .join(", ");
}

// ----- bootstrap ------------------------------------------------------------

function bindEvents() {
  el.chooseStudentBtn.addEventListener("click", chooseStudent);
  el.chooseAdminBtn.addEventListener("click", chooseAdmin);
  el.backHomeBtn.addEventListener("click", backHome);

  el.studentBackToCoworkingsBtn.addEventListener("click", () => setScreen(SCREEN.STUDENT_COWORKINGS));
  el.studentReloadMapBtn.addEventListener("click", () => reloadStudentMap());
  el.studentStartAt.addEventListener("change", () => reloadStudentMap());
  el.studentEndAt.addEventListener("change", () => reloadStudentMap());

  el.adminLoginBtn.addEventListener("click", adminLogin);
  el.adminPassword.addEventListener("keydown", (e) => { if (e.key === "Enter") adminLogin(); });

  el.adminNewCoworkingBtn.addEventListener("click", () => openCoworkingDialog(null));
  el.adminEditCoworkingBtn.addEventListener("click", () => {
    const cw = state.adminCoworkings.find((c) => c.id === state.adminSelectedCoworkingId);
    if (cw) openCoworkingDialog(cw);
  });
  el.adminDeleteCoworkingBtn.addEventListener("click", deleteSelectedCoworking);

  el.loadAllBookingsBtn.addEventListener("click", loadAllBookings);
  el.updateLimitBtn.addEventListener("click", updateLimit);
  el.runReportBtn.addEventListener("click", runReport);
  el.reloadLogsBtn.addEventListener("click", loadAdminLogs);

  // Booking dialog (student).
  el.bookingDialog.addEventListener("close", () => {
    if (el.bookingDialog.returnValue === "confirm") {
      const seatId = parseInt(el.bookingDialog.dataset.seatId || "0", 10);
      if (seatId) confirmBooking(seatId);
    }
  });

  // Seat dialog (admin).
  el.seatDialog.addEventListener("close", () => {
    if (el.seatDialog.returnValue === "confirm") saveSeatFromDialog();
  });
  el.seatDialogDelete.addEventListener("click", (e) => {
    e.preventDefault();
    deleteSeatFromDialog();
  });

  // Coworking dialog.
  el.coworkingDialog.addEventListener("close", () => {
    if (el.coworkingDialog.returnValue === "confirm") saveCoworkingFromDialog();
  });
}

document.addEventListener("DOMContentLoaded", () => {
  bindElements();
  bindEvents();
  bindAdminTabs();
  setScreen(SCREEN.LANDING);
});
