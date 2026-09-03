# GO Shortener - Complete Product & Development Specification

Build a production-ready public URL-shortening service called **GO Shortener**.

Primary domain:

`go.arcn.online`

The application will run on a small Ubuntu VPS with approximately **1 GB RAM and 20 GB storage**, alongside File Browser and other lightweight services.

The application must therefore be **lightweight, efficient, and suitable for running without Docker**.

The project should be designed so it can be deployed directly as a Linux service using `systemd`.

---

# 1. PRODUCT OVERVIEW

GO is a public URL shortener with:

* Anonymous link creation
* User accounts
* Email/password authentication
* Google authentication
* CAPTCHA
* Random short URLs
* Link expiration
* Link renewal
* Link management
* Analytics
* Monthly quotas
* Abuse reporting
* User moderation
* Temporary user restrictions
* Permanent-link requests
* Admin management
* Login/security auditing

The core philosophy is:

> **Simple to use for normal visitors, powerful for registered users, and controllable by administrators.**

Do NOT add unnecessary enterprise features.

The interface should feel modern, clean, fast, minimal, and polished.

---

# 2. IMPORTANT: NO CUSTOM ALIASES

Do NOT allow normal users to choose their own short URL.

For example, users should NOT be able to create:

`go.arcn.online/youtube`

Instead, GO generates a secure random identifier:

`go.arcn.online/K8xP2q`

The identifier should be:

* Cryptographically random
* URL-safe
* Collision-resistant
* 6-8 characters initially
* Case-sensitive if appropriate
* Efficient to generate

The system must check for collisions before inserting.

Custom aliases should not be part of the initial public product.

If desired later, custom aliases can be an administrator-only feature.

---

# 3. HOMEPAGE

The homepage should immediately show the URL shortening interface.

Example:

```text
GO

Shorten your link

[ https://example.com/very/long/url                  ]

Expiration
[ 7 days ▼ ]

[ SHORTEN ]

No account required.
```

For anonymous users, the expiration options must respect the anonymous maximum.

The homepage should also contain subtle links to:

* Login
* Sign Up
* About
* Terms
* Privacy
* Report a Link

Do not make the homepage feel like an admin dashboard.

---

# 4. ANONYMOUS USERS

Visitors must be able to shorten URLs without creating an account.

Anonymous limits:

### Link quota

**15 links per 24-hour period per abuse-control identity.**

Do not rely solely on IP addresses because multiple legitimate users may share an IP.

Use appropriate rate-limiting and abuse-prevention techniques.

### Anonymous expiration

Maximum:

**7 days**

Available options could include:

* 1 hour
* 1 day
* 3 days
* 7 days

Do NOT allow anonymous users to create:

* 30-day links
* 3-month links
* 6-month links
* 1-year links
* Auto-renew links

### Anonymous analytics

Anonymous users do not get a dashboard.

After shortening, show:

```text
Your shortened link:

https://go.arcn.online/K8xP2q

[ COPY ]
```

Do not expose sensitive information.

---

# 5. USER REGISTRATION

Users can create an account.

Registration fields:

```text
First name
Last name
Email
Password
Confirm password
```

Include:

* CAPTCHA
* Terms acceptance
* Email verification if supported
* Strong password requirements
* Rate limiting
* Duplicate email protection

Never store plaintext passwords.

Passwords must be securely hashed using an appropriate modern password hashing algorithm.

---

# 6. GOOGLE LOGIN

Support:

**Continue with Google**

Use **Firebase Authentication** for Google authentication if appropriate.

Firebase should handle authentication.

The GO backend should maintain its own user profile and authorization information.

Suggested user fields:

```text
id
first_name
last_name
email
auth_provider
firebase_uid
role
status
created_at
updated_at
last_login_at
```

Possible authentication providers:

```text
email
google
```

If a user signs in with Google using an email that already has an existing account, handle account linking safely rather than accidentally creating duplicate accounts.

---

# 7. CAPTCHA

Use a modern CAPTCHA solution such as **Cloudflare Turnstile**.

CAPTCHA should be used where appropriate, particularly:

