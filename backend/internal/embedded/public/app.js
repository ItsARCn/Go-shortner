document.addEventListener('DOMContentLoaded', async () => {
  const form = document.getElementById('shorten-form');
  const urlInput = document.getElementById('url-input');
  const expirationSelect = document.getElementById('expiration-select');
  const shortenBtn = document.getElementById('shorten-btn');
  const errorBanner = document.getElementById('error-banner');
  const resultBox = document.getElementById('result-box');
  const resultUrl = document.getElementById('result-url');
  const copyBtn = document.getElementById('copy-btn');
  const resultExpires = document.getElementById('result-expires');
  const navLinksContainer = document.querySelector('.nav-links');
  const cardNote = document.querySelector('.card-footer-note');

  let currentUser = null;

  // Check auth state
  try {
    const res = await fetch('/api/auth/me');
    if (res.ok) {
      currentUser = await res.json();
      // Update nav
      if (navLinksContainer) {
        navLinksContainer.innerHTML = `
          <a href="/dashboard" class="nav-link nav-btn">Dashboard (${currentUser.first_name})</a>
        `;
      }
      // Add registered options to expiration dropdown
      expirationSelect.innerHTML = `
        <option value="1h">1 hour</option>
        <option value="1d">1 day</option>
        <option value="3d">3 days</option>
        <option value="7d">7 days</option>
        <option value="30d" selected>30 days (default)</option>
        <option value="90d">3 months</option>
        <option value="180d">6 months</option>
        <option value="365d">1 year (Maximum)</option>
      `;
      if (cardNote) {
        cardNote.textContent = `Logged in as ${currentUser.email} • 100 links / month`;
      }
    }
  } catch (err) {
    // Keep anonymous defaults
  }

  function showError(msg) {
    errorBanner.textContent = msg;
    errorBanner.classList.add('show');
    resultBox.classList.remove('show');
  }

  function clearError() {
    errorBanner.textContent = '';
    errorBanner.classList.remove('show');
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    clearError();

    const rawUrl = urlInput.value.trim();
    if (!rawUrl) {
      showError('Please enter a destination URL.');
      urlInput.focus();
      return;
    }

    shortenBtn.disabled = true;
    shortenBtn.textContent = 'Shortening...';

    try {
      const payload = {
        url: rawUrl,
        expiration: expirationSelect.value
      };

      const res = await fetch('/api/links/shorten', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      const data = await res.json();

      if (!res.ok) {
        showError(data.error || 'Failed to shorten URL. Please try again.');
        return;
      }

      // Display result
      resultUrl.value = data.short_url;
      const expDate = new Date(data.expires_at);
      resultExpires.textContent = `Expires on ${expDate.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}`;
      resultBox.classList.add('show');

      // Reset copy button
      copyBtn.textContent = 'Copy';
      copyBtn.classList.remove('copied');

      resultBox.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } catch (err) {
      showError('Network error. Unable to reach server.');
    } finally {
      shortenBtn.disabled = false;
      shortenBtn.textContent = 'Shorten';
    }
  });

  copyBtn.addEventListener('click', async () => {
    if (!resultUrl.value) return;
    try {
      await navigator.clipboard.writeText(resultUrl.value);
      copyBtn.textContent = 'Copied!';
      copyBtn.classList.add('copied');
      setTimeout(() => {
        copyBtn.textContent = 'Copy';
        copyBtn.classList.remove('copied');
      }, 2500);
    } catch (err) {
      resultUrl.select();
      document.execCommand('copy');
      copyBtn.textContent = 'Copied!';
      copyBtn.classList.add('copied');
    }
  });
});
