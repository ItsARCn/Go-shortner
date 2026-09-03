#!/usr/bin/env bash
set -e

# ==============================================================================
# GO Shortener - Automated Production VPS Installer
# Target Server: Ubuntu VPS (Ultra-lightweight, systemd, Zero Docker)
# Default Location: /root/Go-shortner
# ==============================================================================

REPO="ItsARCn/Go-shortner"
INSTALL_DIR="${GO_INSTALL_DIR:-/root/Go-shortner}"
DATA_DIR="${INSTALL_DIR}/data"
SERVICE_FILE="/etc/systemd/system/go-shortener.service"

# Handle --uninstall and --purge shortcuts directly
if [ "$1" = "--uninstall" ] || [ "$1" = "--purge" ] || [ "$2" = "--purge" ]; then
    echo "=========================================="
    echo "    GO Shortener Uninstaller              "
    echo "=========================================="
    if [ "$(id -u)" -ne 0 ]; then
        echo "[ERROR] Uninstaller must be run as root or with sudo." >&2
        exit 1
    fi
    if systemctl is-active --quiet go-shortener.service 2>/dev/null; then
        echo "[INFO] Stopping go-shortener service..."
        systemctl stop go-shortener.service || true
    fi
    if systemctl is-enabled --quiet go-shortener.service 2>/dev/null; then
        echo "[INFO] Disabling go-shortener service..."
        systemctl disable go-shortener.service || true
    fi
    if [ -f "${SERVICE_FILE}" ]; then
        echo "[INFO] Removing systemd service unit..."
        rm -f "${SERVICE_FILE}"
        systemctl daemon-reload
        systemctl reset-failed || true
    fi
    if [ "$1" = "--purge" ] || [ "$2" = "--purge" ]; then
        echo "[INFO] Purging ${INSTALL_DIR} completely..."
        rm -rf "${INSTALL_DIR}"
        echo "[SUCCESS] GO Shortener and all data have been completely purged."
    else
        rm -f "${INSTALL_DIR}/go-shortener"
        echo "[SUCCESS] Safe uninstall complete. Configuration and database in ${INSTALL_DIR} kept."
    fi
    exit 0
fi

echo "=========================================="
echo "    GO Shortener Production Installer     "
echo "=========================================="
echo "[INFO] Target Directory: ${INSTALL_DIR}"

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

# 3. Automatically Create Target Directories
mkdir -p "${INSTALL_DIR}"
mkdir -p "${DATA_DIR}"
chmod 700 "${INSTALL_DIR}"
chmod 700 "${DATA_DIR}"

# 4. Fetch or Install Binary
TMP_BIN="/tmp/${BINARY_NAME}"

if [ -f "$1" ] && [ "$1" != "--uninstall" ]; then
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

# 4b. Embed uninstaller script directly to target directory
cat << 'UNINSTALL_SCRIPT' > "${INSTALL_DIR}/uninstall.sh"
#!/usr/bin/env bash
set -e

INSTALL_DIR="${GO_INSTALL_DIR:-/root/Go-shortner}"
SERVICE_FILE="/etc/systemd/system/go-shortener.service"

echo "=========================================="
echo "    GO Shortener Uninstaller              "
echo "=========================================="

# 1. Require Root / Sudo
if [ "$(id -u)" -ne 0 ]; then
    echo "[ERROR] This uninstaller must be run as root or with sudo." >&2
    exit 1
fi

# 2. Stop and Disable Systemd Service
if systemctl is-active --quiet go-shortener.service 2>/dev/null; then
    echo "[INFO] Stopping go-shortener service..."
    systemctl stop go-shortener.service || true
fi

if systemctl is-enabled --quiet go-shortener.service 2>/dev/null; then
    echo "[INFO] Disabling go-shortener service..."
    systemctl disable go-shortener.service || true
fi

# 3. Remove Systemd Service File
if [ -f "${SERVICE_FILE}" ]; then
    echo "[INFO] Removing systemd unit: ${SERVICE_FILE}..."
    rm -f "${SERVICE_FILE}"
    systemctl daemon-reload
    systemctl reset-failed || true
fi