* Registration
* Anonymous shortening
* Login after suspicious activity
* Potentially other abuse-sensitive actions

CAPTCHA must be verified server-side.

Never trust a CAPTCHA result supplied only by the browser.

---

# 8. USER DASHBOARD

Authenticated users get a dashboard.

Example:

```text
GO

Welcome back, User

Links
37 / 100

Clicks
2,841

Active
31

Expired
6

--------------------------------

Your Links

SHORT       DESTINATION       CLICKS      EXPIRES
K8xP2q      github.com/...       42       30 days
a91LmQ      youtube.com/...      318       1 year
Xk72Qa      example.com/...       9       7 days
```

Provide:

* Search
* Sort
* Filter
* Pagination
* Active links
* Expired links
* Deleted links where appropriate

---

# 9. USER QUOTA

Registered users receive:

**100 link creations per calendar month.**

Display usage clearly:

```text
72 / 100 links used

Resets in 12 days
```

The quota should apply to **link creation**, not every time the user edits a link.

For example:

Creating:

`K8xP2q`

uses one quota slot.

Editing its expiration does NOT create another quota charge.

Renewing an expired link also should NOT create another quota charge because it is the same link being renewed.

Admins do not consume normal quotas.

---

# 10. TRUSTED USERS

The system should support administrator-assigned higher quotas.

Examples:

```text
100/month
250/month
500/month
Custom
```

The quota system should be configurable rather than hardcoded throughout the application.

User records should support:

```text
quota_limit
quota_period
```

---

# 11. LINK CREATION

When creating a link:

```text
Destination URL
[ https://example.com ]

Expiration
[ 30 days ▼ ]

[ SHORTEN ]
```

The backend should:

1. Validate the URL
2. Normalize it where appropriate
3. Check abuse/security rules
4. Generate a random short ID
5. Store the link
6. Return the short URL

Store information such as:

```text
id
short_code
destination_url
owner_id
created_at
expires_at
status
auto_renew
click_count
```

---

# 12. LINK EXPIRATION

Supported expiration periods for registered users:

* 1 hour
* 1 day
* 7 days
* 30 days
* 3 months
* 6 months
* 1 year

Maximum normal user expiration:

**1 year**

There must be NO normal "Never expires" option.

---

# 13. EXPIRED LINKS

Expired links should NOT immediately be permanently deleted.

Instead:

```text
Status:
EXPIRED
```

When someone visits an expired short URL:

```text
This link has expired.

This link is no longer active.

Created:
September 3, 2026

Expired:
October 3, 2026
```

Do not reveal unnecessary private owner information.

---

# 14. RENEW EXPIRED LINKS

This is an important feature.

Users should be able to renew their own expired links.

Example:

```text
K8xP2q

Status:
EXPIRED

Expired:
September 3, 2026

[ RENEW LINK ]
```

Clicking Renew:

```text
Renew link

New expiration:

[ 7 days ]
[ 30 days ]
[ 3 months ]
[ 6 months ]
[ 1 year ]

[ RENEW ]
```

The same short code must remain:

```text
go.arcn.online/K8xP2q
```

Do NOT create a new short URL.

Renewing does not consume another monthly link-creation quota.

Normal users cannot renew beyond their maximum allowed expiration.

---

# 15. PERMANENT / AUTO-RENEW LINKS

There should be no normal permanent link option.

Instead, registered users can request permanent/auto-renew access.

Example:

```text
Permanent Link Request

Link:
go.arcn.online/K8xP2q

Reason:

[ This is my portfolio and I need the
  link to remain active. ]

[ SUBMIT REQUEST ]
```

Admin sees:

```text
Permanent Link Requests

User
Link
Destination
Reason
Created
Status

[ APPROVE ]
[ DENY ]
```

If approved:

```text
auto_renew = true
```

The link can remain active indefinitely unless:

* User deletes it
* Admin disables it
* Account is banned
* Link is found to violate policy

Admin must be able to revoke auto-renew.

---

# 16. LINK MANAGEMENT

Each authenticated user should be able to:

* View links
* Copy link
* View destination
* View analytics
* Delete link
* Renew expired link
* Request auto-renew
* View expiration
* Search links
* Sort links

