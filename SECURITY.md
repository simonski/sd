# Security Policy

## Supported versions

Security fixes are provided for the latest released version and `main`.

## Reporting a vulnerability

1. **Do not open a public issue.**
2. Email: **security@simonski.dev** with:
   - affected version/commit
   - reproduction steps
   - impact assessment
   - proposed mitigation (if available)

## Response expectations

- Initial acknowledgement: within **2 business days**
- Triage and severity classification: within **5 business days**
- Remediation target:
  - Critical/High: as soon as practical, prioritized immediately
  - Medium/Low: scheduled in upcoming releases

## Disclosure

- Coordinated disclosure is preferred.
- Reporters will be credited unless they request anonymity.

## Security hardening notes

- `sd` stores local session data under `.sd/`; users should treat this directory as sensitive.
- For shared machines, enforce filesystem permissions and use encrypted home directories.

