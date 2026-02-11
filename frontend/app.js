// API 已在 auth.js 中定义
if (!isLoggedIn()) {
  window.location.href = '/app/login.html';
}

let currentPage = 1;
const pageSize = 15;
let deleteTargetId = null;

// DOM
const tableBody = document.getElementById('tableBody');
const paginationEl = document.getElementById('pagination');
const modal = document.getElementById('modal');
const deleteModal = document.getElementById('deleteModal');
const recordForm = document.getElementById('recordForm');
const modalTitle = document.getElementById('modalTitle');
const recordId = document.getElementById('recordId');
const formDate = document.getElementById('formDate');
const formAmount = document.getElementById('formAmount');
const formCategory = document.getElementById('formCategory');
const formDescription = document.getElementById('formDescription');

const startDate = document.getElementById('startDate');
const endDate = document.getElementById('endDate');
const keyword = document.getElementById('keyword');

// 初始化今日日期
formDate.value = new Date().toISOString().slice(0, 10);

async function fetchRecords(page = 1) {
  const params = new URLSearchParams();
  params.set('page', page);
  params.set('page_size', pageSize);
  if (startDate.value) params.set('start_date', startDate.value);
  if (endDate.value) params.set('end_date', endDate.value);
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim());

  const res = await fetchAuth(`${API}/records?${params}`);
  if (!res.ok) throw new Error('获取列表失败');
  return res.json();
}