Do not allow users to change the short code.

---

# 17. ANALYTICS

Each link should have an analytics page.

Example:

```text
K8xP2q

Total clicks
2,841

Today
42

This week
318

This month
1,201
```

Possible analytics:

* Click timestamp
* Country
* Referrer
* Device type
* Browser category
* Operating system category

Use privacy-conscious analytics.

Do not unnecessarily store raw IP addresses permanently.

If IP information is required for abuse prevention, store it securely and only for an appropriate retention period.

---

# 18. PUBLIC REDIRECT

Visiting:

```text
https://go.arcn.online/K8xP2q
```

should redirect to the destination.

Normal valid links should redirect quickly.

Do not add an unnecessary intermediate page for every link.

If a destination is suspicious or blocked, show an appropriate warning instead.

---

# 19. LINK REPORTING

Every public short link should have a way to report abuse.

Possible reasons:

```text
Phishing
Malware
Scam
Spam
Illegal content
Other
```

Example:

```text
Report this link

Link:
go.arcn.online/K8xP2q

Reason:
[ Phishing ▼ ]

Additional information:
[ ... ]

[ SUBMIT REPORT ]
```

Prevent report spam with rate limiting.

---

# 20. ABUSE PROTECTION

Because GO is publicly accessible, abuse prevention is critical.

Implement:

* URL validation
* Rate limiting
* CAPTCHA
* Account quotas
* Anonymous quotas
* Suspicious URL detection
* Report system
* User banning
* User timeouts
* Link disabling
* Admin review

Never assume that a URL is safe simply because it is syntactically valid.

---

# 21. ADMIN PANEL

Admin interface:

```text
GO ADMIN
```

Main dashboard:

```text
Users                 1,284
Total Links           8,392
Active Links          7,911
Expired Links           481
Reports                  13
Banned Users               8
Timed Out Users            3
```

Sidebar:

```text
Overview

Users
Links
Reports
Login Records
Permanent Links
Admin Management
System
```

---

# 22. USER MANAGEMENT

User table:

```text
NAME       EMAIL              AUTH       STATUS

John       john@gmail.com     Google     ACTIVE
Alex       alex@gmail.com     Email      BANNED
Sam        sam@gmail.com      Google     ACTIVE
```

Filters:

```text
Authentication:
All
Email
Google

Status:
All
Active
Timed out
Banned
```

Admin can view:

* Name
* Email
* Authentication provider
* Account creation date
* Last login
* Link count
* Current quota
* Status

---

# 23. USER TIMEOUT

Admin can temporarily restrict a user.

Timeout options:

```text
30 seconds
1 minute
5 minutes
30 minutes
1 hour
6 hours
12 hours
1 day
3 days
7 days
Custom
```

The backend should store the actual expiration timestamp.

Example:

```text
status = timed_out
timeout_until = timestamp
```

The user can still log in but cannot create or manage links according to the restriction policy.

Show:

```text
Your account is temporarily restricted.

Reason:
Suspicious link activity.

Remaining:
02h 31m
```

Once the timeout expires, automatically return the account to active status.

---

# 24. BAN SYSTEM

Admin can permanently ban a user.

Ban dialog:

```text
Ban user

Reason:
[ Abuse ]

☑ Disable their active links

[ BAN USER ]
```

When banned:

* User cannot create links
* User cannot use restricted account functions
* Existing links may be disabled depending on admin selection
* Login behavior should clearly indicate the account is banned

Admin can later unban the account.

---

# 25. LINK MODERATION

Admin should be able to:

* Search any link
* View destination
* View owner
* View creation date
* View expiration
* View click count
* Disable link
* Re-enable link
* Delete link
* View reports

Example:

```text
Link:
go.arcn.online/K8xP2q

Owner:
user@example.com

Destination:
https://example.com

Status:
ACTIVE

Reports:
3

[ DISABLE ]
[ DELETE ]
```

---

# 26. LOGIN RECORDS

Create a security/audit page:

```text
Login Records

TIME       ACCOUNT          METHOD      RESULT

11:42 PM   user@gmail.com   Google      SUCCESS
11:39 PM   test@gmail.com   Email       FAILED
11:31 PM   abc@gmail.com    Email       SUCCESS
```

