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

## Notes

- Media URLs require a valid user JWT (header or `?token=`)
- All mutation endpoints are admin-only
- Media library paths must be absolute server paths; relative paths are
  rejected
- The admin directory browser normalizes paths to a clean absolute path, so
  relative input and `..` segments resolve below the filesystem root
- `POST /api/libraries/{id}/open` launches the system file manager on the
  server; it is admin-only and requires the library path to exist
- Passwords are stored as bcrypt hashes only
