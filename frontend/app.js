// API 已在 auth.js 中定义
if (!isLoggedIn()) {
  window.location.href = '/app/login.html';
}

let currentPage = 1;
let recordsPageSize = 20;
let deleteTargetId = null;
let currentRecords = [];
let currentSort = { field: 'date', dir: 'desc' }; // 默认按日期倒序
let allFilteredRecords = [];
let isAllRecordsSortMode = false;
let currentSummaryData = null;
let currentSummaryList = [];
let currentSummaryPage = 1;
let summaryPageSize = 20;
let currentSummaryMode = null;
let currentReportData = null;
let currentReportDaily = [];
let currentReportCategory = [];
let currentReportDailyPage = 1;
let currentReportCategoryPage = 1;
let reportDailyPageSize = 20;
let reportCategoryPageSize = 20;
let currentUsers = [];
let currentUsersPage = 1;
let usersPageSize = 20;
let currentLogs = [];
let allLogs = [];
let isAllLogsSortMode = false;
let currentLogsTotal = 0;
let currentLogsPage = 1;
let logsPageSize = 20;
// 各区域排序状态（用于正序 / 倒序切换）
let summarySort = { field: null, dir: 'desc' };
let summaryBreakdownSort = { field: null, dir: 'desc' };
let reportDailySort = { field: null, dir: 'desc' };
let reportCatSort = { field: null, dir: 'desc' };
let usersSort = { field: null, dir: 'desc' };
let logsSort = { field: null, dir: 'desc' };

// DOM
const tableBody = document.getElementById('tableBody');
const paginationEl = document.getElementById('pagination');
const recordsPageSizeSelect = document.getElementById('recordsPageSize');
const modal = document.getElementById('modal');
const deleteModal = document.getElementById('deleteModal');
const recordForm = document.getElementById('recordForm');
const modalTitle = document.getElementById('modalTitle');
const recordId = document.getElementById('recordId');
const formDate = document.getElementById('formDate');
const formAmount = document.getElementById('formAmount');
const formCategory = document.getElementById('formCategory');
const formDescription = document.getElementById('formDescription');

const thDate = document.getElementById('thDate');
const thAmount = document.getElementById('thAmount');
const thCategory = document.getElementById('thCategory');

const btnThemeToggle = document.getElementById('btnThemeToggle');
const btnAdd = document.getElementById('btnAdd');

const startDate = document.getElementById('startDate');
const endDate = document.getElementById('endDate');
const keyword = document.getElementById('keyword');
const usersPaginationEl = document.getElementById('usersPagination');
const usersPageSizeSelect = document.getElementById('usersPageSize');
const logsPageSizeSelect = document.getElementById('logsPageSize');
const summaryPaginationWrap = document.getElementById('summaryPaginationWrap');
const summaryPaginationEl = document.getElementById('summaryPagination');
const summaryPageSizeSelect = document.getElementById('summaryPageSize');
const reportDailyPaginationWrap = document.getElementById('reportDailyPaginationWrap');
const reportDailyPaginationEl = document.getElementById('reportDailyPagination');
const reportDailyPageSizeSelect = document.getElementById('reportDailyPageSize');
const reportCategoryPaginationWrap = document.getElementById('reportCategoryPaginationWrap');
const reportCategoryPaginationEl = document.getElementById('reportCategoryPagination');
const reportCategoryPageSizeSelect = document.getElementById('reportCategoryPageSize');

// 初始化今日日期
formDate.value = new Date().toISOString().slice(0, 10);
if (recordsPageSizeSelect) recordsPageSizeSelect.value = String(recordsPageSize);
if (usersPageSizeSelect) usersPageSizeSelect.value = String(usersPageSize);
if (logsPageSizeSelect) logsPageSizeSelect.value = String(logsPageSize);
if (summaryPageSizeSelect) summaryPageSizeSelect.value = String(summaryPageSize);
if (reportDailyPageSizeSelect) reportDailyPageSizeSelect.value = String(reportDailyPageSize);
if (reportCategoryPageSizeSelect) reportCategoryPageSizeSelect.value = String(reportCategoryPageSize);

async function fetchRecords(page = 1, customPageSize = recordsPageSize) {
  const params = new URLSearchParams();
  params.set('page', page);
  params.set('page_size', customPageSize);
  if (startDate.value) params.set('start_date', startDate.value);
  if (endDate.value) params.set('end_date', endDate.value);
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim());

  const res = await fetchAuth(`${API}/records?${params}`);
  if (!res.ok) throw new Error('获取列表失败');
  return res.json();
}

async function fetchAllFilteredRecords() {
  const firstPage = await fetchRecords(1, recordsPageSize);
  const total = Number(firstPage.total || 0);
  const totalPages = Math.max(1, Math.ceil(total / recordsPageSize));
  const list = [...(firstPage.data || [])];

  for (let p = 2; p <= totalPages; p += 1) {
    const next = await fetchRecords(p, recordsPageSize);
    list.push(...(next.data || []));
  }

  allFilteredRecords = list;
  return allFilteredRecords;
}

function sortRecords(list) {
  if (!Array.isArray(list) || list.length === 0) return list;
  const { field, dir } = currentSort;
  const factor = dir === 'desc' ? -1 : 1;
  return [...list].sort((a, b) => {
    if (field === 'date') {
      // YYYY-MM-DD 字符串比较即可
      return a.date === b.date ? 0 : (a.date > b.date ? factor : -factor);
    }
    if (field === 'amount') {
      const av = a.amount || 0;
      const bv = b.amount || 0;
      return av === bv ? 0 : (av > bv ? factor : -factor);
    }
    if (field === 'category') {
      const av = (a.category || '').toString();
      const bv = (b.category || '').toString();
      return av === bv ? 0 : (av > bv ? factor : -factor);
    }
    return 0;
  });
}

function renderTable(data) {
  const list = sortRecords(data.data || []);
  if (list.length === 0) {
    tableBody.innerHTML = '<tr><td colspan="5" class="empty">暂无记录</td></tr>';
    return;
  }
  tableBody.innerHTML = list.map(r => `
    <tr>
      <td>${r.date}</td>
      <td class="${r.amount >= 0 ? 'amount-income' : 'amount-expense'}">
        ${r.amount >= 0 ? '+' : ''}${r.amount.toFixed(2)}
      </td>
      <td>${r.category || '-'}</td>
      <td>${r.description || '-'}</td>
      <td>
        <div class="actions">
          <button class="btn btn-outline btn-sm btn-edit" data-id="${r.id}">编辑</button>
          <button class="btn btn-outline btn-sm btn-delete" data-id="${r.id}">删除</button>
        </div>
      </td>
    </tr>
  `).join('');

  tableBody.querySelectorAll('.btn-edit').forEach(btn => {
    btn.addEventListener('click', () => openEdit(Number(btn.dataset.id)));
  });
  tableBody.querySelectorAll('.btn-delete').forEach(btn => {
    btn.addEventListener('click', () => openDeleteConfirm(Number(btn.dataset.id)));
  });
}