Record:

* Timestamp
* Account
* Authentication method
* Success/failure
* Appropriate security metadata

Allow filtering:

```text
All
Successful
Failed
Email
Google
Admin
```

Do not expose sensitive authentication information.

---

# 27. ADMIN SYSTEM

Use proper role-based access control.

Roles:

### SUPER ADMIN

Can:

* Manage admins
* Remove admins
* Ban users
* Timeout users
* Manage links
* Handle reports
* Approve permanent links
* Change system settings
* View audit logs

### MODERATOR

Can:

* View users
* Timeout users
* Ban users if permitted
* Disable links
* Handle reports

Only SUPER ADMIN can create/remove administrators.

---

# 28. DO NOT USE A UNIVERSAL HARDCODED ADMIN PASSWORD

Do NOT put a universal password directly in source code.

Do NOT implement:

```text
if password == "842009"
```

Instead:

Use environment variables or a secure configuration mechanism.

Example:

```text
GO_ADMIN_EMAIL
GO_ADMIN_PASSWORD_HASH
```

or another secure authentication mechanism.

The actual credentials must never be committed to Git.

Passwords must be hashed.

---

# 29. ADMIN ACCESS

Normal user authentication:

```text
Email / Google
        ↓
Account
        ↓
Role
        ↓
Admin access if authorized
```

If a non-admin user tries to access:

```text
/admin
```

return:

```text
You are not authorized to access the admin panel.
```

Do not reveal unnecessary information about administrator accounts.

---

# 30. SUSPICIOUS ADMIN LOGIN ATTEMPT

If a user attempts to authenticate to an admin-only area and fails authorization, create an audit event.

Example:

```text
Admin access attempt

Account:
user@example.com

Time:
September 3, 2026 11:42 PM

Result:
UNAUTHORIZED
```

The legitimate Super Admin should be able to see these events.

Optionally provide notification functionality later.

Do NOT reveal to the unauthorized user whether any administrator credential was correct.

---

# 31. ADMIN MANAGEMENT

Super Admin can:

```text
Add Administrator
Remove Administrator
Change Role
```

Admin table:

```text
NAME       EMAIL             ROLE

Naitik     admin@example.com SUPER ADMIN
John       john@gmail.com    MODERATOR
```

Only Super Admin can modify this list.

---

# 32. DATABASE

Use **SQLite** initially.

This VPS is small and does not need PostgreSQL.

Database should contain logical tables such as:

```text
users
links
link_clicks
reports
login_records
admin_audit_logs
permanent_link_requests
quota_usage
sessions
```

Use proper indexes.

The database must be stored outside the publicly accessible web directory.

---

# 33. SECURITY

The application must:

* Validate all user input
* Use parameterized SQL
* Prevent SQL injection
* Prevent XSS
* Prevent CSRF where applicable
* Secure authentication
* Hash passwords
* Protect sessions
* Validate uploaded/requested URLs
* Rate-limit sensitive endpoints
* Prevent path traversal
* Prevent unauthorized admin access
* Never expose secrets
* Never commit secrets to Git

---

# 34. API

Structure the backend cleanly.

Example:

```text
/api/auth/*
/api/links/*
/api/users/*
/api/admin/*
/api/reports/*
/api/analytics/*
```

Do not expose admin APIs to normal users.

Every API endpoint must perform server-side authorization.

---

# 35. FRONTEND DESIGN

The UI should be:

* Modern
* Minimal
* Fast
* Responsive
* Mobile-friendly
* Dark-mode friendly
* Accessible

Avoid:

* Excessive gradients
* Huge animations
* Heavy component libraries
* Unnecessary dashboards
* Bloated JavaScript
* Large background videos
* Excessive charts

The website should load quickly even on slow connections.

---

# 36. VISUAL STYLE

GO should have its own identity.

Suggested style:

```text
GO

Simple.
Fast.
Controlled.
```

Use a clean dark interface with subtle borders, compact cards, clear typography, and restrained animations.

The public homepage should be extremely simple.

