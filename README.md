# GO Shortener

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Build & Release](https://img.shields.io/badge/CI%2FCD-GitHub%20Actions-2088FF?style=flat&logo=githubactions)](https://github.com/ItsARCn/Go-shortner/actions)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Binary Size](https://img.shields.io/badge/Binary-~13MB%20(Standalone)-emerald)](https://github.com/ItsARCn/Go-shortner/releases)
[![Memory Usage](https://img.shields.io/badge/RAM%20Footprint-%3C0.5MB-blueviolet)](https://github.com/ItsARCn/Go-shortner)

A high-performance, ultra-lightweight, and fully self-contained URL shortener written in pure Go. Engineered specifically for resource-constrained environments (e.g. 1 GB RAM Ubuntu VPS running alongside other applications), **GO Shortener** compiles into a single executable with an embedded frontend, requires **zero external runtime dependencies**, **zero Docker**, **zero Node/npm**, and uses pure Go SQLite with WAL mode.

---

## Highlights

- **Ultra-Lightweight**: ~13 MB compiled binary; runtime memory footprint stays **under 0.5 MB RAM**.
- **100% Standalone (Go `embed.FS`)**: HTML, CSS, JavaScript, and assets are embedded directly into the Go executable. No external frontend folders required in production.
- **Zero-Compile Deployment (PRD §54.7)**: GitHub Actions builds `linux/amd64` and `linux/arm64` release binaries in the cloud. Your production VPS simply downloads the precompiled binary.
- **Pure Go SQLite (CGO-Free)**: Backed by `modernc.org/sqlite` with Write-Ahead Logging (WAL) and single-writer concurrency controls to prevent database lock contention.
- **Strict SSRF & Recursion Shield**: Blocks loopback, private RFC1918 subnets, cloud metadata IPs (`169.254.169.254`), self-domain recursion, and unsafe URI schemes (`javascript:`, `file:`, `data:`, `ftp:`).
- **Dual-Tier Quota Engine**:
  - *Anonymous*: 15 links / 24 hours (tracked via hashed IP). Expiration clamped to maximum 7 days.
  - *Registered*: 100 links / calendar month. Expiration up to 1 year.
- **Preserved Code Renewal**: Expired links return a branded HTTP 410 Gone page. Registered owners can renew expired links preserving the exact short code with 0 quota penalty.
- **Permanent Links & Auto-Renew**: Users can submit permanent link requests with justifications. Approved links auto-renew indefinitely and never expire.
- **Privacy-Conscious Analytics**: Aggregates total clicks, today/week/month trends, device breakdown, browser share, OS distribution, and top referrers without storing raw visitor IPs.
- **Hybrid Authentication**:
  - Email/password with `bcrypt` (cost 12) stored in SQLite.
  - Google OAuth via Firebase with automatic account linking to existing email accounts.
  - Session tokens stored in HTTP-only, `SameSite=Lax` cookies (`go_session`).
  - Optional Cloudflare Turnstile CAPTCHA protection.
- **Admin Control Center (`/admin`)**:
  - Real-time system overview metrics.
  - User management (search, status filters, promotion/demotion).
  - Duration-based user timeouts (30s to 7d) and permanent bans with bulk link deactivation.
  - Link moderation (disable, enable, delete).
  - Abuse reports review queue (`/report`).
  - Permanent link requests approval queue.
  - Security audit viewer for login and authorization attempts.

---

## Architecture & Production File Layout

When deployed on your VPS, everything is neatly isolated under `/root/Go-shortner/`:

```text
/root/Go-shortner/
├── go-shortener          # Single compiled binary (with embedded frontend)
├── .env                  # Configuration, ports, and secrets (chmod 600)
├── uninstall.sh          # One-click uninstaller script
└── data/
    └── go.sqlite         # Production SQLite database (WAL mode)
```

---

## Quick Start (Local Development)

### Prerequisites
- [Go 1.22+](https://go.dev/dl/) installed locally.

### Setup & Run
```bash
# 1. Clone the repository
git clone https://github.com/ItsARCn/Go-shortner.git
cd Go-shortner

# 2. Copy and customize development environment
cp .env.example .env

# 3. Run all automated tests
cd backend && go test -v ./... && cd ..

# 4. Start local development server
cd backend && go run ./cmd/server/main.go
```
Open [http://localhost:3000](http://localhost:3000) in your browser.

---

## Production Deployment (Zero-Compile on VPS)

In accordance with PRD Section 54.7, **your VPS does not compile anything**. The build pipeline handles compilation via GitHub Actions:

### Step 1: Release via GitHub Actions
Push a Git tag to build the multi-architecture binaries:
```bash
git tag v1.0.0
git push origin v1.0.0
```
GitHub Actions will automatically run the test suite, cross-compile `go-shortener-linux-amd64` and `go-shortener-linux-arm64`, generate SHA256 checksums, and publish a GitHub Release.

---

### Step 2: Install on Production VPS (Ubuntu)

Run the automated installer on your VPS as root:
```bash
curl -fsSL https://raw.githubusercontent.com/ItsARCn/Go-shortner/main/scripts/install.sh | sudo bash
```
*(Or transfer [`scripts/install.sh`](scripts/install.sh) to your server and run `sudo bash install.sh`).*

The installer will:
1. Detect host architecture (`x86_64` vs `aarch64`).
2. Download the matching prebuilt release binary.
3. Automatically create `/root/Go-shortner/` and `/root/Go-shortner/data/`.
4. Generate a secure production `.env` (with random `JWT_SECRET`). **Zero Hardcoded Credentials**: The very first user to register at `https://go.arcn.online/register` (via email or Google) automatically claims the Super Admin role and ownership!
5. Install and start the `systemd` service (`go-shortener.service`).

---

### Step 3: Cloudflare Tunnel Configuration

Configure Cloudflare Tunnel (`cloudflared`) to forward traffic to `127.0.0.1:3000`:
```yaml
tunnel: <TUNNEL_ID>
credentials-file: /root/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: go.arcn.online
    service: http://127.0.0.1:3000
  - service: http_status:404
```
Start or restart your tunnel:
```bash
systemctl restart cloudflared
```

---

## Managing the Service

```bash
# Check service status
systemctl status go-shortener

# View live application logs
journalctl -u go-shortener -f

# Restart application
systemctl restart go-shortener

# Stop application
systemctl stop go-shortener
```

---

## Updating to a New Version

When you release a new version tag (e.g. `v1.0.1`), simply rerun the installer on your VPS:
```bash
sudo bash /root/Go-shortner/install.sh
```
> **Safety Guarantee**: Upgrades **never overwrite** your existing SQLite database (`/root/Go-shortner/data/go.sqlite`) or your `.env` secrets.

---

## Uninstallation

To remove **GO Shortener** from your VPS:

#### Safe Uninstall (Keeps your database and configuration safe):
```bash
/root/Go-shortner/uninstall.sh
```
*Stops and removes the systemd service and deletes the binary, leaving your database and `.env` intact for future use.*

#### Complete Purge (Erases all data and directories):
```bash
/root/Go-shortner/uninstall.sh --purge
```
*Completely removes the service and permanently deletes `/root/Go-shortner/` and all database records.*

---

## Core API Endpoints

### Public Endpoints
| Method | Route | Description |
|---|---|---|
| `POST` | `/api/links/shorten` | Shorten a destination URL (rate-limited, optional auth) |
| `GET` | `/{code}` | Redirect to destination URL (302 Found) or 410 Gone |
| `GET` | `/api/links/{code}/info` | Metadata about a shortened URL |
| `POST` | `/api/reports` | Submit an abuse/phishing report |
| `GET` | `/api/health` | Health check endpoint |

### Authentication Endpoints
| Method | Route | Description |
|---|---|---|
| `POST` | `/api/auth/register` | Register new user with email and password |
| `POST` | `/api/auth/login` | Login with email and password |
| `POST` | `/api/auth/google` | Sign in / link account via Google OAuth (Firebase token) |
| `POST` | `/api/auth/logout` | Clear session cookie |
| `GET` | `/api/auth/me` | Fetch authenticated user profile |

### User Management (Protected)
| Method | Route | Description |
|---|---|---|
| `GET` | `/api/user/dashboard` | Dashboard stats and links list |
| `GET` | `/api/user/links` | Paginated links owned by user |
| `GET` | `/api/user/links/{code}/analytics` | Detailed aggregate click analytics |
| `POST` | `/api/user/links/{code}/renew` | Renew an expired link preserving the short code |
| `POST` | `/api/user/links/{code}/request-permanent` | Request permanent auto-renew status |
| `DELETE`| `/api/user/links/{code}` | Soft delete a link |

### Admin Endpoints (RBAC Protected)
| Method | Route | Min Role | Description |
|---|---|---|---|
| `GET` | `/api/admin/overview` | Moderator | System statistics counters |
| `GET` | `/api/admin/users` | Moderator | Searchable user listing |
| `POST` | `/api/admin/users/{id}/timeout` | Moderator | Apply temporary timeout (30s to 7d) |
| `POST` | `/api/admin/users/{id}/ban` | Moderator | Permanently ban user (optional link deactivation) |
| `POST` | `/api/admin/users/{id}/unban` | Moderator | Restore banned or timed out user |
| `POST` | `/api/admin/users/{id}/role` | Super Admin | Change user role (`super_admin`, `moderator`, `user`) |
| `GET` | `/api/admin/links` | Moderator | Inspect and search all links |
| `POST` | `/api/admin/links/{code}/disable` | Moderator | Disable an abusive link (returns 403) |
| `POST` | `/api/admin/links/{code}/enable` | Moderator | Re-enable a disabled link |
| `DELETE`| `/api/admin/links/{code}` | Moderator | Permanently delete a link |
| `GET` | `/api/admin/reports` | Moderator | Review abuse reports queue |
| `POST` | `/api/admin/reports/{id}/resolve` | Moderator | Mark report reviewed or dismissed |
| `GET` | `/api/admin/permanent-requests` | Moderator | Review permanent link requests queue |
| `POST` | `/api/admin/permanent-requests/{id}/resolve` | Moderator | Approve or reject permanent request |
| `GET` | `/api/admin/login-records` | Super Admin | Audit security logs and unauthorized access attempts |

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PORT` | HTTP server port | `3000` |
| `HOST` | Bind address | `127.0.0.1` |
| `BASE_URL` | Public base URL | `http://localhost:3000` / `https://go.arcn.online` |
| `DB_PATH` | Path to SQLite database file | `/root/Go-shortner/data/go.sqlite` |
| `JWT_SECRET` | Secret key for signing session tokens (32+ chars) | Auto-generated on install |
| `SESSION_DURATION_HOURS` | Session cookie validity duration | `72` |
| `ANONYMOUS_DAILY_QUOTA` | Max links per anonymous IP per 24 hours | `15` |
| `REGISTERED_MONTHLY_QUOTA` | Max links per registered user per calendar month | `100` |
| `ANONYMOUS_MAX_EXPIRATION_DAYS` | Max expiration window for anonymous links | `7` |
| `REGISTERED_MAX_EXPIRATION_DAYS` | Max expiration window for registered links | `365` |
| `TURNSTILE_ENABLED` | Enable Cloudflare Turnstile CAPTCHA | `false` |
| `TURNSTILE_SITE_KEY` | Turnstile public site key | Empty |
| `TURNSTILE_SECRET_KEY` | Turnstile secret key | Empty |
| `FIREBASE_PROJECT_ID` | Firebase project ID for Google OAuth | Empty |

---

## Security Model

1. **IP Privacy**: Visitor IP addresses are hashed using SHA-256 with an identity salt before click analytics aggregation or abuse logs. Raw IPs are never stored in the database.
2. **Password Hashing**: Stored with `bcrypt` (cost 12).
3. **Session Cookies**: Encrypted JWT sessions sent over `HttpOnly`, `SameSite=Lax`, and `Secure` (in production) cookies.
4. **Super Admin Immutability**: Built-in service guardrails strictly prevent moderators from timing out, banning, or demoting `super_admin` accounts.
5. **Intrusion Auditing**: Unauthorized attempts to access `/api/admin/*` endpoints are intercepted, blocked with `403 Forbidden`, and recorded in `login_records` with timestamp, account email, IP hash, and User-Agent.

---

## License

This project is licensed under the [MIT License](LICENSE).
