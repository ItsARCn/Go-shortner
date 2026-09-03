document.addEventListener('DOMContentLoaded', async () => {
  let currentUser = null;
  let activeTab = 'overview';

  // Navigation tabs
  const tabs = document.querySelectorAll('.admin-tab');
  const tabContents = document.querySelectorAll('.admin-tab-content');
  const adminGreeting = document.getElementById('admin-greeting');
  const badgeReports = document.getElementById('badge-reports-count');
  const adminLogoutBtn = document.getElementById('admin-logout-btn');

  // Verify Admin Access
  try {
    const meRes = await fetch('/api/auth/me');
    if (!meRes.ok) {
      window.location.href = '/login';
      return;
    }
    currentUser = await meRes.json();
    if (currentUser.role !== 'super_admin' && currentUser.role !== 'moderator') {
      alert('Access Denied: You do not have administrator permissions.');
      window.location.href = '/dashboard';
      return;
    }
    adminGreeting.textContent = `${currentUser.first_name} (${currentUser.role})`;
  } catch (err) {
    window.location.href = '/login';
    return;
  }

  // Tab Switching
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tabContents.forEach(c => c.classList.remove('active'));

      tab.classList.add('active');
      activeTab = tab.getAttribute('data-tab');
      document.getElementById(`tab-${activeTab}`).classList.add('active');

      loadActiveTabData();
    });
  });

  function loadActiveTabData() {
    switch (activeTab) {
      case 'overview': loadOverview(); break;
      case 'users': loadUsers(); break;
      case 'links': loadLinks(); break;
      case 'reports': loadReports(); break;
      case 'logs': loadLogs(); break;
    }
  }

  // 1. OVERVIEW
  async function loadOverview() {
    try {
      const res = await fetch('/api/admin/overview');
      if (!res.ok) return;
      const d = await res.json();

      document.getElementById('ov-users').textContent = d.total_users.toLocaleString();
      document.getElementById('ov-links').textContent = d.total_links.toLocaleString();
      document.getElementById('ov-active').textContent = d.active_links.toLocaleString();
      document.getElementById('ov-expired').textContent = d.expired_links.toLocaleString();
      document.getElementById('ov-reports').textContent = d.reports_count.toLocaleString();
      document.getElementById('ov-banned').textContent = d.banned_users.toLocaleString();
      document.getElementById('ov-timeout').textContent = d.timed_out_users.toLocaleString();

      badgeReports.textContent = d.reports_count;
    } catch (err) {}
  }

  // 2. USERS
  let userStatusFilter = 'all';
  let userSearchTerm = '';
  const usersTbody = document.getElementById('admin-users-tbody');
  const userSearchInput = document.getElementById('admin-user-search');

  document.querySelectorAll('.user-filter-status').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.user-filter-status').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      userStatusFilter = btn.getAttribute('data-status');
      loadUsers();
    });
  });

  let userSearchTimeout = null;
  userSearchInput.addEventListener('input', () => {
    clearTimeout(userSearchTimeout);
    userSearchTimeout = setTimeout(() => {
      userSearchTerm = userSearchInput.value.trim();
      loadUsers();
    }, 300);
  });

  async function loadUsers() {
    try {
      const res = await fetch(`/api/admin/users?status=${userStatusFilter}&search=${encodeURIComponent(userSearchTerm)}`);
      if (!res.ok) {
        usersTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Failed to load users</td></tr>`;
        return;
      }
      const data = await res.json();
      renderUsers(data.users);
    } catch (err) {
      usersTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Network error</td></tr>`;
    }
  }

  function renderUsers(users) {
    if (!users || users.length === 0) {
      usersTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:var(--text-dim); padding:2rem;">No users found.</td></tr>`;
      return;
    }

    usersTbody.innerHTML = users.map(u => {
      let statusBadge = `<span class="badge-active">ACTIVE</span>`;
      if (u.status === 'banned') {
        statusBadge = `<span class="badge-expired">BANNED</span>`;
      } else if (u.status === 'timed_out') {
        const until = u.timeout_until ? new Date(u.timeout_until).toLocaleTimeString() : '';
        statusBadge = `<span style="background:rgba(167,139,250,0.15); color:#a78bfa; font-size:0.7rem; font-weight:700; padding:0.2rem 0.5rem; border-radius:9999px;">TIMED OUT (${until})</span>`;
      }

      const isProtected = (u.role === 'super_admin');

      return `
        <tr>
          <td>
            <div style="font-weight:600;">${escapeHTML(u.first_name)} ${escapeHTML(u.last_name)}</div>
            <div style="font-size:0.8rem; color:var(--text-muted);">${escapeHTML(u.email)}</div>
          </td>
          <td><span class="click-chip">${escapeHTML(u.auth_provider)}</span></td>
          <td><span style="font-size:0.8rem; font-weight:600; text-transform:uppercase; color:var(--text-muted);">${u.role}</span></td>
          <td>${statusBadge}</td>
          <td><span class="click-chip">${u.link_count}</span></td>
          <td style="font-size:0.8rem; color:var(--text-dim);">${new Date(u.created_at).toLocaleDateString()}</td>
          <td style="text-align:right;">
            ${isProtected ? `<span style="font-size:0.75rem; color:var(--text-dim);">Protected</span>` : `
              <div class="table-actions">
                ${u.status === 'banned' || u.status === 'timed_out' ? `
                  <button class="action-btn unban-user-btn" data-id="${u.id}">Unban</button>
                ` : `
                  <button class="action-btn timeout-user-btn" data-id="${u.id}" data-email="${escapeHTML(u.email)}">Timeout</button>
                  <button class="action-btn ban-user-btn" data-id="${u.id}" data-email="${escapeHTML(u.email)}">Ban</button>
                `}
              </div>
            `}
          </td>
        </tr>
      `;
    }).join('');

    // Attach actions
    document.querySelectorAll('.timeout-user-btn').forEach(b => {
      b.addEventListener('click', () => {
        openTimeoutModal(b.getAttribute('data-id'), b.getAttribute('data-email'));
      });
    });

    document.querySelectorAll('.ban-user-btn').forEach(b => {
      b.addEventListener('click', () => {
        openBanModal(b.getAttribute('data-id'), b.getAttribute('data-email'));
      });
    });

    document.querySelectorAll('.unban-user-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const id = b.getAttribute('data-id');
        if (confirm('Restore user to active status?')) {
          await fetch(`/api/admin/users/${id}/unban`, { method: 'POST' });
          loadUsers();
          loadOverview();
        }
      });
    });
  }

  // 3. LINKS
  let linkStatusFilter = 'all';
  let linkSearchTerm = '';
  const linksTbody = document.getElementById('admin-links-tbody');
  const linkSearchInput = document.getElementById('admin-link-search');

  document.querySelectorAll('.link-filter-status').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.link-filter-status').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      linkStatusFilter = btn.getAttribute('data-status');
      loadLinks();
    });
  });

  let linkSearchTimeout = null;
  linkSearchInput.addEventListener('input', () => {
    clearTimeout(linkSearchTimeout);
    linkSearchTimeout = setTimeout(() => {
      linkSearchTerm = linkSearchInput.value.trim();
      loadLinks();
    }, 300);
  });

  async function loadLinks() {
    try {
      const res = await fetch(`/api/admin/links?status=${linkStatusFilter}&search=${encodeURIComponent(linkSearchTerm)}`);
      if (!res.ok) {
        linksTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Failed to load links</td></tr>`;
        return;
      }
      const data = await res.json();
      renderLinks(data.links);
    } catch (err) {
      linksTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Network error</td></tr>`;
    }
  }

  function renderLinks(links) {
    if (!links || links.length === 0) {
      linksTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:var(--text-dim); padding:2rem;">No links found.</td></tr>`;
      return;
    }

    linksTbody.innerHTML = links.map(l => {
      let statusBadge = `<span class="badge-active">${l.status}</span>`;
      if (l.status === 'DISABLED' || l.status === 'EXPIRED' || l.status === 'DELETED') {
        statusBadge = `<span class="badge-expired">${l.status}</span>`;
      }

      const reportBadge = l.report_count > 0
        ? `<span style="background:rgba(239,68,68,0.2); color:#f87171; font-weight:700; font-size:0.75rem; padding:0.15rem 0.4rem; border-radius:4px;">${l.report_count}</span>`
        : `<span style="color:var(--text-dim); font-size:0.8rem;">0</span>`;

      const fullShortURL = `${window.location.origin}/${l.short_code}`;

      return `
        <tr>
          <td>
            <a href="${fullShortURL}" target="_blank" class="link-code">/${l.short_code}</a>
          </td>
          <td><div class="dest-url" title="${escapeHTML(l.destination_url)}">${escapeHTML(l.destination_url)}</div></td>
          <td style="font-size:0.85rem; color:var(--text-muted);">${escapeHTML(l.owner_email)}</td>
          <td><span class="click-chip">${l.click_count}</span></td>
          <td>${reportBadge}</td>
          <td>${statusBadge}</td>
          <td style="text-align:right;">
            <div class="table-actions">
              ${l.status === 'DISABLED' ? `
                <button class="action-btn enable-link-btn" data-code="${l.short_code}" style="color:#34d399;">Enable</button>
              ` : `
                <button class="action-btn disable-link-btn" data-code="${l.short_code}" style="color:#fb7185;">Disable</button>
              `}
              <button class="action-btn delete-link-btn" data-code="${l.short_code}">Delete</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');

    document.querySelectorAll('.disable-link-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const code = b.getAttribute('data-code');
        if (confirm(`Disable /${code}? Redirection will immediately stop.`)) {
          await fetch(`/api/admin/links/${code}/disable`, { method: 'POST' });
          loadLinks();
          loadOverview();
        }
      });
    });

    document.querySelectorAll('.enable-link-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const code = b.getAttribute('data-code');
        await fetch(`/api/admin/links/${code}/enable`, { method: 'POST' });
        loadLinks();
        loadOverview();
      });
    });

    document.querySelectorAll('.delete-link-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const code = b.getAttribute('data-code');
        if (confirm(`Delete /${code}?`)) {
          await fetch(`/api/admin/links/${code}`, { method: 'DELETE' });
          loadLinks();
          loadOverview();
        }
      });
    });
  }

  // 4. REPORTS
  let reportStatusFilter = 'pending';
  const reportsTbody = document.getElementById('admin-reports-tbody');

  document.querySelectorAll('.report-filter-status').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.report-filter-status').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      reportStatusFilter = btn.getAttribute('data-status');
      loadReports();
    });
  });

  async function loadReports() {
    try {
      const res = await fetch(`/api/admin/reports?status=${reportStatusFilter}`);
      if (!res.ok) {
        reportsTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Failed to load reports</td></tr>`;
        return;
      }
      const data = await res.json();
      renderReports(data.reports);
    } catch (err) {
      reportsTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:#f43f5e; padding:2rem;">Network error</td></tr>`;
    }
  }

  function renderReports(reports) {
    if (!reports || reports.length === 0) {
      reportsTbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:var(--text-dim); padding:2rem;">No ${reportStatusFilter} reports.</td></tr>`;
      return;
    }

    reportsTbody.innerHTML = reports.map(r => `
      <tr>
        <td>
          <a href="/${r.short_code}" target="_blank" class="link-code">/${r.short_code}</a>
          <div class="dest-url" style="font-size:0.75rem; max-width:200px;">${escapeHTML(r.destination_url)}</div>
        </td>
        <td><span style="color:#fb7185; font-weight:700; font-size:0.8rem; text-transform:uppercase;">${escapeHTML(r.reason)}</span></td>
        <td style="font-size:0.85rem; max-width:220px;">${escapeHTML(r.details || 'None')}</td>
        <td style="font-family:var(--font-mono); font-size:0.75rem; color:var(--text-dim);">${escapeHTML(r.reporter_ip_hash.substring(0, 12))}...</td>
        <td style="font-size:0.8rem; color:var(--text-dim);">${new Date(r.created_at).toLocaleString()}</td>
        <td><span class="click-chip">${r.status}</span></td>
        <td style="text-align:right;">
          <div class="table-actions">
            ${r.status === 'pending' ? `
              <button class="action-btn disable-and-resolve-btn" data-id="${r.id}" data-code="${r.short_code}" style="color:#fb7185;">Disable Link</button>
              <button class="action-btn dismiss-report-btn" data-id="${r.id}">Dismiss</button>
            ` : `
              <span style="font-size:0.8rem; color:var(--text-dim);">Done</span>
            `}
          </div>
        </td>
      </tr>
    `).join('');

    document.querySelectorAll('.disable-and-resolve-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const id = b.getAttribute('data-id');
        const code = b.getAttribute('data-code');
        if (confirm(`Disable /${code} and mark report as reviewed?`)) {
          await fetch(`/api/admin/links/${code}/disable`, { method: 'POST' });
          await fetch(`/api/admin/reports/${id}/resolve`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ status: 'reviewed' })
          });
          loadReports();
          loadOverview();
        }
      });
    });

    document.querySelectorAll('.dismiss-report-btn').forEach(b => {
      b.addEventListener('click', async () => {
        const id = b.getAttribute('data-id');
        await fetch(`/api/admin/reports/${id}/resolve`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: 'dismissed' })
        });
        loadReports();
        loadOverview();
      });
    });
  }

  // 5. LOGIN RECORDS
  let logResultFilter = 'all';
  const logsTbody = document.getElementById('admin-logs-tbody');

  document.querySelectorAll('.log-filter-result').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.log-filter-result').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      logResultFilter = btn.getAttribute('data-result');
      loadLogs();
    });
  });

  async function loadLogs() {
    try {
      const res = await fetch(`/api/admin/login-records?result=${logResultFilter}`);
      if (!res.ok) {
        logsTbody.innerHTML = `<tr><td colspan="6" style="text-align:center; color:#f43f5e; padding:2rem;">Failed to load audit records</td></tr>`;
        return;
      }
      const data = await res.json();
      renderLogs(data.records);
    } catch (err) {
      logsTbody.innerHTML = `<tr><td colspan="6" style="text-align:center; color:#f43f5e; padding:2rem;">Network error</td></tr>`;
    }
  }

  function renderLogs(records) {
    if (!records || records.length === 0) {
      logsTbody.innerHTML = `<tr><td colspan="6" style="text-align:center; color:var(--text-dim); padding:2rem;">No login records found.</td></tr>`;
      return;
    }

    logsTbody.innerHTML = records.map(r => {
      let badge = `<span class="badge-active">SUCCESS</span>`;
      if (r.result === 'FAILED') {
        badge = `<span class="badge-expired">FAILED</span>`;
      } else if (r.result === 'UNAUTHORIZED') {
        badge = `<span style="background:rgba(239,68,68,0.25); color:#f87171; font-weight:700; font-size:0.7rem; padding:0.2rem 0.5rem; border-radius:9999px;">UNAUTHORIZED</span>`;
      }

      return `
        <tr>
          <td style="font-size:0.8rem; color:var(--text-dim); font-family:var(--font-mono);">${new Date(r.created_at).toLocaleString()}</td>
          <td style="font-weight:500;">${escapeHTML(r.account_email)}</td>
          <td><span class="click-chip">${r.auth_method}</span></td>
          <td>${badge}</td>
          <td style="font-family:var(--font-mono); font-size:0.75rem; color:var(--text-dim);">${escapeHTML(r.ip_hash.substring(0, 14))}...</td>
          <td style="font-size:0.75rem; color:var(--text-muted); max-width:240px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">${escapeHTML(r.user_agent)}</td>
        </tr>
      `;
    }).join('');
  }

  // Modals (Timeout & Ban)
  const timeoutModal = document.getElementById('timeout-modal');
  const banModal = document.getElementById('ban-modal');

  function openTimeoutModal(id, email) {
    document.getElementById('timeout-user-id').value = id;
    document.getElementById('timeout-target-email').textContent = email;
    timeoutModal.classList.add('show');
  }

  function openBanModal(id, email) {
    document.getElementById('ban-user-id').value = id;
    document.getElementById('ban-target-email').textContent = email;
    banModal.classList.add('show');
  }

  document.getElementById('close-timeout-modal').addEventListener('click', () => timeoutModal.classList.remove('show'));
  document.getElementById('cancel-timeout-btn').addEventListener('click', () => timeoutModal.classList.remove('show'));
  document.getElementById('close-ban-modal').addEventListener('click', () => banModal.classList.remove('show'));
  document.getElementById('cancel-ban-btn').addEventListener('click', () => banModal.classList.remove('show'));

  document.getElementById('timeout-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('timeout-user-id').value;
    const duration = document.getElementById('timeout-duration-select').value;
    const reason = document.getElementById('timeout-reason').value;

    const res = await fetch(`/api/admin/users/${id}/timeout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ duration, reason })
    });

    if (res.ok) {
      timeoutModal.classList.remove('show');
      loadUsers();
      loadOverview();
    } else {
      const d = await res.json();
      alert(d.error || 'Failed to timeout user');
    }
  });

  document.getElementById('ban-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('ban-user-id').value;
    const reason = document.getElementById('ban-reason').value;
    const disableLinks = document.getElementById('ban-disable-links').checked;

    const res = await fetch(`/api/admin/users/${id}/ban`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ reason, disable_links: disableLinks })
    });

    if (res.ok) {
      banModal.classList.remove('show');
      loadUsers();
      loadOverview();
    } else {
      const d = await res.json();
      alert(d.error || 'Failed to ban user');
    }
  });

  // Admin Logout
  adminLogoutBtn.addEventListener('click', async () => {
    try {
      await fetch('/api/auth/logout', { method: 'POST' });
    } finally {
      window.location.href = '/login';
    }
  });

  function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Initial load
  await loadOverview();
});