function renderPager(el, total, page, pageSize, onPageChange) {
  if (!el) return;
  const totalPages = Math.ceil(total / pageSize) || 1;
  el.innerHTML = `
    <button ${page <= 1 ? 'disabled' : ''} data-page="1">首页</button>
    <button ${page <= 1 ? 'disabled' : ''} data-page="${page - 1}">上一页</button>
    <span>第 ${page} / ${totalPages} 页，共 ${total} 条</span>
    <button ${page >= totalPages ? 'disabled' : ''} data-page="${page + 1}">下一页</button>
    <button ${page >= totalPages ? 'disabled' : ''} data-page="${totalPages}">尾页</button>
    <span class="pager-jump">
      跳转
      <input type="number" min="1" max="${totalPages}" value="${page}" aria-label="跳转页码" />
      页
      <button data-jump="1">确定</button>
    </span>
  `;
  el.querySelectorAll('button[data-page]').forEach(btn => {
    if (!btn.disabled) btn.addEventListener('click', () => onPageChange(Number(btn.dataset.page)));
  });
  const jumpBtn = el.querySelector('button[data-jump="1"]');
  const jumpInput = el.querySelector('.pager-jump input');
  const doJump = () => {
    if (!jumpInput) return;
    const val = Number(jumpInput.value);
    if (!Number.isFinite(val)) return;
    const target = Math.min(Math.max(Math.trunc(val), 1), totalPages);
    onPageChange(target);
  };
  jumpBtn && jumpBtn.addEventListener('click', doJump);
  jumpInput && jumpInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') doJump();
  });
}

function renderPagination(total, page, pageSize, onPageChange = loadPage) {
  renderPager(paginationEl, total, page, pageSize, onPageChange);
}

async function loadPage(page = 1) {
  tableBody.innerHTML = '<tr><td colspan="5" class="loading">加载中...</td></tr>';
  try {
    isAllRecordsSortMode = false;
    const data = await fetchRecords(page);
    currentRecords = data.data || [];
    currentPage = page;
    renderTable(data);
    renderPagination(data.total, data.page, recordsPageSize);
  } catch (e) {
    tableBody.innerHTML = `<tr><td colspan="5" class="empty">加载失败: ${e.message}</td></tr>`;
  }
}

function openAdd() {
  modalTitle.textContent = '添加记录';
  recordId.value = '';
  recordForm.reset();
  formDate.value = new Date().toISOString().slice(0, 10);
  formAmount.value = '';
  syncDateDisplay('formDate');
  modal.classList.add('show');
}

async function openEdit(id) {
  try {
    const res = await fetchAuth(`${API}/records/${id}`);
    if (!res.ok) throw new Error('获取失败');
    const r = await res.json();
    recordId.value = r.id;
    formDate.value = r.date;
    syncDateDisplay('formDate');
    formAmount.value = r.amount != null ? r.amount.toFixed(2) : '';
    formCategory.value = r.category || '';
    formDescription.value = r.description || '';
    modalTitle.textContent = '编辑记录';
    modal.classList.add('show');
  } catch (e) {
    alert('加载记录失败: ' + e.message);
  }
}

function closeModal() {
  modal.classList.remove('show');
}

function openDeleteConfirm(id) {
  deleteTargetId = id;
  deleteModal.classList.add('show');
}

function closeDeleteModal() {
  deleteTargetId = null;
  deleteModal.classList.remove('show');
}