function renderTable(data) {
  if (!data.data || data.data.length === 0) {
    tableBody.innerHTML = '<tr><td colspan="5" class="empty">暂无记录</td></tr>';
    return;
  }
  tableBody.innerHTML = data.data.map(r => `
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

function renderPagination(total, page) {
  const totalPages = Math.ceil(total / pageSize) || 1;
  paginationEl.innerHTML = `
    <button ${page <= 1 ? 'disabled' : ''} data-page="${page - 1}">上一页</button>
    <span>第 ${page} / ${totalPages} 页，共 ${total} 条</span>
    <button ${page >= totalPages ? 'disabled' : ''} data-page="${page + 1}">下一页</button>
  `;
  paginationEl.querySelectorAll('button').forEach(btn => {
    if (!btn.disabled) {
      btn.addEventListener('click', () => loadPage(Number(btn.dataset.page)));
    }
  });
}

async function loadPage(page = 1) {
  tableBody.innerHTML = '<tr><td colspan="5" class="loading">加载中...</td></tr>';
  try {
    const data = await fetchRecords(page);
    currentPage = page;
    renderTable(data);
    renderPagination(data.total, data.page);
  } catch (e) {
    tableBody.innerHTML = `<tr><td colspan="5" class="empty">加载失败: ${e.message}</td></tr>`;
  }
}

function openAdd() {
  modalTitle.textContent = '添加记录';
  recordId.value = '';
  recordForm.reset();
  formDate.value = new Date().toISOString().slice(0, 10);
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

document.getElementById('btnDeleteCancel').addEventListener('click', closeDeleteModal);
document.getElementById('btnDeleteConfirm').addEventListener('click', doDelete);

modal.addEventListener('click', (e) => {
  if (e.target === modal) closeModal();
});
deleteModal.addEventListener('click', (e) => {
  if (e.target === deleteModal) closeDeleteModal();
});
document.getElementById('settingsModal').addEventListener('click', (e) => {
  if (e.target.id === 'settingsModal') e.target.classList.remove('show');
});

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

async function loadUsers() {
  const tbody = document.getElementById('usersTableBody');
  try {
    const res = await fetchAuth(`${API}/auth/users`);
    if (!res.ok) throw new Error('无权限');
    const data = await res.json();
    if (!data.data || data.data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无用户</td></tr>';
      return;
    }
    tbody.innerHTML = data.data.map(u => `
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
    tbody.querySelectorAll('.btn-edit-user').forEach(btn => {
      btn.addEventListener('click', () => openEditUser(Number(btn.dataset.id)));
    });
    tbody.querySelectorAll('.btn-chpwd-user').forEach(btn => {
      btn.addEventListener('click', () => openChgPwdUser(Number(btn.dataset.id)));
    });
    tbody.querySelectorAll('.btn-del-user').forEach(btn => {
      btn.addEventListener('click', () => openDeleteUser(Number(btn.dataset.id)));
    });
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">${e.message}</td></tr>`;
  }
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

// ===== 操作日志（管理员） =====
let logsPage = 1;
const logsPageSize = 20;
async function loadLogs(page = 1) {
  const tbody = document.getElementById('logsTableBody');
  const paginationEl = document.getElementById('logsPagination');
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="5" class="loading">加载中...</td></tr>';
  try {
    const params = new URLSearchParams();
    params.set('page', page);
    params.set('page_size', logsPageSize);
    const action = document.getElementById('logActionFilter')?.value;
    if (action) params.set('action', action);
    const res = await fetchAuth(`${API}/auth/operation-logs?${params}`);
    if (!res.ok) throw new Error('无权限');
    const data = await res.json();
    logsPage = page;
    if (!data.data || data.data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无日志</td></tr>';
      paginationEl.innerHTML = '';
      return;
    }
    tbody.innerHTML = data.data.map(l => `
      <tr>
        <td>${l.created_at ? l.created_at.slice(0, 19).replace('T', ' ') : '-'}</td>
        <td>${l.username}</td>
        <td>${l.action}</td>
        <td>${l.detail || '-'}</td>
        <td>${l.ip || '-'}</td>
      </tr>
    `).join('');
    const totalPages = Math.ceil(data.total / logsPageSize) || 1;
    paginationEl.innerHTML = `
      <button ${page <= 1 ? 'disabled' : ''} data-page="${page - 1}">上一页</button>
      <span>第 ${page} / ${totalPages} 页，共 ${data.total} 条</span>
      <button ${page >= totalPages ? 'disabled' : ''} data-page="${page + 1}">下一页</button>
    `;
    paginationEl.querySelectorAll('button').forEach(btn => {
      if (!btn.disabled) btn.addEventListener('click', () => loadLogs(Number(btn.dataset.page)));
    });
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">${e.message}</td></tr>`;
  }
}
document.getElementById('btnLoadLogs')?.addEventListener('click', () => loadLogs(1));

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
    renderSummaryCards(data);
    renderSummaryDetail(data);
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

function renderSummaryDetail(data) {
  if (data.records && data.records.length > 0) {
    summaryDetail.innerHTML = `
      <h3>明细</h3>
      <table class="table">
        <thead><tr><th>日期</th><th>金额</th><th>分类</th><th>描述</th></tr></thead>
        <tbody>
          ${data.records.map(r => `
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
  } else if (data.breakdown && data.breakdown.length > 0) {
    summaryDetail.innerHTML = `
      <h3>分项</h3>
      <table class="table">
        <thead><tr><th>日期/月份</th><th>收入</th><th>支出</th><th>结余</th><th>笔数</th></tr></thead>
        <tbody>
          ${data.breakdown.map(b => `
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
  } else {
    summaryDetail.innerHTML = '<h3>明细</h3><div class="empty">暂无数据</div>';
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
    renderReport(data);
  } catch (e) {
    reportContent.innerHTML = `<div class="empty">${e.message}</div>`;
  }
}

function renderReport(data) {
  let html = `
    <div class="report-overview">
      <div class="stat"><div class="label">收入</div><div class="value amount-income">+${data.income.toFixed(2)}</div></div>
      <div class="stat"><div class="label">支出</div><div class="value amount-expense">-${data.expense.toFixed(2)}</div></div>
      <div class="stat"><div class="label">结余</div><div class="value">${data.balance.toFixed(2)}</div></div>
      <div class="stat"><div class="label">笔数</div><div class="value">${data.count}</div></div>
    </div>
  `;
  if (data.daily && data.daily.length > 0) {
    html += `
      <h3>按日统计</h3>
      <table class="table">
        <thead><tr><th>日期</th><th>收入</th><th>支出</th><th>结余</th><th>笔数</th></tr></thead>
        <tbody>
          ${data.daily.map(d => `
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
  if (data.by_category && data.by_category.length > 0) {
    html += `
      <h3>按分类统计</h3>
      <table class="table">
        <thead><tr><th>分类</th><th>收入</th><th>支出</th><th>合计</th><th>笔数</th></tr></thead>
        <tbody>
          ${data.by_category.map(c => `
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

// 初始加载
initUserRole().then(() => loadPage());
