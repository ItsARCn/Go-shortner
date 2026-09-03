#!/usr/bin/env bash
set -e

# ==============================================================================
# GO Shortener - Automated Production VPS Installer
# Target Server: Ubuntu VPS (Ultra-lightweight, systemd, Zero Docker)
# ==============================================================================

REPO="ItsARCn/Go-shortner"
INSTALL_DIR="/opt/go-shortener"
DATA_DIR="${INSTALL_DIR}/data"
SERVICE_FILE="/etc/systemd/system/go-shortener.service"

echo "=========================================="
echo "    GO Shortener Production Installer     "
echo "=========================================="

# 1. Require Root / Sudo
if [ "$(id -u)" -ne 0 ]; then
    echo "[ERROR] This installer must be run as root or with sudo." >&2
    exit 1
fi

# 2. Detect Host Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64|amd64)
        BINARY_NAME="go-shortener-linux-amd64"
        ;;
    aarch64|arm64)
        BINARY_NAME="go-shortener-linux-arm64"
        ;;
    *)
        echo "[ERROR] Unsupported system architecture: ${ARCH}" >&2
        exit 1
        ;;
esac

echo "[INFO] Detected architecture: ${ARCH} (${BINARY_NAME})"

# 3. Create Directories
mkdir -p "${INSTALL_DIR}"
mkdir -p "${DATA_DIR}"
chmod 755 "${INSTALL_DIR}"
chmod 750 "${DATA_DIR}"

# 4. Fetch or Install Binary
TMP_BIN="/tmp/${BINARY_NAME}"

if [ -f "$1" ]; then
    echo "[INFO] Using supplied local binary: $1"
    cp "$1" "${INSTALL_DIR}/go-shortener"
else
    echo "[INFO] Fetching latest release binary from GitHub (${REPO})..."
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
    curl -fsSL -L "${DOWNLOAD_URL}" -o "${TMP_BIN}" || {
        echo "[ERROR] Failed to download prebuilt binary from GitHub Release."
        echo "Ensure a GitHub Release exists with tag 'v*' or supply a local binary: ./install.sh <binary_path>"
        exit 1
    }
    mv "${TMP_BIN}" "${INSTALL_DIR}/go-shortener"
fi

chmod +x "${INSTALL_DIR}/go-shortener"

# 5. Safe Configuration Setup (NEVER overwrite existing .env on update)
ENV_FILE="${INSTALL_DIR}/.env"
if [ ! -f "${ENV_FILE}" ]; then
    echo "[INFO] Creating initial production .env configuration..."
    RANDOM_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 40)
    ADMIN_INIT_PW="AdminSecurePass_$(head -c 12 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 12)!"

    cat << ENVCONF > "${ENV_FILE}"
# Server Configuration
PORT=3000
HOST=127.0.0.1
BASE_URL=https://go.arcn.online
APP_ENV=production

# Database
DB_PATH=${DATA_DIR}/go.sqlite

# Security & Sessions
JWT_SECRET=${RANDOM_SECRET}
SESSION_DURATION_HOURS=72

# Quotas & Limits
ANONYMOUS_DAILY_QUOTA=15
REGISTERED_MONTHLY_QUOTA=100
ANONYMOUS_MAX_EXPIRATION_DAYS=7
REGISTERED_MAX_EXPIRATION_DAYS=365

# Super Admin Initial Bootstrap
ADMIN_BOOTSTRAP_EMAIL=admin@arcn.online
ADMIN_BOOTSTRAP_PASSWORD=${ADMIN_INIT_PW}

# Cloudflare Turnstile CAPTCHA (Optional - add keys if required)
TURNSTILE_ENABLED=false
TURNSTILE_SITE_KEY=
TURNSTILE_SECRET_KEY=

# Firebase Auth (Optional - for Google OAuth)
FIREBASE_PROJECT_ID=
ENVCONF

    chmod 600 "${ENV_FILE}"
    echo "---------------------------------------------------------------"
    echo "[IMPORTANT] Bootstrapped Admin Credentials Created:"
    echo "  Email:    admin@arcn.online"
    echo "  Password: ${ADMIN_INIT_PW}"
    echo "  Config:   ${ENV_FILE}"
    echo "  Please change this password upon first login!"
    echo "---------------------------------------------------------------"
else
    echo "[INFO] Existing .env found at ${ENV_FILE}. Preserving existing configuration and secrets."
fi

# 6. Install Systemd Service
echo "[INFO] Installing systemd service unit..."
cat << SERVICEUNIT > "${SERVICE_FILE}"
[Unit]
Description=GO Shortener - High Performance URL Shortener
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/go-shortener
Restart=always
RestartSec=3
LimitNOFILE=65535

ProtectSystem=full
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SERVICEUNIT

systemctl daemon-reload
systemctl enable go-shortener.service
systemctl restart go-shortener.service

echo ""
echo "==============================================================="
echo "   GO Shortener Installed & Started Successfully!             "
echo "==============================================================="
echo "Service Status:  systemctl status go-shortener"
echo "Service Logs:    journalctl -u go-shortener -f"
echo "Local Endpoint:  http://127.0.0.1:3000"
echo "Public Domain:   https://go.arcn.online (via Cloudflare Tunnel)"
echo "==============================================================="