The dashboard can be more information-dense.

The admin panel should feel like a proper control center.

---

# 37. PUBLIC HOMEPAGE FLOW

Anonymous visitor:

```text
Open GO
   ↓
Paste URL
   ↓
Choose expiration
   ↓
CAPTCHA if required
   ↓
SHORTEN
   ↓
Receive short URL
   ↓
COPY
```

---

# 38. REGISTERED USER FLOW

```text
Sign Up
   ↓
Verify account
   ↓
Dashboard
   ↓
Create link
   ↓
Choose expiration
   ↓
Receive short URL
   ↓
Manage link later
   ↓
View analytics
   ↓
Renew if expired
   ↓
Request auto-renew if necessary
```

---

# 39. EXPIRED LINK LIFECYCLE

A link should follow:

```text
CREATED
   ↓
ACTIVE
   ↓
EXPIRED
   ↓
RENEWED
   ↓
ACTIVE
```

Alternatively:

```text
ACTIVE
   ↓
DISABLED
```

for moderation.

Do not physically delete expired links automatically unless a separate cleanup policy is implemented.

---

# 40. LINK STATES

Support states such as:

```text
ACTIVE
EXPIRED
DISABLED
DELETED
```

Auto-renew links:

```text
ACTIVE + AUTO_RENEW
```

---

# 41. QUOTA LIFECYCLE

Anonymous:

```text
15 / 24 hours
```

Registered:

```text
100 / calendar month
```

Renewals should not consume another creation quota.

Editing existing link properties should not consume creation quota.

Admin-created/moderated actions should not consume user quota.

---

# 42. ADMIN OVERRIDES

Admins should be able to override normal user restrictions where appropriate.

For example:

```text
Change quota
Approve auto-renew
Disable link
Enable link
Timeout
Ban
Unban
```

Every important admin action should create an audit record.

---

# 43. AUDIT LOG

Record important administrative actions:

```text
Admin created user restriction
Admin banned user
Admin disabled link
Admin approved permanent link
Admin changed quota
Admin created administrator
Admin removed administrator
```

Example:

```text
September 3, 2026 11:42 PM

ADMIN:
admin@example.com

ACTION:
DISABLED_LINK

TARGET:
K8xP2q

REASON:
Reported phishing
```

---

# 44. SYSTEM SETTINGS

Admin should eventually be able to configure:

```text
Anonymous quota
Registered quota
Anonymous maximum expiration
Registered maximum expiration
CAPTCHA configuration
Default expiration
Rate limits
```

Do not hardcode these throughout the codebase.

Use configuration values.

---

# 45. PERFORMANCE REQUIREMENTS

The application must be designed for:

**1 GB RAM VPS**

Avoid:

* Docker
* Kubernetes
* Redis unless genuinely required
* PostgreSQL
* Elasticsearch
* Large background workers
* Heavy analytics systems

Prefer:

* SQLite
* In-process caching where useful
* systemd
* lightweight backend
* lightweight frontend

The application should remain usable with several bots and File Browser running on the same VPS.

---

# 46. DEPLOYMENT

The final application should support:

```text
go.arcn.online
```

behind the existing Cloudflare setup.

The application itself should listen only on localhost, for example:

```text
127.0.0.1:3000
```

Cloudflare Tunnel/reverse proxy handles public access.

Do not expose the application directly to the internet unnecessarily.

---

# 47. FILE STRUCTURE

Use a clean project structure.

For example:

```text
/root/go-shortner/

├── backend/
│   ├── src/
│   ├── routes/
│   ├── services/
│   ├── middleware/
│   ├── database/
│   └── ...
│
├── frontend/
│   ├── src/
│   └── ...
│
├── data/
│   └── go.sqlite
│
├── config/
│
├── .env
├── package.json
└── README.md
```

Do not expose:

```text
data/
.env
database files
server source
```

through the web server.

---

# 48. SYSTEMD

Create a service such as:

```text
go-shortener.service
```

It should:

* Start automatically
* Restart after crashes
* Run under an appropriate restricted user where practical
* Use the correct working directory
* Load secrets from a protected environment/config file
* Log through journald

Example conceptual flow:

