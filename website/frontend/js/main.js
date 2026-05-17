// ── Firebase config (replace with yours) ──────────────────────
const firebaseConfig = {
  apiKey:            "YOUR_API_KEY",
  authDomain:        "vortelio-3e7a8.firebaseapp.com",
  projectId:         "vortelio-3e7a8",
  storageBucket:     "vortelio-3e7a8.appspot.com",
  messagingSenderId: "YOUR_SENDER_ID",
  appId:             "YOUR_APP_ID",
};

// ── Init ──────────────────────────────────────────────────────
let _fbUser = null;

function initFirebase() {
  if (typeof firebase === 'undefined') return;
  firebase.initializeApp(firebaseConfig);
  firebase.auth().onAuthStateChanged(user => {
    _fbUser = user;
    updateAuthUI();
  });
}

function updateAuthUI() {
  const loginBtn  = document.getElementById('nav-login');
  const logoutBtn = document.getElementById('nav-logout');
  const accountEl = document.getElementById('nav-account');
  if (!loginBtn) return;

  if (_fbUser) {
    loginBtn.style.display  = 'none';
    if (logoutBtn)  logoutBtn.style.display  = '';
    if (accountEl)  accountEl.style.display  = '';
    if (accountEl)  accountEl.textContent = _fbUser.email || 'Account';
  } else {
    loginBtn.style.display  = '';
    if (logoutBtn)  logoutBtn.style.display  = 'none';
    if (accountEl)  accountEl.style.display  = 'none';
  }
}

// ── Checkout ──────────────────────────────────────────────────
async function upgrade(plan) {
  if (!_fbUser) {
    // Redirect to login then back
    sessionStorage.setItem('pendingPlan', plan);
    window.location.href = '/login.html';
    return;
  }

  const btn = document.querySelector(`[data-plan="${plan}"] .upgrade-btn`);
  if (btn) { btn.textContent = 'Loading…'; btn.disabled = true; }

  try {
    const token = await _fbUser.getIdToken();
    const origin = location.origin;
    const res = await fetch('/api/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({
        plan,
        email:       _fbUser.email,
        uid:         _fbUser.uid,
        success_url: origin + '/success.html?plan=' + plan,
        cancel_url:  origin + '/#pricing',
      }),
    });
    const data = await res.json();
    if (data.url) {
      window.location.href = data.url;
    } else {
      toast('❌ ' + (data.error || 'Checkout error'), true);
      if (btn) { btn.textContent = 'Upgrade'; btn.disabled = false; }
    }
  } catch (e) {
    toast('❌ ' + e.message, true);
    if (btn) { btn.textContent = 'Upgrade'; btn.disabled = false; }
  }
}

// ── Download ──────────────────────────────────────────────────
function downloadLatest(os) {
  const releases = 'https://github.com/metiu1/Vortelio/releases/latest/download/';
  const map = {
    windows: 'vortelio-windows-amd64.exe',
    mac:     'vortelio-darwin-amd64',
    linux:   'vortelio-linux-amd64',
  };
  if (map[os]) window.location.href = releases + map[os];
}

// ── Toast ─────────────────────────────────────────────────────
function toast(msg, isErr) {
  const el = document.getElementById('toast');
  if (!el) return;
  el.textContent = msg;
  el.style.borderColor = isErr ? '#dc2626' : '';
  el.classList.add('show');
  setTimeout(() => el.classList.remove('show'), 3000);
}

// ── Handle pending upgrade after login ────────────────────────
function checkPendingPlan() {
  const plan = sessionStorage.getItem('pendingPlan');
  if (plan && _fbUser) {
    sessionStorage.removeItem('pendingPlan');
    upgrade(plan);
  }
}

// ── Boot ──────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  initFirebase();
});
