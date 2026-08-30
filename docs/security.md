# Security Policy

## Supported versions

VideoCMS is under active development (v0.x). Security fixes are applied to the
`main` branch and released with the next tag. For production use, always run the
latest commit or tagged release.

## Reporting a vulnerability

Please **do not** open a public issue for security problems. Report them privately
by opening a GitHub issue with the “security” label or contacting the maintainers
directly. Include:

- Affected version/commit
- Steps to reproduce
- Impact description

## Deployment hardening checklist

- Set a strong, random `JWT_SECRET` (e.g. `openssl rand -hex 32`)
- Specify the initial admin with `ADMIN_USERNAME` / `ADMIN_PASSWORD` and change
  the password after first login
- Expose the service only over HTTPS (reverse proxy with TLS) for anything beyond
  a trusted LAN
- Keep PostgreSQL credentials private; don’t reuse the app database for other apps
- Use the per-user hidden-path filters if some folders should not be visible
- Use admin content controls (title blocking and library blocking) to hide
  content globally without deleting files
- Restrict media library paths to what the process actually needs to read
- If you enable the DLNA/UPnP media server (`DLNA_ENABLED=1`), set
  `DLNA_ALLOWED_IPS` to the LAN subnets allowed to browse/stream; leave it
  empty only on a fully trusted network
- For SAML SSO, keep `SAML_SP_KEY` private (readable only by the service user)
  and make sure the IdP metadata URL is served over HTTPS
- For SMTP notifications, prefer TLS (STARTTLS on 587/25, implicit TLS on 465)
  so credentials are never sent in plaintext

## Notes

- Media URLs require a valid user JWT (header or `?token=`)
- All mutation endpoints are admin-only
- Public share links use unguessable tokens with expiry, optional bcrypt
  password and a domain allow-list; casting reuses a 1-hour video share so the
  user JWT never leaves the browser
- Webhook deliveries are signed with HMAC-SHA256
  (`X-Videocms-Signature`); verify the signature before acting on events
- SAML assertions are verified with the IdP certificate (signature, conditions,
  audience); SSO users bind to `users.oauth_sub` (`oidc:`/`saml:` prefix) and
  are subject to the same role checks as local users
- Media library paths must be absolute server paths; relative paths are
  rejected
- The admin directory browser normalizes paths to a clean absolute path, so
  relative input and `..` segments resolve below the filesystem root
- Upload and yt-dlp download target folders must be existing absolute server
  directories; paths containing `..` are rejected, and uploaded filenames are
  reduced to their base name before anything touches disk
- Remux downloads (`GET /api/videos/{id}/download/remux`) only read the video
  and its subtitle files and never write to the server
- The yt-dlp background worker is admin-only and runs jobs sequentially, one
  URL at a time
- `POST /api/libraries/{id}/open` launches the system file manager on the
  server; it is admin-only and requires the library path to exist
- Passwords are stored as bcrypt hashes only
