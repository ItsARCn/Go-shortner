#!/usr/bin/env bash
set -e

# ==============================================================================
# GO Shortener - Production VPS Uninstaller
# ==============================================================================

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
        # Remove only the binary, leaving data and .env intact
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
        echo "  ./uninstall.sh --purge"
        echo "---------------------------------------------------------------"
    fi
else
    echo "[INFO] Install directory ${INSTALL_DIR} does not exist."
fi

echo ""
echo "==============================================================="
echo "   GO Shortener Uninstalled Successfully                      "
echo "==============================================================="
