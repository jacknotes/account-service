const loginForm = document.getElementById('loginForm');
const registerForm = document.getElementById('registerForm');
const totpRow = document.getElementById('totpRow');
const loginError = document.getElementById('loginError');
const regError = document.getElementById('regError');
const switchHint = document.getElementById('switchHint');
const btnSwitch = document.getElementById('btnSwitch');

let needsTOTP = false;

async function login() {
  const username = document.getElementById('loginUsername').value.trim();
  const password = document.getElementById('loginPassword').value;
  const totpCode = document.getElementById('loginTOTP').value.trim();
  loginError.textContent = '';
  if (!username || !password) {
    loginError.textContent = '请输入用户名和密码';
    return;
  }
  if (needsTOTP && !totpCode) {
    loginError.textContent = '请输入 TOTP 验证码';
    return;
  }
  try {
    const res = await fetch(API + '/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, totp_code: totpCode || undefined }),
    });
    const data = await res.json();
    if (!res.ok) {
      loginError.textContent = data.error || '登录失败';
      return;
    }
    if (data.needs_totp) {
      needsTOTP = true;
      totpRow.style.display = 'block';
      document.getElementById('loginTOTP').focus();
      loginError.textContent = '请输入 TOTP 验证码';
      return;
    }
    setToken(data.token);
    window.location.href = '/app/';
  } catch (e) {
    loginError.textContent = e.message || '网络错误';
  }
}

async function register() {
  const username = document.getElementById('regUsername').value.trim();
  const password = document.getElementById('regPassword').value;
  regError.textContent = '';
  if (!username || !password) {
    regError.textContent = '请输入用户名和密码';
    return;
  }
  if (password.length < 6) {
    regError.textContent = '密码至少 6 位';
    return;
  }
  try {
    const res = await fetch(API + '/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const data = await res.json();
    if (!res.ok) {
      regError.textContent = data.error || '注册失败';
      return;
    }
    setToken(data.token);
    window.location.href = '/app/';
  } catch (e) {
    regError.textContent = e.message || '网络错误';
  }
}

function showRegister() {
  loginForm.style.display = 'none';
  registerForm.style.display = 'block';
  switchHint.textContent = '已有账号？';
  btnSwitch.textContent = '登录';
  btnSwitch.onclick = showLogin;
}

function showLogin() {
  registerForm.style.display = 'none';
  loginForm.style.display = 'block';
  switchHint.textContent = '尚无账号？';
  btnSwitch.textContent = '注册';
  btnSwitch.onclick = showRegister;
  needsTOTP = false;
  totpRow.style.display = 'none';
}

document.getElementById('btnLogin').addEventListener('click', login);
document.getElementById('btnRegister').addEventListener('click', register);
document.getElementById('btnShowLogin').addEventListener('click', showLogin);
btnSwitch.addEventListener('click', (e) => { e.preventDefault(); showRegister(); });

document.getElementById('loginPassword').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') login();
});
document.getElementById('loginTOTP').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') login();
});
document.getElementById('regPassword').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') register();
});

if (isLoggedIn()) {
  window.location.href = '/app/';
} else {
  // 仅当无用户时显示注册入口
  fetch(API + '/auth/register/status')
    .then(r => r.json())
    .then(data => {
      if (data.allow_register) {
        document.getElementById('authSwitch').style.display = 'block';
      }
    })
    .catch(() => {});
}