```text
systemd
   ↓
GO Shortener
   ↓
127.0.0.1:3000
   ↓
Cloudflare Tunnel
   ↓
go.arcn.online
```

---

# 49. BACKUP

The application should make it easy to back up:

```text
SQLite database
configuration
```

Do not include secrets unnecessarily in public project archives.

---

# 50. FUTURE FEATURES

Design the architecture so these can be added later without rewriting the application:

* Custom aliases for trusted/admin users
* API keys
* Public API
* QR code generation
* Advanced analytics
* Link password protection
* Bulk link creation
* Browser extension
* CLI
* Webhooks
* Team accounts

Do NOT implement all of these now.

Keep the initial version focused.

---

# 51. INITIAL VERSION PRIORITY

Build in this order:

### Phase 1

* Homepage
* URL shortening
* Random short codes
* Redirects
* Expiration
* Anonymous quota
* SQLite
* Basic rate limiting

### Phase 2

* Email registration
* Login
* User dashboard
* Registered quota
* Link management
* Expired-link renewal

### Phase 3

* Google authentication
* Firebase integration
* CAPTCHA
* Analytics

### Phase 4

* Reports
* Admin panel
* User moderation
* Timeout
* Ban
* Link moderation
* Login records

### Phase 5

* Permanent-link requests
* Auto-renew
* Admin management
* Audit logs
* System settings

---

# 52. MOST IMPORTANT PRODUCT RULES

Implement these exactly:

```text
Anonymous:
15 links / 24 hours
Maximum expiration: 7 days

Registered:
100 links / calendar month
Maximum expiration: 1 year

Trusted:
Admin-defined higher quota

Permanent:
Only through admin approval

Custom aliases:
NOT AVAILABLE initially

Renew expired links:
YES

Renewal:
Keeps the same short URL
Does NOT consume another creation quota

Authentication:
Email + Google

CAPTCHA:
Required where appropriate

Database:
SQLite

Deployment:
No Docker required

Public:
go.arcn.online

Admin:
Role-based access control
```

---

# 53. IMPORTANT DEVELOPMENT RULE

Do not simply generate the entire project blindly.

First:

1. Design the architecture
2. Define the database schema
3. Define the API
4. Define authentication flow
5. Define security model
6. Define frontend routes
7. Define backend routes
8. Then implement

After implementation:

1. Run tests
2. Test authentication
3. Test quotas
4. Test expiration
5. Test renewal
6. Test redirects
7. Test CAPTCHA
8. Test authorization
9. Test admin permissions
10. Test rate limiting
11. Test malicious input
12. Test deployment

Do not claim something works unless it has actually been tested.

---

## 54. Local Development First → Final Standalone Build

The entire **GO Shortener** project must be developed and fully tested locally first. Do **not** optimize for production packaging during the initial development phase.

### 54.1 Local Development

During development:

* Run the backend locally.
* Run the frontend locally.
* Use SQLite locally.
* Keep frontend and backend easy to modify and debug.
* Use normal development tooling such as Go tooling and, if needed, Node.js/npm for the frontend development environment.
* Make hot reload/development mode convenient where practical.
* Use a local `.env` file for development secrets.
* Use a separate development SQLite database.
* Do not require Docker.
* Do not connect the local development environment to the production VPS or production database.

Example:

```text
GO Shortener
├── frontend/
├── backend/
├── data/
│   └── go-dev.sqlite
├── .env
└── ...
```

The local version must behave as close as reasonably possible to the eventual production version.

### 54.2 Test Everything Locally

Before creating the production build, thoroughly test:

* URL shortening
* Random short-code generation
* Redirects
* Expiration
* Expired-link pages
* Link renewal
* Anonymous quotas
* Registered-user quotas
* Authentication
* Google authentication
* Email authentication
* CAPTCHA
* Dashboard
* Analytics
* Reports
* Admin panel
* User management
* Bans
* Timeouts
* Link moderation
* Permanent/auto-renew requests
* Admin roles
* Audit logs
* Login records
* Security protections
* Rate limiting
* Invalid URLs
* Database errors
* Server restart/recovery
* Concurrent requests
* Mobile/responsive UI