# 4. Handle Directory and Data Removal
PURGE=false
if [ "$1" = "--purge" ] || [ "$1" = "-f" ]; then
    PURGE=true
fi

if [ -d "${INSTALL_DIR}" ]; then
    if [ "${PURGE}" = true ]; then
        echo "[INFO] Purging entire directory ${INSTALL_DIR} (including database and .env)..."
        rm -rf "${INSTALL_DIR}"
        echo "[SUCCESS] GO Shortener and all associated data have been completely purged."
    else
        echo "[INFO] Removing binary ${INSTALL_DIR}/go-shortener..."
        rm -f "${INSTALL_DIR}/go-shortener"
        echo ""
        echo "---------------------------------------------------------------"
        echo "[NOTE] Safe Uninstall Complete!"
        echo "  - The systemd service was removed."
        echo "  - The executable was removed."
        echo "  - Your database (${INSTALL_DIR}/data/go.sqlite) and (.env) were KEPT safely."
        echo ""
        echo "If you want to completely erase the database and configuration, run:"
        echo "  rm -rf ${INSTALL_DIR}"
        echo "Or rerun uninstall with purge:"
        echo "  /root/Go-shortner/uninstall.sh --purge"
        echo "---------------------------------------------------------------"
    fi
fi

echo ""
echo "==============================================================="
echo "   GO Shortener Uninstalled Successfully                      "
echo "==============================================================="
UNINSTALL_SCRIPT
chmod +x "${INSTALL_DIR}/uninstall.sh"

# 5. Safe Configuration Setup (NEVER overwrite existing .env on update)
ENV_FILE="${INSTALL_DIR}/.env"
if [ ! -f "${ENV_FILE}" ]; then
    echo "[INFO] Creating initial production .env configuration..."
    RANDOM_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 40)
    cat << ENVCONF > "${ENV_FILE}"
# Server Configuration
PORT=3000
HOST=127.0.0.1
BASE_URL=https://go.arcn.online
APP_ENV=production

# Database (Strictly inside /root/Go-shortner/data)
DB_PATH=${DATA_DIR}/go.sqlite

# Security & Sessions
JWT_SECRET=${RANDOM_SECRET}
SESSION_DURATION_HOURS=72

# Quotas & Limits
ANONYMOUS_DAILY_QUOTA=15
REGISTERED_MONTHLY_QUOTA=100
ANONYMOUS_MAX_EXPIRATION_DAYS=7
REGISTERED_MAX_EXPIRATION_DAYS=365

# Cloudflare Turnstile CAPTCHA (Optional - add keys if required)
TURNSTILE_ENABLED=false
TURNSTILE_SITE_KEY=
TURNSTILE_SECRET_KEY=

# Firebase Web App Configuration (For Google OAuth Sign-In)
FIREBASE_API_KEY=
FIREBASE_AUTH_DOMAIN=
FIREBASE_PROJECT_ID=
FIREBASE_STORAGE_BUCKET=
FIREBASE_MESSAGING_SENDER_ID=
FIREBASE_APP_ID=
ENVCONF

    chmod 600 "${ENV_FILE}"
    echo "---------------------------------------------------------------"
    echo "[IMPORTANT] First User Super Admin Claim Active:"
    echo "  Open https://go.arcn.online/register and create your account."
    echo "  The very first user to register (via email or Google)"
    echo "  automatically becomes the Super Administrator!"
    echo "  Config File: ${ENV_FILE}"
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
echo "Install Path:    ${INSTALL_DIR}"
echo "Binary:          ${INSTALL_DIR}/go-shortener"
echo "Database:        ${DATA_DIR}/go.sqlite"
echo "Config:          ${ENV_FILE}"
echo "Service Status:  systemctl status go-shortener"
echo "Service Logs:    journalctl -u go-shortener -f"
echo "Local Endpoint:  http://127.0.0.1:3000"
echo "Public Domain:   https://go.arcn.online (via Cloudflare Tunnel)"
echo ""
echo "To Uninstall:    ${INSTALL_DIR}/uninstall.sh"
echo "To Purge All:    ${INSTALL_DIR}/uninstall.sh --purge"
echo "==============================================================="
