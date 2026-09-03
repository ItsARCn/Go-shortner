document.addEventListener('DOMContentLoaded', async () => {
  let currentUser = null;
  let activeFilter = 'all';
  let searchTerm = '';

  const userGreeting = document.getElementById('user-greeting');
  const statQuota = document.getElementById('stat-quota');
  const statReset = document.getElementById('stat-reset');
  const statClicks = document.getElementById('stat-clicks');
  const statActive = document.getElementById('stat-active');
  const statExpired = document.getElementById('stat-expired');
  const linksTbody = document.getElementById('links-tbody');
  const searchInput = document.getElementById('link-search');
  const filterTabs = document.querySelectorAll('.filter-tab');
  const logoutBtn = document.getElementById('logout-btn');

  // Modal elements
  const renewModal = document.getElementById('renew-modal');
  const closeRenewBtn = document.getElementById('close-renew-modal');
  const cancelRenewBtn = document.getElementById('cancel-renew-btn');
  const renewModalCode = document.getElementById('renew-modal-code');
  const renewCodeHidden = document.getElementById('renew-code-hidden');
  const renewForm = document.getElementById('renew-form');
  const renewExpiration = document.getElementById('renew-expiration');
  const confirmRenewBtn = document.getElementById('confirm-renew-btn');

  // 1. Authenticate user
  try {
    const meRes = await fetch('/api/auth/me');
    if (!meRes.ok) {
      window.location.href = '/login';
      return;
    }
    currentUser = await meRes.json();
    userGreeting.textContent = `${currentUser.first_name} ${currentUser.last_name}`;
  } catch (err) {
    window.location.href = '/login';
    return;
  }

  // 2. Fetch Dashboard Data
  async function loadDashboard() {
    try {
      const url = `/api/user/dashboard?status=${activeFilter}&search=${encodeURIComponent(searchTerm)}`;
      const res = await fetch(url);
      if (!res.ok) {
        if (res.status === 401) {
          window.location.href = '/login';
          return;
        }
        linksTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: #f43f5e; padding: 2rem;">Failed to load data.</td></tr>`;
        return;
      }

      const data = await res.json();
      renderStats(data.stats);
      renderLinks(data.links);
    } catch (err) {
      linksTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: #f43f5e; padding: 2rem;">Network error loading dashboard.</td></tr>`;
    }
  }

  function renderStats(stats) {
    statQuota.textContent = `${stats.quota_used} / ${stats.quota_limit}`;
    statReset.textContent = `Resets in ${stats.days_until_reset} days`;
    statClicks.textContent = stats.total_clicks.toLocaleString();
    statActive.textContent = stats.active_links.toLocaleString();
    statExpired.textContent = stats.expired_links.toLocaleString();
  }

  function renderLinks(links) {
    if (!links || links.length === 0) {
      linksTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--text-dim); padding: 3rem;">No shortened links found.</td></tr>`;
      return;
    }

    const rows = links.map(link => {
      const isExpired = link.is_expired;
      const statusBadge = isExpired
        ? `<span class="badge-expired">EXPIRED</span>`
        : `<span class="badge-active">ACTIVE</span>`;

      const expDate = new Date(link.expires_at).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
      });

      const fullShortURL = `${window.location.origin}/${link.short_code}`;

      return `
        <tr>
          <td>
            <a href="${fullShortURL}" target="_blank" class="link-code">/${link.short_code}</a>
          </td>
          <td>
            <div class="dest-url" title="${escapeHTML(link.destination_url)}">${escapeHTML(link.destination_url)}</div>
          </td>
          <td><span class="click-chip">${link.click_count.toLocaleString()}</span></td>
          <td>${statusBadge}</td>
          <td style="color: var(--text-muted); font-size: 0.85rem;">${expDate}</td>
          <td style="text-align: right;">
            <div class="table-actions">
              <button class="action-btn copy-action-btn" data-url="${fullShortURL}">Copy</button>
              ${isExpired ? `<button class="action-btn renew-action-btn" data-code="${link.short_code}">Renew</button>` : ''}
              <button class="action-btn delete-action-btn" data-code="${link.short_code}">Delete</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');

    linksTbody.innerHTML = rows;

    // Attach row events
    document.querySelectorAll('.copy-action-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.getAttribute('data-url'));
        const originalText = btn.textContent;
        btn.textContent = 'Copied!';
        btn.style.color = '#10b981';
        setTimeout(() => {
          btn.textContent = originalText;
          btn.style.color = '';
        }, 1500);
      });
    });

    document.querySelectorAll('.renew-action-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const code = btn.getAttribute('data-code');
        openRenewModal(code);
      });
    });

    document.querySelectorAll('.delete-action-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const code = btn.getAttribute('data-code');
        if (confirm(`Are you sure you want to delete /${code}? Redirection will stop immediately.`)) {
          await deleteLink(code);
        }
      });
    });
  }

  async function deleteLink(code) {
    try {
      const res = await fetch(`/api/user/links/${code}`, { method: 'DELETE' });
      if (res.ok) {
        await loadDashboard();
      } else {
        alert('Failed to delete link');
      }
    } catch (err) {
      alert('Network error while deleting');
    }
  }

  function openRenewModal(code) {
    renewModalCode.textContent = `/${code}`;
    renewCodeHidden.value = code;
    renewModal.classList.add('show');
  }

  function closeRenewModal() {
    renewModal.classList.remove('show');
  }

  closeRenewBtn.addEventListener('click', closeRenewModal);
  cancelRenewBtn.addEventListener('click', closeRenewModal);

  renewForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const code = renewCodeHidden.value;
    const exp = renewExpiration.value;

    confirmRenewBtn.disabled = true;
    confirmRenewBtn.textContent = 'Renewing...';

    try {
      const res = await fetch(`/api/user/links/${code}/renew`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ expiration: exp })
      });

      if (res.ok) {
        closeRenewModal();
        await loadDashboard();
      } else {
        const errData = await res.json();
        alert(errData.error || 'Failed to renew link');
      }
    } catch (err) {
      alert('Network error while renewing');
    } finally {
      confirmRenewBtn.disabled = false;
      confirmRenewBtn.textContent = 'Confirm Renewal';
    }
  });

  // Filter tabs
  filterTabs.forEach(tab => {
    tab.addEventListener('click', () => {
      filterTabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      activeFilter = tab.getAttribute('data-status');
      loadDashboard();
    });
  });

  // Search input debounce
  let searchTimeout = null;
  searchInput.addEventListener('input', () => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      searchTerm = searchInput.value.trim();
      loadDashboard();
    }, 300);
  });

  // Logout
  logoutBtn.addEventListener('click', async () => {
    try {
      await fetch('/api/auth/logout', { method: 'POST' });
    } finally {
      window.location.href = '/login';
    }
  });

  function escapeHTML(str) {
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Initial load
  await loadDashboard();
});
