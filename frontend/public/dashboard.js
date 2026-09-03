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
  const statBin = document.getElementById('stat-bin');
  const binCountBadge = document.getElementById('bin-count-badge');
  const linksTbody = document.getElementById('links-tbody');
  const searchInput = document.getElementById('link-search');
  const filterTabs = document.querySelectorAll('.filter-tab');
  const logoutBtn = document.getElementById('logout-btn');

  // Renew Modal elements
  const renewModal = document.getElementById('renew-modal');
  const closeRenewBtn = document.getElementById('close-renew-modal');
  const cancelRenewBtn = document.getElementById('cancel-renew-btn');
  const renewModalCode = document.getElementById('renew-modal-code');
  const renewCodeHidden = document.getElementById('renew-code-hidden');
  const renewForm = document.getElementById('renew-form');
  const renewExpiration = document.getElementById('renew-expiration');
  const confirmRenewBtn = document.getElementById('confirm-renew-btn');

  // Permanent Modal elements
  const permModal = document.getElementById('permanent-modal');
  const closePermBtn = document.getElementById('close-permanent-modal');
  const cancelPermBtn = document.getElementById('cancel-permanent-btn');
  const permModalCode = document.getElementById('permanent-modal-code');
  const permCodeHidden = document.getElementById('permanent-code-hidden');
  const permForm = document.getElementById('permanent-form');
  const permReason = document.getElementById('permanent-reason');

  // Analytics Modal elements
  const analyticsModal = document.getElementById('analytics-modal');
  const closeAnalyticsBtn = document.getElementById('close-analytics-modal');
  const analyticsCode = document.getElementById('analytics-code');
  const analyticsDest = document.getElementById('analytics-dest');
  const metricTotal = document.getElementById('metric-total');
  const metricToday = document.getElementById('metric-today');
  const metricWeek = document.getElementById('metric-week');
  const metricMonth = document.getElementById('metric-month');
  const devicesList = document.getElementById('analytics-devices-list');
  const browsersList = document.getElementById('analytics-browsers-list');
  const osList = document.getElementById('analytics-os-list');
  const referrersList = document.getElementById('analytics-referrers-list');

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
    if (statBin) {
      statBin.textContent = (stats.bin_links || 0).toLocaleString();
    }
    if (binCountBadge) {
      if (stats.bin_links > 0) {
        binCountBadge.textContent = stats.bin_links;
        binCountBadge.style.display = 'inline-block';
      } else {
        binCountBadge.style.display = 'none';
      }
    }
  }

  function renderLinks(links) {
    if (!links || links.length === 0) {
      if (activeFilter === 'bin') {
        linksTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--color-stone); padding: 3rem;">The Bin is empty. Deleted links stay here for 7 days before permanent removal.</td></tr>`;
      } else {
        linksTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--color-stone); padding: 3rem;">No shortened links found.</td></tr>`;
      }
      return;
    }

    const rows = links.map(link => {
      const fullShortURL = `${window.location.origin}/${link.short_code}`;

      if (activeFilter === 'bin') {
        const daysLeft = link.days_remaining_in_bin || 7;
        const deletedDate = link.deleted_at ? new Date(link.deleted_at).toLocaleDateString(undefined, {
          month: 'short', day: 'numeric', year: 'numeric'
        }) : 'Recently';

        return `
          <tr>
            <td>
              <span class="link-code" style="opacity: 0.65;">/${escapeHTML(link.short_code)}</span>
            </td>
            <td>
              <div class="dest-url" title="${escapeHTML(link.destination_url)}">${escapeHTML(link.destination_url)}</div>
            </td>
            <td><span class="click-chip">${link.click_count.toLocaleString()}</span></td>
            <td><span class="badge-bin">⏳ ${daysLeft}d left</span></td>
            <td style="color: var(--color-stone); font-size: 0.85rem;">Deleted ${deletedDate}</td>
            <td style="text-align: right;">
              <div class="table-actions">
                <button class="action-btn restore-action-btn" data-code="${link.short_code}">Restore</button>
                <button class="action-btn perm-delete-action-btn" data-code="${link.short_code}">Delete Forever</button>
              </div>
            </td>
          </tr>
        `;
      }

      const isExpired = link.is_expired;
      let statusBadge = isExpired
        ? `<span class="badge-expired">EXPIRED</span>`
        : `<span class="badge-active">ACTIVE</span>`;

      let expText = new Date(link.expires_at).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
      });

      if (link.auto_renew) {
        statusBadge = `<span class="badge-active" style="background:rgba(99,102,241,0.2); color:#818cf8; border-color: rgba(99,102,241,0.3);">PERMANENT</span>`;
        expText = 'Never (Auto-renew)';
      }

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
          <td style="color: var(--color-pearl); font-size: 0.85rem;">${expText}</td>
          <td style="text-align: right;">
            <div class="table-actions">
              <button class="action-btn analytics-action-btn" data-code="${link.short_code}">Analytics</button>
              <button class="action-btn copy-action-btn" data-url="${fullShortURL}">Copy</button>
              ${!link.auto_renew && !isExpired ? `<button class="action-btn req-perm-action-btn" data-code="${link.short_code}" style="color:#818cf8; border-color:rgba(99,102,241,0.3);">Make Permanent</button>` : ''}
              ${isExpired ? `<button class="action-btn renew-action-btn" data-code="${link.short_code}">Renew</button>` : ''}
              <button class="action-btn delete-action-btn" data-code="${link.short_code}">Delete</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');

    linksTbody.innerHTML = rows;

    // Attach row events
    document.querySelectorAll('.restore-action-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const code = btn.getAttribute('data-code');
        btn.disabled = true;
        btn.textContent = 'Restoring...';
        try {
          const res = await fetch(`/api/user/links/${code}/restore`, {
            method: 'POST'
          });
          const data = await res.json();
          if (res.ok) {
            loadDashboard();
          } else {
            alert(data.error || 'Failed to restore link');
            btn.disabled = false;
            btn.textContent = 'Restore';
          }
        } catch (err) {
          alert('Network error restoring link');
          btn.disabled = false;
          btn.textContent = 'Restore';
        }
      });
    });

    document.querySelectorAll('.perm-delete-action-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const code = btn.getAttribute('data-code');
        if (!confirm(`Are you sure you want to permanently erase /${code}? This action cannot be undone.`)) {
          return;
        }
        btn.disabled = true;
        btn.textContent = 'Deleting...';
        try {
          const res = await fetch(`/api/user/links/${code}/permanent`, {
            method: 'DELETE'
          });
          const data = await res.json();
          if (res.ok) {
            loadDashboard();
          } else {
            alert(data.error || 'Failed to permanently delete link');
            btn.disabled = false;
            btn.textContent = 'Delete Forever';
          }
        } catch (err) {
          alert('Network error deleting link');
          btn.disabled = false;
          btn.textContent = 'Delete Forever';
        }
      });
    });

    document.querySelectorAll('.analytics-action-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const code = btn.getAttribute('data-code');
        openAnalyticsModal(code);
      });
    });

    document.querySelectorAll('.req-perm-action-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const code = btn.getAttribute('data-code');
        openPermanentModal(code);
      });
    });

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
        if (confirm(`Move /${code} to the Bin? You can recover it within 7 days.`)) {
          await deleteLink(code);
        }
      });
    });
  }

  async function openAnalyticsModal(code) {
    analyticsCode.textContent = `/${code}`;
    analyticsDest.textContent = 'Loading analytics...';
    metricTotal.textContent = '-';
    metricToday.textContent = '-';
    metricWeek.textContent = '-';
    metricMonth.textContent = '-';
    devicesList.innerHTML = '<div style="color:var(--text-dim); font-size:0.8rem;">Loading...</div>';
    browsersList.innerHTML = '<div style="color:var(--text-dim); font-size:0.8rem;">Loading...</div>';
    osList.innerHTML = '<div style="color:var(--text-dim); font-size:0.8rem;">Loading...</div>';
    referrersList.innerHTML = '<div style="color:var(--text-dim); font-size:0.8rem;">Loading...</div>';

    analyticsModal.classList.add('show');

    try {
      const res = await fetch(`/api/user/links/${code}/analytics`);
      if (!res.ok) {
        analyticsDest.textContent = 'Failed to load analytics data';
        return;
      }
      const data = await res.json();
      analyticsDest.textContent = `Destination: ${data.destination_url}`;
      metricTotal.textContent = data.total_clicks.toLocaleString();
      metricToday.textContent = data.clicks_today.toLocaleString();
      metricWeek.textContent = data.clicks_this_week.toLocaleString();
      metricMonth.textContent = data.clicks_this_month.toLocaleString();

      devicesList.innerHTML = renderDistributionList(data.devices);
      browsersList.innerHTML = renderDistributionList(data.browsers);
      osList.innerHTML = renderDistributionList(data.operating_systems);
      referrersList.innerHTML = renderDistributionList(data.top_referrers);
    } catch (err) {
      analyticsDest.textContent = 'Network error fetching analytics';
    }
  }

  function renderDistributionList(items) {
    if (!items || items.length === 0) {
      return `<div style="color:var(--text-dim); font-size:0.8rem; padding: 0.5rem 0;">No click data recorded yet.</div>`;
    }

    return items.map(item => `
      <div class="analytics-row">
        <div class="analytics-row-header">
          <span>${escapeHTML(item.name)}</span>
          <span style="font-family:var(--font-mono); color:var(--text-muted);">${item.count} (${item.percentage.toFixed(0)}%)</span>
        </div>
        <div class="analytics-bar-bg">
          <div class="analytics-bar-fill" style="width: ${item.percentage}%;"></div>
        </div>
      </div>
    `).join('');
  }

  closeAnalyticsBtn.addEventListener('click', () => {
    analyticsModal.classList.remove('show');
  });

  // Permanent Request Modal
  function openPermanentModal(code) {
    permModalCode.textContent = `/${code}`;
    permCodeHidden.value = code;
    permReason.value = '';
    permModal.classList.add('show');
  }

  closePermBtn.addEventListener('click', () => permModal.classList.remove('show'));
  cancelPermBtn.addEventListener('click', () => permModal.classList.remove('show'));

  permForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const code = permCodeHidden.value;
    const reason = permReason.value.trim();

    try {
      const res = await fetch(`/api/user/links/${code}/request-permanent`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason })
      });
      const data = await res.json();
      if (res.ok) {
        permModal.classList.remove('show');
        alert(data.message || 'Request submitted successfully');
      } else {
        alert(data.error || 'Failed to submit permanent link request');
      }
    } catch (err) {
      alert('Network error while requesting permanent link');
    }
  });

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