Create automated tests wherever practical and manually test the complete application before packaging.

### 54.3 Production Build Comes Last

**Only after development and testing are complete**, create the production distribution.

The final production version should be packaged as a **standalone executable**, so the production VPS does not need the complete source-code project or development dependencies.

Target architecture:

```text
/opt/go-shortener/
├── go-shortener
├── data/
│   └── go.sqlite
└── .env
```

The frontend should be embedded into the Go executable so the production installation does **not** need a separate frontend directory.

The final production server should ideally require only:

* the compiled GO executable
* SQLite database
* configuration/secrets
* systemd
* existing OS libraries required by the executable

It should **not** require:

* Node.js
* npm
* Docker
* PostgreSQL
* Redis
* the frontend source code
* the backend source code
* development dependencies

### 54.4 Release Builds

Create release binaries for at least:

```text
go-shortener-linux-amd64
go-shortener-linux-arm64
```

The project should also support reproducible builds and preferably provide a GitHub Actions workflow that automatically builds release binaries when a version/tag is created.

### 54.5 Production Installer

At the end of development, create an installer that can:

1. Download the correct release binary.
2. Create the application directory.
3. Create the required data directory.
4. Install the systemd service.
5. Configure permissions.
6. Start the service.
7. Provide basic status/version commands.
8. Support future updates without deleting the SQLite database or configuration.

Example final installation experience:

```bash
curl -fsSL https://go.arcn.online/install.sh | sudo bash
```

The installer must **never overwrite or delete the existing production database or `.env` configuration during an update**.

### 54.6 Development and Production Must Stay Separate

Use separate configurations:

```text
Development
    ↓
Local testing
    ↓
Automated tests
    ↓
Production build
    ↓
Release binary
    ↓
Production VPS
```

Do not use production credentials, Firebase configuration, CAPTCHA secrets, or production databases during local testing.

The final goal is:

> **Build and test the complete GO Shortener locally first. Once everything works correctly, compile it into a lightweight standalone production application that can simply be installed on the VPS.**

### 54.7 GitHub Actions Release Builds

The project **must not require the developer's local device to compile the production binaries**.

Create a GitHub Actions workflow such as:

```text
.github/
└── workflows/
    └── release.yml
```

The workflow should:

1. Trigger when a Git tag such as `v1.0.0` is pushed.
2. Set up the required Go version.
3. Build the application on GitHub Actions.
4. Produce Linux binaries for:

   * `amd64`
   * `arm64`
5. Embed the frontend into the Go binary.
6. Run the automated test suite before creating the release.
7. Fail the release if tests fail.
8. Create a GitHub Release containing the compiled binaries.
9. Generate checksums for the binaries.
10. Keep the source code, build environment, and production binaries separate.

Example:

```text
GitHub repository
       │
       │ git tag v1.0.0
       ▼
GitHub Actions
       │
       ├── Tests
       ├── Build linux/amd64
       ├── Build linux/arm64
       ├── Generate SHA256 checksums
       └── Create GitHub Release
                    │
                    ├── go-shortener-linux-amd64
                    └── go-shortener-linux-arm64
```

The production VPS should **download the appropriate precompiled binary from the GitHub Release** rather than compiling anything itself.

This means your workflow is basically:

**Code locally → test locally → push to GitHub → tag release → GitHub builds it → VPS downloads binary.** 🚀

That's the setup I'd use.

---

# FINAL PRODUCT

The finished application should feel like:

```text
                         GO
                          │
          ┌───────────────┼────────────────┐
          │               │                │
       PUBLIC           USERS            ADMIN
          │               │                │
      Shorten          Dashboard        Control
      CAPTCHA          Analytics        Users
      Redirect         History          Links
      Reports          Renewal          Reports
      Rate limits      Quotas           Bans
                                      Timeouts
                                      Audit logs
                                      Admins
```

The most important goal is:

> **GO should be lightweight enough to comfortably coexist with File Browser, Cloudflare Tunnel, multiple lightweight Discord bots, and other small services on a 1 GB VPS.**

Build it as a clean, maintainable application rather than a quick prototype.
