# Security

## Reporting a vulnerability

If you believe you have found a security issue (for example, path escape from snapshot roots, unsafe file exposure via the API), please report it **privately** to the repository maintainers (open a security advisory on GitHub if enabled, or contact via the profile/email shown on the maintainer account).

Please do **not** open a public issue for undisclosed vulnerabilities.

## Scope notes

- `timedog-server` is intended for **local/trusted** use. Do not expose it to the open internet without authentication and TLS.
- Content endpoints are constrained to paths under the snapshot roots recorded for a session; report any bypass you find.
