# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.8.x   | :white_check_mark: |
| < 0.8   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in hCTF, please report it responsibly.

### How to Report

**Do NOT open a public issue for security vulnerabilities.**

Instead, please report via one of these channels:

1. **GitHub Private Vulnerability Reporting**: [Report a vulnerability](../../security/advisories/new)

### What to Include

When reporting a vulnerability, please include:

- **Description**: Clear description of the vulnerability
- **Impact**: What could an attacker accomplish?
- **Reproduction**: Step-by-step instructions to reproduce
- **Version**: hCTF version affected
- **Environment**: Operating system, deployment method
- **Possible fix**: If you have suggestions (optional)

### Response Timeline

| Action | Timeframe |
| ------ | --------- |
| Acknowledgment | Within 48 hours |
| Initial assessment | Within 7 days |
| Fix released | Depends on severity (see below) |
| Public disclosure | After fix released + users notified |

### Severity Classifications

| Severity | Examples | Fix Timeline |
|----------|----------|--------------|
| Critical | RCE, SQL injection, auth bypass | 7 days |
| High | XSS, CSRF, privilege escalation | 14 days |
| Medium | Information disclosure, DoS | 30 days |
| Low | Defense in depth improvements | Next release |

### Security Best Practices for Deployments

1. **JWT Secret**: Always set a unique JWT secret in production
   ```bash
   export JWT_SECRET=$(openssl rand -base64 32)
   ```

2. **Database**: Keep database files outside web root with proper permissions
   ```bash
   chmod 600 hctf.db
   ```

3. **HTTPS**: Use HTTPS in production (terminate at reverse proxy)

4. **Updates**: Keep hCTF updated to the latest version

5. **Backups**: Regular database backups before updates

### Security Features

hCTF includes these security measures:

- Passwords hashed with bcrypt (cost 12)
- SQL injection prevention via parameterized queries
- XSS prevention via Go's html/template auto-escaping
- CSRF protection via SameSite cookies
- Secure password reset tokens (32-byte random, 30min expiry)
- Constant-time password comparison

### Acknowledgments

We thank the following security researchers for responsible disclosure:

- *None yet - be the first!*

## Security Update Policy

Security updates are released as patch versions (e.g., 0.5.1 for 0.5.x). Users should:

1. Subscribe to GitHub releases for notifications
2. Monitor the [CHANGELOG](CHANGELOG.md) for security-related fixes
3. Update promptly when security patches are released