async function doDelete() {
  if (!deleteTargetId) return;
  try {
    const res = await fetchAuth(`${API}/records/${deleteTargetId}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('删除失败');
    closeDeleteModal();
    loadPage(currentPage);
  } catch (e) {
    alert('删除失败: ' + e.message);
  }
}

recordForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const id = recordId.value;
  const payload = {
    date: formDate.value,
    amount: parseFloat(formAmount.value),
    category: formCategory.value.trim(),
    description: formDescription.value.trim(),
  };

  try {
    const url = id ? `${API}/records/${id}` : `${API}/records`;
    const method = id ? 'PUT' : 'POST';
    const body = id ? payload : payload;
    const res = await fetchAuth(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || '保存失败');
    }
    closeModal();
    loadPage(id ? currentPage : 1);
  } catch (e) {
    alert('保存失败: ' + e.message);
  }
});

document.getElementById('btnAdd').addEventListener('click', openAdd);
document.getElementById('btnLogout').addEventListener('click', logout);

// 账户设置
let totpSetupSecret = '';
async function openTOTPModal() {
  const totpModal = document.getElementById('totpModal');
  try {
    const meRes = await fetchAuth(`${API}/auth/me`);
    const me = await meRes.json();
    if (me.totp_enabled) {
      document.getElementById('totpEnableArea').style.display = 'none';
      document.getElementById('totpDisableArea').style.display = 'block';
    } else {
      document.getElementById('totpEnableArea').style.display = 'block';
      document.getElementById('totpDisableArea').style.display = 'none';
      const setupRes = await fetchAuth(`${API}/auth/totp/setup`);
      const setup = await setupRes.json();
      totpSetupSecret = setup.secret;
      document.getElementById('totpSecretText').textContent = '密钥: ' + setup.secret;
      document.getElementById('totpQR').innerHTML = '<img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=' + encodeURIComponent(setup.url) + '" alt="QR" />';
      document.getElementById('totpEnableCode').value = '';
    }
    document.getElementById('settingsModal').classList.remove('show');
    totpModal.classList.add('show');
  } catch (e) {
    alert(e.message);
  }
}

let currentUserRole = 'user';
document.getElementById('btnSettings').addEventListener('click', async () => {
  document.getElementById('changePwdOld').value = '';
  document.getElementById('changePwdNew').value = '';
  document.getElementById('settingsAddUserSection').style.display = currentUserRole === 'admin' ? 'block' : 'none';
  document.getElementById('settingsModal').classList.add('show');
});

document.getElementById('settingsModalClose').addEventListener('click', () => {
  document.getElementById('settingsModal').classList.remove('show');
});

document.getElementById('btnOpenTOTP').addEventListener('click', openTOTPModal);

document.getElementById('btnChangePassword').addEventListener('click', async () => {
  const oldPwd = document.getElementById('changePwdOld').value;
  const newPwd = document.getElementById('changePwdNew').value;
  if (!oldPwd || !newPwd) { alert('请填写当前密码和新密码'); return; }
  if (newPwd.length < 6) { alert('新密码至少6位'); return; }
  try {
    const res = await fetchAuth(`${API}/auth/change-password`, {
      method: 'POST',
      body: JSON.stringify({ old_password: oldPwd, new_password: newPwd }),
    });
    if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
    document.getElementById('settingsModal').classList.remove('show');
    alert('密码已修改');
  } catch (e) {
    alert(e.message);
  }
});

// 账户设置：根据角色显示添加用户提示（管理员在用户管理操作）

document.getElementById('totpModalClose').addEventListener('click', () => {
  document.getElementById('totpModal').classList.remove('show');
});

document.getElementById('btnTOTPEnable').addEventListener('click', async () => {
  const code = document.getElementById('totpEnableCode').value.trim();
  if (!code) { alert('请输入验证码'); return; }
  try {
    const res = await fetchAuth(`${API}/auth/totp/enable`, {
      method: 'POST',
      body: JSON.stringify({ secret: totpSetupSecret, code }),
    });
    if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
    document.getElementById('totpModal').classList.remove('show');
    alert('TOTP 已启用');
  } catch (e) {
    alert(e.message);
  }
});

document.getElementById('totpModal').addEventListener('click', (e) => {
  if (e.target.id === 'totpModal') {
    document.getElementById('totpModal').classList.remove('show');
  }
});

document.getElementById('btnTOTPDisable').addEventListener('click', async () => {
  const password = document.getElementById('totpDisablePassword').value;
  const code = document.getElementById('totpDisableCode').value.trim();
  if (!password || !code) { alert('请输入密码和 TOTP 验证码'); return; }
  try {
    const res = await fetchAuth(`${API}/auth/totp/disable`, {
      method: 'POST',
      body: JSON.stringify({ password, code }),
    });
    if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
    document.getElementById('totpModal').classList.remove('show');
    alert('TOTP 已关闭');
  } catch (e) {
    alert(e.message);
  }
});
document.getElementById('modalClose').addEventListener('click', closeModal);
document.getElementById('btnCancel').addEventListener('click', closeModal);
document.getElementById('btnSearch').addEventListener('click', () => loadPage(1));
document.getElementById('btnReset').addEventListener('click', () => {
  startDate.value = '';
  endDate.value = '';
  syncDateDisplay('startDate');
  syncDateDisplay('endDate');
  keyword.value = '';
  loadPage(1);
});
if (recordsPageSizeSelect) {
  recordsPageSizeSelect.addEventListener('change', () => {
    recordsPageSize = Number(recordsPageSizeSelect.value) || 20;
    isAllRecordsSortMode = false;
    allFilteredRecords = [];
    loadPage(1);
  });
}
if (usersPageSizeSelect) {
  usersPageSizeSelect.addEventListener('change', () => {
    usersPageSize = Number(usersPageSizeSelect.value) || 20;
    loadUsers(1);
  });
}
if (logsPageSizeSelect) {
  logsPageSizeSelect.addEventListener('change', () => {
    logsPageSize = Number(logsPageSizeSelect.value) || 20;
    isAllLogsSortMode = false;
    allLogs = [];
    loadLogs(1);
  });
}
if (summaryPageSizeSelect) {
  summaryPageSizeSelect.addEventListener('change', () => {
    summaryPageSize = Number(summaryPageSizeSelect.value) || 20;
    currentSummaryPage = 1;
    renderSummaryDetail();
  });
}
if (reportDailyPageSizeSelect) {
  reportDailyPageSizeSelect.addEventListener('change', () => {
    reportDailyPageSize = Number(reportDailyPageSizeSelect.value) || 20;
    currentReportDailyPage = 1;
    if (currentReportData) renderReport(currentReportData);
  });
}
if (reportCategoryPageSizeSelect) {
  reportCategoryPageSizeSelect.addEventListener('change', () => {
    reportCategoryPageSize = Number(reportCategoryPageSizeSelect.value) || 20;
    currentReportCategoryPage = 1;
    if (currentReportData) renderReport(currentReportData);
  });
}

document.getElementById('btnDeleteCancel').addEventListener('click', closeDeleteModal);
document.getElementById('btnDeleteConfirm').addEventListener('click', doDelete);

deleteModal.addEventListener('click', (e) => {
  if (e.target === deleteModal) closeDeleteModal();
});
document.getElementById('settingsModal').addEventListener('click', (e) => {
  if (e.target.id === 'settingsModal') e.target.classList.remove('show');
});

function renderSortedPage(page = 1) {
  const sorted = sortRecords(allFilteredRecords);
  const total = sorted.length;
  const totalPages = Math.max(1, Math.ceil(total / recordsPageSize));
  const safePage = Math.min(Math.max(page, 1), totalPages);
  const start = (safePage - 1) * recordsPageSize;
  const pageList = sorted.slice(start, start + recordsPageSize);

  currentPage = safePage;
  currentRecords = pageList;
  renderTable({ data: pageList });
  renderPagination(total, safePage, recordsPageSize, renderSortedPage);
}

async function changeSort(field) {
  if (!field) return;
  if (currentSort.field === field) {
    currentSort.dir = currentSort.dir === 'desc' ? 'asc' : 'desc';
  } else {
    currentSort = { field, dir: 'desc' };
  }

  tableBody.innerHTML = '<tr><td colspan="5" class="loading">排序中...</td></tr>';
  try {
    if (!isAllRecordsSortMode || allFilteredRecords.length === 0) {
      await fetchAllFilteredRecords();
    }
    isAllRecordsSortMode = true;
    renderSortedPage(1);
    refreshRecordsSortIndicators();
  } catch (e) {
    tableBody.innerHTML = `<tr><td colspan="5" class="empty">排序失败: ${e.message}</td></tr>`;
  }
}

if (thDate) thDate.addEventListener('click', () => changeSort('date'));
if (thAmount) thAmount.addEventListener('click', () => changeSort('amount'));
if (thCategory) thCategory.addEventListener('click', () => changeSort('category'));
refreshRecordsSortIndicators();

// =====  Tab 切换 =====
const sectionRecords = document.getElementById('sectionRecords');
const sectionSummary = document.getElementById('sectionSummary');
const sectionReport = document.getElementById('sectionReport');
const sectionUsers = document.getElementById('sectionUsers');
const tabs = document.querySelectorAll('.tab');
const tabUsers = document.getElementById('tabUsers');

let currentUserName = '';
async function initUserRole() {
  try {
    const res = await fetchAuth(`${API}/auth/me`);
    const me = await res.json();
    currentUserRole = me.role || 'user';
    currentUserName = me.username || '';
    document.getElementById('currentUserName').textContent = currentUserName || '未知';
    if (currentUserRole === 'admin') {
      tabUsers.style.display = 'block';
      const tabLogs = document.getElementById('tabLogs');
      if (tabLogs) tabLogs.style.display = 'block';
    }
  } catch (e) {}
}

const sectionLogs = document.getElementById('sectionLogs');

function showTab(name) {
  sectionRecords.style.display = name === 'records' ? 'block' : 'none';
  sectionSummary.style.display = name === 'summary' ? 'block' : 'none';
  sectionReport.style.display = name === 'report' ? 'block' : 'none';
  sectionUsers.style.display = name === 'users' ? 'block' : 'none';
  if (sectionLogs) sectionLogs.style.display = name === 'logs' ? 'block' : 'none';
  if (btnAdd) btnAdd.style.display = name === 'records' ? '' : 'none';
  tabs.forEach(t => t.classList.toggle('active', t.dataset.tab === name));
  if (name === 'users') loadUsers();
  if (name === 'logs') loadLogs();
}

tabs.forEach(t => {
  t.addEventListener('click', () => showTab(t.dataset.tab));
});

// ===== 用户管理（管理员） =====
let userModalMode = 'add';
let editingUserId = null;
let deleteUserId = null;

async function loadUsers(page = 1) {
  const tbody = document.getElementById('usersTableBody');
  try {
    const res = await fetchAuth(`${API}/auth/users`);
    if (!res.ok) throw new Error('无权限');
    const data = await res.json();
    if (!data.data || data.data.length === 0) {
      currentUsers = [];
      tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无用户</td></tr>';
      renderPager(usersPaginationEl, 0, 1, usersPageSize, loadUsers);
      return;
    }
    currentUsers = data.data;
    renderUsersTable(undefined, page);
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">${e.message}</td></tr>`;
  }
}

function renderUsersTable(sortField, page = currentUsersPage) {
  const tbody = document.getElementById('usersTableBody');
  if (!tbody) return;
  if (sortField === 'id') {
    currentUsers = sortWithToggle(currentUsers, 'id', 'number', usersSort);
  } else if (sortField === 'username') {
    currentUsers = sortWithToggle(currentUsers, 'username', 'string', usersSort);
  } else if (sortField === 'role') {
    currentUsers = sortWithToggle(currentUsers, 'role', 'string', usersSort);
  } else if (sortField === 'created_at') {
    currentUsers = sortWithToggle(currentUsers, 'created_at', 'string', usersSort);
  }
  const total = currentUsers.length;
  const totalPages = Math.max(1, Math.ceil(total / usersPageSize));
  const safePage = Math.min(Math.max(page, 1), totalPages);
  const start = (safePage - 1) * usersPageSize;
  const list = currentUsers.slice(start, start + usersPageSize);
  currentUsersPage = safePage;

  tbody.innerHTML = list.map(u => `
      <tr>
        <td>${u.id}</td>
        <td>${u.username}</td>
        <td>${u.role === 'admin' ? '管理员' : '普通用户'}</td>
        <td>${u.created_at ? u.created_at.slice(0, 10) : '-'}</td>
        <td>
          <div class="actions">
            <button class="btn btn-outline btn-sm btn-edit-user" data-id="${u.id}">编辑</button>
            <button class="btn btn-outline btn-sm btn-chpwd-user" data-id="${u.id}">改密</button>
            <button class="btn btn-outline btn-sm btn-del-user" data-id="${u.id}">删除</button>
          </div>
        </td>
      </tr>
    `).join('');
  renderPager(usersPaginationEl, total, safePage, usersPageSize, loadUsers);
  refreshUsersSortIndicators();
  tbody.querySelectorAll('.btn-edit-user').forEach(btn => {
    btn.addEventListener('click', () => openEditUser(Number(btn.dataset.id)));
  });
  tbody.querySelectorAll('.btn-chpwd-user').forEach(btn => {
    btn.addEventListener('click', () => openChgPwdUser(Number(btn.dataset.id)));
  });
  tbody.querySelectorAll('.btn-del-user').forEach(btn => {
    btn.addEventListener('click', () => openDeleteUser(Number(btn.dataset.id)));
  });
}

function openAddUser() {
  userModalMode = 'add';
  editingUserId = null;
  document.getElementById('userModalTitle').textContent = '添加用户';
  document.getElementById('userFormUsername').value = '';
  document.getElementById('userFormUsername').disabled = false;
  document.getElementById('userFormPasswordGroup').style.display = 'block';
  document.getElementById('userFormPassword').value = '';
  document.getElementById('userFormRoleGroup').style.display = 'block';
  document.getElementById('userFormRole').value = 'user';
  document.getElementById('userFormNewPasswordGroup').style.display = 'none';
  document.getElementById('userModal').classList.add('show');
}

async function openEditUser(id) {
  userModalMode = 'edit';
  editingUserId = id;
  try {
    const res = await fetchAuth(`${API}/auth/users/${id}`);
    if (!res.ok) throw new Error('获取失败');
    const u = await res.json();
    document.getElementById('userModalTitle').textContent = '编辑用户';
    document.getElementById('userFormUsername').value = u.username;
    document.getElementById('userFormUsername').disabled = false;
    document.getElementById('userFormPasswordGroup').style.display = 'none';
    document.getElementById('userFormRoleGroup').style.display = 'block';
    document.getElementById('userFormRole').value = u.role || 'user';
    document.getElementById('userFormNewPasswordGroup').style.display = 'none';
    document.getElementById('userModal').classList.add('show');
  } catch (e) {
    alert(e.message);
  }
}

async function openChgPwdUser(id) {
  userModalMode = 'chpwd';
  editingUserId = id;
  document.getElementById('userModalTitle').textContent = '修改密码';
  document.getElementById('userFormUsername').value = '';
  document.getElementById('userFormUsername').disabled = true;
  document.getElementById('userFormPasswordGroup').style.display = 'none';
  document.getElementById('userFormRoleGroup').style.display = 'none';
  document.getElementById('userFormNewPasswordGroup').style.display = 'block';
  document.getElementById('userFormNewPassword').value = '';
  document.getElementById('userModal').classList.add('show');
}

function openDeleteUser(id) {
  deleteUserId = id;
  document.getElementById('userDeleteModal').classList.add('show');
}

document.getElementById('btnAddUserModal').addEventListener('click', openAddUser);
document.getElementById('userModalClose').addEventListener('click', () => {
  document.getElementById('userModal').classList.remove('show');
});
document.getElementById('btnUserFormSubmit').addEventListener('click', async () => {
  if (userModalMode === 'add') {
    const username = document.getElementById('userFormUsername').value.trim();
    const password = document.getElementById('userFormPassword').value;
    const role = document.getElementById('userFormRole').value;
    if (!username || !password) { alert('请填写用户名和密码'); return; }
    if (password.length < 6) { alert('密码至少6位'); return; }
    try {
      const res = await fetchAuth(`${API}/auth/users`, {
        method: 'POST',
        body: JSON.stringify({ username, password, role }),
      });
      if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
      document.getElementById('userModal').classList.remove('show');
      loadUsers();
      alert('用户已添加');
    } catch (e) { alert(e.message); }
  } else if (userModalMode === 'edit') {
    const username = document.getElementById('userFormUsername').value.trim();
    const role = document.getElementById('userFormRole').value;
    if (!username) { alert('请填写用户名'); return; }
    try {
      const res = await fetchAuth(`${API}/auth/users/${editingUserId}`, {
        method: 'PUT',
        body: JSON.stringify({ username, role }),
      });
      if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
      document.getElementById('userModal').classList.remove('show');
      loadUsers();
      alert('已更新');
    } catch (e) { alert(e.message); }
  } else if (userModalMode === 'chpwd') {
    const password = document.getElementById('userFormNewPassword').value;
    if (!password || password.length < 6) { alert('新密码至少6位'); return; }
    try {
      const res = await fetchAuth(`${API}/auth/users/${editingUserId}/change-password`, {
        method: 'POST',
        body: JSON.stringify({ password }),
      });
      if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
      document.getElementById('userModal').classList.remove('show');
      alert('密码已修改');
    } catch (e) { alert(e.message); }
  }
});

document.getElementById('btnUserDeleteCancel').addEventListener('click', () => {
  deleteUserId = null;
  document.getElementById('userDeleteModal').classList.remove('show');
});
document.getElementById('btnUserDeleteConfirm').addEventListener('click', async () => {
  if (!deleteUserId) return;
  try {
    const res = await fetchAuth(`${API}/auth/users/${deleteUserId}`, { method: 'DELETE' });
    if (!res.ok) { const e = await res.json(); throw new Error(e.error); }
    document.getElementById('userDeleteModal').classList.remove('show');
    deleteUserId = null;
    loadUsers();
    alert('已删除');
  } catch (e) { alert(e.message); }
});

document.getElementById('userModal').addEventListener('click', (e) => {
  if (e.target.id === 'userModal') document.getElementById('userModal').classList.remove('show');
});
document.getElementById('userDeleteModal').addEventListener('click', (e) => {
  if (e.target.id === 'userDeleteModal') document.getElementById('userDeleteModal').classList.remove('show');
});

// 用户表头排序
const usersThId = document.getElementById('usersThId');
const usersThUsername = document.getElementById('usersThUsername');
const usersThRole = document.getElementById('usersThRole');
const usersThCreatedAt = document.getElementById('usersThCreatedAt');
usersThId && usersThId.addEventListener('click', () => renderUsersTable('id'));
usersThUsername && usersThUsername.addEventListener('click', () => renderUsersTable('username'));
usersThRole && usersThRole.addEventListener('click', () => renderUsersTable('role'));
usersThCreatedAt && usersThCreatedAt.addEventListener('click', () => renderUsersTable('created_at'));

// ===== 操作日志（管理员） =====

function renderLogsPagination(total, page, onPageChange = loadLogs) {
  renderPager(document.getElementById('logsPagination'), total, page, logsPageSize, onPageChange);
}

async function fetchLogsPage(page = 1) {
  const params = new URLSearchParams();
  params.set('page', page);
  params.set('page_size', logsPageSize);
  const action = document.getElementById('logActionFilter')?.value;
  if (action) params.set('action', action);
  const res = await fetchAuth(`${API}/auth/operation-logs?${params}`);
  if (!res.ok) throw new Error('无权限');
  return res.json();
}

async function fetchAllLogs() {
  const first = await fetchLogsPage(1);
  const total = Number(first.total || 0);
  const totalPages = Math.max(1, Math.ceil(total / logsPageSize));
  const list = [...(first.data || [])];
  for (let p = 2; p <= totalPages; p += 1) {
    const next = await fetchLogsPage(p);
    list.push(...(next.data || []));
  }
  allLogs = list;
  currentLogsTotal = total;
  return allLogs;
}

async function loadLogs(page = 1) {
  const tbody = document.getElementById('logsTableBody');
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="5" class="loading">加载中...</td></tr>';
  try {
    isAllLogsSortMode = false;
    const data = await fetchLogsPage(page);
    currentLogsPage = page;
    currentLogsTotal = Number(data.total || 0);
    if (!data.data || data.data.length === 0) {
      currentLogs = [];
      tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无日志</td></tr>';
      renderLogsPagination(0, 1);
      return;
    }
    currentLogs = data.data;
    renderLogsTable(currentLogs);
    renderLogsPagination(currentLogsTotal, page);
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">${e.message}</td></tr>`;
  }
}

function renderLogsTable(list = []) {
  const tbody = document.getElementById('logsTableBody');
  if (!tbody) return;
  tbody.innerHTML = list.map(l => `
      <tr>
        <td>${l.created_at ? l.created_at.slice(0, 19).replace('T', ' ') : '-'}</td>
        <td>${l.username}</td>
        <td>${l.action}</td>
        <td>${l.detail || '-'}</td>
        <td>${l.ip || '-'}</td>
      </tr>
    `).join('');
  refreshLogsSortIndicators();
}

function sortLogsList(list) {
  if (!Array.isArray(list) || list.length === 0 || !logsSort.field) return [...(list || [])];
  const cmp = logsSort.dir === 'desc' ? sortDesc : sortAsc;
  return [...list].sort((a, b) => cmp(a, b, logsSort.field, 'string'));
}

function renderSortedLogsPage(page = 1) {
  const sorted = sortLogsList(allLogs);
  const total = sorted.length;
  const totalPages = Math.max(1, Math.ceil(total / logsPageSize));
  const safePage = Math.min(Math.max(page, 1), totalPages);
  const start = (safePage - 1) * logsPageSize;
  const pageList = sorted.slice(start, start + logsPageSize);
  currentLogsPage = safePage;
  currentLogs = pageList;
  renderLogsTable(pageList);
  renderLogsPagination(total, safePage, renderSortedLogsPage);
}

async function changeLogsSort(field) {
  if (!field) return;
  if (logsSort.field === field) {
    logsSort.dir = logsSort.dir === 'desc' ? 'asc' : 'desc';
  } else {
    logsSort.field = field;
    logsSort.dir = 'desc';
  }
  const tbody = document.getElementById('logsTableBody');
  if (tbody) tbody.innerHTML = '<tr><td colspan="5" class="loading">排序中...</td></tr>';
  try {
    if (!isAllLogsSortMode || allLogs.length === 0) {
      await fetchAllLogs();
    }
    isAllLogsSortMode = true;
    const sorted = [...allLogs].sort((a, b) => (logsSort.dir === 'desc' ? sortDesc : sortAsc)(a, b, field, 'string'));
    allLogs = sorted;
    renderSortedLogsPage(1);
  } catch (e) {
    if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="empty">${e.message}</td></tr>`;
  }
}

document.getElementById('btnLoadLogs')?.addEventListener('click', () => loadLogs(1));

// 日志表头排序
const logsThTime = document.getElementById('logsThTime');
const logsThUser = document.getElementById('logsThUser');
const logsThAction = document.getElementById('logsThAction');
logsThTime && logsThTime.addEventListener('click', () => changeLogsSort('created_at'));
logsThUser && logsThUser.addEventListener('click', () => changeLogsSort('username'));
logsThAction && logsThAction.addEventListener('click', () => changeLogsSort('action'));

// ===== 汇总 =====
const summaryType = document.getElementById('summaryType');
const summaryDailyParams = document.getElementById('summaryDailyParams');
const summaryMonthlyParams = document.getElementById('summaryMonthlyParams');
const summaryYearlyParams = document.getElementById('summaryYearlyParams');
const summaryDate = document.getElementById('summaryDate');
const summaryYear = document.getElementById('summaryYear');
const summaryMonth = document.getElementById('summaryMonth');
const summaryYearOnly = document.getElementById('summaryYearOnly');
const summaryCards = document.getElementById('summaryCards');
const summaryDetail = document.getElementById('summaryDetail');

summaryType.addEventListener('change', () => {
  const v = summaryType.value;
  summaryDailyParams.style.display = v === 'daily' ? 'block' : 'none';
  summaryMonthlyParams.style.display = v === 'monthly' ? 'flex' : 'none';
  summaryYearlyParams.style.display = v === 'yearly' ? 'flex' : 'none';
});

const today = new Date().toISOString().slice(0, 10);
summaryDate.value = today;
summaryYear.value = summaryYearOnly.value = today.slice(0, 4);
summaryMonth.value = today.slice(5, 7);

async function loadSummary() {
  const type = summaryType.value;
  let url = '';
  if (type === 'daily') {
    const d = summaryDate.value;
    if (!d) { alert('请选择日期'); return; }
    url = `${API}/summary/daily?date=${d}`;
  } else if (type === 'monthly') {
    const y = summaryYear.value, m = summaryMonth.value;
    if (!y || !m) { alert('请填写年份和月份'); return; }
    url = `${API}/summary/monthly?year=${y}&month=${m}`;
  } else {
    const y = summaryYearOnly.value;
    if (!y) { alert('请填写年份'); return; }
    url = `${API}/summary/yearly?year=${y}`;
  }
  summaryCards.innerHTML = '<div class="loading">加载中...</div>';
  summaryDetail.innerHTML = '';
  try {
    const res = await fetchAuth(url);
    if (!res.ok) throw new Error('获取失败');
    const data = await res.json();
    currentSummaryData = data;
    renderSummaryCards(data);
    currentSummaryMode = Array.isArray(data.records) && data.records.length > 0 ? 'records' : (Array.isArray(data.breakdown) && data.breakdown.length > 0 ? 'breakdown' : null);
    currentSummaryList = currentSummaryMode === 'records' ? [...(data.records || [])] : [...(data.breakdown || [])];
    currentSummaryPage = 1;
    renderSummaryDetail();
  } catch (e) {
    summaryCards.innerHTML = `<div class="empty">加载失败: ${e.message}</div>`;
  }
}

function renderSummaryCards(data) {
  summaryCards.innerHTML = `
    <div class="summary-card income">
      <div class="label">收入</div>
      <div class="value">+${(data.income || 0).toFixed(2)}</div>
    </div>
    <div class="summary-card expense">
      <div class="label">支出</div>
      <div class="value">-${(data.expense || 0).toFixed(2)}</div>
    </div>
    <div class="summary-card balance">
      <div class="label">结余</div>
      <div class="value">${(data.balance || 0).toFixed(2)}</div>
    </div>
    <div class="summary-card">
      <div class="label">笔数</div>
      <div class="value">${data.count || 0}</div>
    </div>
  `;
}

function renderSummaryDetail() {
  if (!Array.isArray(currentSummaryList) || currentSummaryList.length === 0) {
    summaryDetail.innerHTML = '<h3>明细</h3><div class="empty">暂无数据</div>';
    if (summaryPaginationWrap) summaryPaginationWrap.style.display = 'none';
    return;
  }

  const total = currentSummaryList.length;
  const totalPages = Math.max(1, Math.ceil(total / summaryPageSize));
  const safePage = Math.min(Math.max(currentSummaryPage, 1), totalPages);
  const start = (safePage - 1) * summaryPageSize;
  const list = currentSummaryList.slice(start, start + summaryPageSize);
  currentSummaryPage = safePage;

  if (currentSummaryMode === 'records') {
    summaryDetail.innerHTML = `
      <h3>明细</h3>
      <table class="table">
        <thead>
          <tr>
            <th id="summaryThDate">日期</th>
            <th id="summaryThAmount">金额</th>
            <th id="summaryThCategory">分类</th>
            <th>描述</th>
          </tr>
        </thead>
        <tbody>
          ${list.map(r => `
            <tr>
              <td>${r.date}</td>
              <td class="${r.amount >= 0 ? 'amount-income' : 'amount-expense'}">
                ${r.amount >= 0 ? '+' : ''}${r.amount.toFixed(2)}
              </td>
              <td>${r.category || '-'}</td>
              <td>${r.description || '-'}</td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    `;
  } else {
    summaryDetail.innerHTML = `
      <h3>分项</h3>
      <table class="table">
        <thead>
          <tr>
            <th id="summaryThPeriod">日期/月份</th>
            <th id="summaryThIncome">收入</th>
            <th id="summaryThExpense">支出</th>
            <th id="summaryThBalance">结余</th>
            <th id="summaryThCount">笔数</th>
          </tr>
        </thead>
        <tbody>
          ${list.map(b => `
            <tr>
              <td>${b.period}</td>
              <td class="amount-income">+${b.income.toFixed(2)}</td>
              <td class="amount-expense">-${b.expense.toFixed(2)}</td>
              <td>${b.balance.toFixed(2)}</td>
              <td>${b.count}</td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    `;
  }

  if (summaryPaginationWrap) summaryPaginationWrap.style.display = 'flex';
  renderPager(summaryPaginationEl, total, safePage, summaryPageSize, (page) => {
    currentSummaryPage = page;
    renderSummaryDetail();
  });
  refreshSummarySortIndicators();
  attachSummarySortHandlers();
}

function sortDesc(a, b, key, type) {
  if (type === 'number') {
    const av = Number(a[key] || 0);
    const bv = Number(b[key] || 0);
    return bv - av;
  }
  const av = (a[key] || '').toString();
  const bv = (b[key] || '').toString();
  return bv > av ? 1 : (bv < av ? -1 : 0);
}

function sortAsc(a, b, key, type) {
  if (type === 'number') {
    const av = Number(a[key] || 0);
    const bv = Number(b[key] || 0);
    return av - bv;
  }
  const av = (a[key] || '').toString();
  const bv = (b[key] || '').toString();
  return av > bv ? 1 : (av < bv ? -1 : 0);
}

function sortWithToggle(list, key, type, state) {
  if (!Array.isArray(list)) return [];
  if (state.field === key) {
    state.dir = state.dir === 'desc' ? 'asc' : 'desc';
  } else {
    state.field = key;
    state.dir = 'desc';
  }
  const cmp = state.dir === 'desc' ? sortDesc : sortAsc;
  return [...list].sort((a, b) => cmp(a, b, key, type));
}

function getSortArrow(state, field) {
  if (!state || state.field !== field) return '';
  return state.dir === 'asc' ? ' ↑' : ' ↓';
}

function setHeaderSortIndicators(headers, state) {
  headers.forEach(h => {
    const el = document.getElementById(h.id);
    if (!el) return;
    el.textContent = `${h.label}${getSortArrow(state, h.field)}`;
  });
}

function refreshRecordsSortIndicators() {
  setHeaderSortIndicators([
    { id: 'thDate', label: '日期', field: 'date' },
    { id: 'thAmount', label: '金额', field: 'amount' },
    { id: 'thCategory', label: '分类', field: 'category' },
  ], currentSort);
}

function refreshUsersSortIndicators() {
  setHeaderSortIndicators([
    { id: 'usersThId', label: 'ID', field: 'id' },
    { id: 'usersThUsername', label: '用户名', field: 'username' },
    { id: 'usersThRole', label: '角色', field: 'role' },
    { id: 'usersThCreatedAt', label: '创建时间', field: 'created_at' },
  ], usersSort);
}

function refreshLogsSortIndicators() {
  setHeaderSortIndicators([
    { id: 'logsThTime', label: '时间', field: 'created_at' },
    { id: 'logsThUser', label: '用户', field: 'username' },
    { id: 'logsThAction', label: '操作', field: 'action' },
  ], logsSort);
}

function refreshSummarySortIndicators() {
  if (currentSummaryMode === 'records') {
    setHeaderSortIndicators([
      { id: 'summaryThDate', label: '日期', field: 'date' },
      { id: 'summaryThAmount', label: '金额', field: 'amount' },
      { id: 'summaryThCategory', label: '分类', field: 'category' },
    ], summarySort);
  } else if (currentSummaryMode === 'breakdown') {
    setHeaderSortIndicators([
      { id: 'summaryThPeriod', label: '日期/月份', field: 'period' },
      { id: 'summaryThIncome', label: '收入', field: 'income' },
      { id: 'summaryThExpense', label: '支出', field: 'expense' },
      { id: 'summaryThBalance', label: '结余', field: 'balance' },
      { id: 'summaryThCount', label: '笔数', field: 'count' },
    ], summaryBreakdownSort);
  }
}

function refreshReportSortIndicators() {
  setHeaderSortIndicators([
    { id: 'reportThPeriod', label: '日期', field: 'period' },
    { id: 'reportThIncome', label: '收入', field: 'income' },
    { id: 'reportThExpense', label: '支出', field: 'expense' },
    { id: 'reportThBalance', label: '结余', field: 'balance' },
    { id: 'reportThCount', label: '笔数', field: 'count' },
  ], reportDailySort);

  setHeaderSortIndicators([
    { id: 'reportThCat', label: '分类', field: 'category' },
    { id: 'reportThCatIncome', label: '收入', field: 'income' },
    { id: 'reportThCatExpense', label: '支出', field: 'expense' },
    { id: 'reportThCatTotal', label: '合计', field: 'total' },
    { id: 'reportThCatCount', label: '笔数', field: 'count' },
  ], reportCatSort);
}

function attachSummarySortHandlers() {
  if (!currentSummaryMode || !Array.isArray(currentSummaryList) || currentSummaryList.length === 0) return;
  if (currentSummaryMode === 'records') {
    const thDate = document.getElementById('summaryThDate');
    const thAmount = document.getElementById('summaryThAmount');
    const thCategory = document.getElementById('summaryThCategory');
    thDate && (thDate.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'date', 'string', summarySort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thAmount && (thAmount.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'amount', 'number', summarySort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thCategory && (thCategory.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'category', 'string', summarySort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
  } else {
    const thPeriod = document.getElementById('summaryThPeriod');
    const thIncome = document.getElementById('summaryThIncome');
    const thExpense = document.getElementById('summaryThExpense');
    const thBalance = document.getElementById('summaryThBalance');
    const thCount = document.getElementById('summaryThCount');
    thPeriod && (thPeriod.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'period', 'string', summaryBreakdownSort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thIncome && (thIncome.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'income', 'number', summaryBreakdownSort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thExpense && (thExpense.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'expense', 'number', summaryBreakdownSort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thBalance && (thBalance.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'balance', 'number', summaryBreakdownSort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
    thCount && (thCount.onclick = () => {
      currentSummaryList = sortWithToggle(currentSummaryList, 'count', 'number', summaryBreakdownSort);
      currentSummaryPage = 1;
      renderSummaryDetail();
    });
  }
}

document.getElementById('btnSummary').addEventListener('click', loadSummary);

// ===== 报表 =====
const reportStartDate = document.getElementById('reportStartDate');
const reportEndDate = document.getElementById('reportEndDate');
const reportContent = document.getElementById('reportContent');

function setDefaultReportRange() {
  const now = new Date();
  const firstDay = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10);
  reportStartDate.value = firstDay;
  reportEndDate.value = today;
}
setDefaultReportRange();

async function loadReport() {
  const start = reportStartDate.value, end = reportEndDate.value;
  if (!start || !end) { alert('请选择日期范围'); return; }
  if (start > end) { alert('起始日期不能大于结束日期'); return; }
  reportContent.innerHTML = '<div class="loading">生成报表中...</div>';
  try {
    const res = await fetchAuth(`${API}/report?start_date=${start}&end_date=${end}`);
    if (!res.ok) throw new Error('生成失败');
    const data = await res.json();
    currentReportData = data;
    currentReportDaily = [...(data.daily || [])];
    currentReportCategory = [...(data.by_category || [])];
    currentReportDailyPage = 1;
    currentReportCategoryPage = 1;
    renderReport(data);
  } catch (e) {
    reportContent.innerHTML = `<div class="empty">${e.message}</div>`;
  }
}

function renderReport(data) {
  const dailyTotal = currentReportDaily.length;
  const dailyPages = Math.max(1, Math.ceil(dailyTotal / reportDailyPageSize));
  currentReportDailyPage = Math.min(Math.max(currentReportDailyPage, 1), dailyPages);
  const dailyStart = (currentReportDailyPage - 1) * reportDailyPageSize;
  const dailyList = currentReportDaily.slice(dailyStart, dailyStart + reportDailyPageSize);

  const catTotal = currentReportCategory.length;
  const catPages = Math.max(1, Math.ceil(catTotal / reportCategoryPageSize));
  currentReportCategoryPage = Math.min(Math.max(currentReportCategoryPage, 1), catPages);
  const catStart = (currentReportCategoryPage - 1) * reportCategoryPageSize;
  const catList = currentReportCategory.slice(catStart, catStart + reportCategoryPageSize);

  let html = `
    <div class="report-overview">
      <div class="stat"><div class="label">收入</div><div class="value amount-income">+${data.income.toFixed(2)}</div></div>
      <div class="stat"><div class="label">支出</div><div class="value amount-expense">-${data.expense.toFixed(2)}</div></div>
      <div class="stat"><div class="label">结余</div><div class="value">${data.balance.toFixed(2)}</div></div>
      <div class="stat"><div class="label">笔数</div><div class="value">${data.count}</div></div>
    </div>
  `;
  if (dailyTotal > 0) {
    html += `
      <h3>按日统计</h3>
      <table class="table">
        <thead>
          <tr>
            <th id="reportThPeriod">日期</th>
            <th id="reportThIncome">收入</th>
            <th id="reportThExpense">支出</th>
            <th id="reportThBalance">结余</th>
            <th id="reportThCount">笔数</th>
          </tr>
        </thead>
        <tbody>
          ${dailyList.map(d => `
            <tr>
              <td>${d.period}</td>
              <td class="amount-income">+${d.income.toFixed(2)}</td>
              <td class="amount-expense">-${d.expense.toFixed(2)}</td>
              <td>${d.balance.toFixed(2)}</td>
              <td>${d.count}</td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    `;
  }
  if (catTotal > 0) {
    html += `
      <h3>按分类统计</h3>
      <table class="table">
        <thead>
          <tr>
            <th id="reportThCat">分类</th>
            <th id="reportThCatIncome">收入</th>
            <th id="reportThCatExpense">支出</th>
            <th id="reportThCatTotal">合计</th>
            <th id="reportThCatCount">笔数</th>
          </tr>
        </thead>
        <tbody>
          ${catList.map(c => `
            <tr>
              <td>${c.category}</td>
              <td class="amount-income">+${c.income.toFixed(2)}</td>
              <td class="amount-expense">-${c.expense.toFixed(2)}</td>
              <td class="${c.total >= 0 ? 'amount-income' : 'amount-expense'}">
                ${c.total >= 0 ? '+' : ''}${c.total.toFixed(2)}
              </td>
              <td>${c.count}</td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    `;
  }
  reportContent.innerHTML = html;

  const showDailyPager = dailyTotal > reportDailyPageSize;
  const showCategoryPager = catTotal > reportCategoryPageSize;

  if (reportDailyPaginationWrap) reportDailyPaginationWrap.style.display = showDailyPager ? 'flex' : 'none';
  if (reportCategoryPaginationWrap) reportCategoryPaginationWrap.style.display = showCategoryPager ? 'flex' : 'none';

  if (showDailyPager) {
    renderPager(reportDailyPaginationEl, dailyTotal, currentReportDailyPage, reportDailyPageSize, (page) => {
      currentReportDailyPage = page;
      renderReport(currentReportData);
    });
  } else if (reportDailyPaginationEl) {
    reportDailyPaginationEl.innerHTML = '';
  }

  if (showCategoryPager) {
    renderPager(reportCategoryPaginationEl, catTotal, currentReportCategoryPage, reportCategoryPageSize, (page) => {
      currentReportCategoryPage = page;
      renderReport(currentReportData);
    });
  } else if (reportCategoryPaginationEl) {
    reportCategoryPaginationEl.innerHTML = '';
  }

  refreshReportSortIndicators();
  attachReportSortHandlers();
}

function attachReportSortHandlers() {
  if (!currentReportData) return;
  if (currentReportDaily && currentReportDaily.length > 0) {
    const thP = document.getElementById('reportThPeriod');
    const thI = document.getElementById('reportThIncome');
    const thE = document.getElementById('reportThExpense');
    const thB = document.getElementById('reportThBalance');
    const thC = document.getElementById('reportThCount');
    thP && (thP.onclick = () => {
      currentReportDaily = sortWithToggle(currentReportDaily, 'period', 'string', reportDailySort);
      currentReportDailyPage = 1;
      renderReport(currentReportData);
    });
    thI && (thI.onclick = () => {
      currentReportDaily = sortWithToggle(currentReportDaily, 'income', 'number', reportDailySort);
      currentReportDailyPage = 1;
      renderReport(currentReportData);
    });
    thE && (thE.onclick = () => {
      currentReportDaily = sortWithToggle(currentReportDaily, 'expense', 'number', reportDailySort);
      currentReportDailyPage = 1;
      renderReport(currentReportData);
    });
    thB && (thB.onclick = () => {
      currentReportDaily = sortWithToggle(currentReportDaily, 'balance', 'number', reportDailySort);
      currentReportDailyPage = 1;
      renderReport(currentReportData);
    });
    thC && (thC.onclick = () => {
      currentReportDaily = sortWithToggle(currentReportDaily, 'count', 'number', reportDailySort);
      currentReportDailyPage = 1;
      renderReport(currentReportData);
    });
  }
  if (currentReportCategory && currentReportCategory.length > 0) {
    const thCat = document.getElementById('reportThCat');
    const thI2 = document.getElementById('reportThCatIncome');
    const thE2 = document.getElementById('reportThCatExpense');
    const thT2 = document.getElementById('reportThCatTotal');
    const thC2 = document.getElementById('reportThCatCount');
    thCat && (thCat.onclick = () => {
      currentReportCategory = sortWithToggle(currentReportCategory, 'category', 'string', reportCatSort);
      currentReportCategoryPage = 1;
      renderReport(currentReportData);
    });
    thI2 && (thI2.onclick = () => {
      currentReportCategory = sortWithToggle(currentReportCategory, 'income', 'number', reportCatSort);
      currentReportCategoryPage = 1;
      renderReport(currentReportData);
    });
    thE2 && (thE2.onclick = () => {
      currentReportCategory = sortWithToggle(currentReportCategory, 'expense', 'number', reportCatSort);
      currentReportCategoryPage = 1;
      renderReport(currentReportData);
    });
    thT2 && (thT2.onclick = () => {
      currentReportCategory = sortWithToggle(currentReportCategory, 'total', 'number', reportCatSort);
      currentReportCategoryPage = 1;
      renderReport(currentReportData);
    });
    thC2 && (thC2.onclick = () => {
      currentReportCategory = sortWithToggle(currentReportCategory, 'count', 'number', reportCatSort);
      currentReportCategoryPage = 1;
      renderReport(currentReportData);
    });
  }
}

document.getElementById('btnReport').addEventListener('click', loadReport);

// 日期输入显示 yyyy/mm/dd
function syncDateDisplay(inputId) {
  const input = document.getElementById(inputId);
  const display = document.querySelector(`.date-display[data-for="${inputId}"]`);
  const field = display?.closest('.date-field');
  if (!input || !display || !field) return;
  const v = input.value;
  if (v) {
    display.textContent = v.replace(/-/g, '/');
    field.classList.add('has-value');
  } else {
    display.textContent = '';
    field.classList.remove('has-value');
  }
}

function initDateDisplays() {
  ['startDate', 'endDate', 'formDate', 'summaryDate', 'reportStartDate', 'reportEndDate'].forEach(id => {
    const input = document.getElementById(id);
    if (!input) return;
    input.addEventListener('change', () => syncDateDisplay(id));
    input.addEventListener('input', () => syncDateDisplay(id));
    syncDateDisplay(id);
    const field = input.closest('.date-field');
    if (field) {
      field.addEventListener('click', () => {
        input.focus();
        if (typeof input.showPicker === 'function') input.showPicker();
      });
    }
  });
}
initDateDisplays();

// 主题切换
function initTheme() {
  const saved = localStorage.getItem('theme') || 'dark';
  const body = document.body;
  if (saved === 'light') {
    body.classList.add('theme-light');
    if (btnThemeToggle) btnThemeToggle.textContent = '☀️ 浅色';
  } else {
    body.classList.remove('theme-light');
    if (btnThemeToggle) btnThemeToggle.textContent = '🌙 深色';
  }
}

function toggleTheme() {
  const body = document.body;
  const isLight = body.classList.toggle('theme-light');
  localStorage.setItem('theme', isLight ? 'light' : 'dark');
  if (btnThemeToggle) {
    btnThemeToggle.textContent = isLight ? '☀️ 浅色' : '🌙 深色';
  }
}

if (btnThemeToggle) {
  btnThemeToggle.addEventListener('click', toggleTheme);
}

// 初始加载
initTheme();
initUserRole().then(() => loadPage());
