# 🛡️ Security Policy

**Gorgeous Installer** installs files into Unreal Engine projects, reads package manifests, handles local project paths, and can invoke build tooling. Security issues in these workflows deserve careful, private handling.

> [!IMPORTANT]
> **Please do not disclose vulnerabilities in public issues, discussions, or comments.** Report them privately to **[vulnerabilities@epicnova.net](mailto:vulnerabilities@epicnova.net)**. Avoid sending credentials, signing keys, or private project files unless a secure exchange method has been agreed upon.

---

## 📦 Supported Versions

Security fixes are evaluated for the **currently maintained codebase**. Older builds and independently modified forks may not receive updates.

| Version / channel | Security support |
| --- | --- |
| Current maintained development branch | Reviewed for security fixes |
| Current published installer builds | Evaluated case by case |
| Older releases and third-party forks | No guaranteed security maintenance |

> [!NOTE]
> This table describes the maintenance policy, not a guarantee of response time or a statement that every published build has been independently security-audited.

## 📢 Reporting a Vulnerability

Send a private report to **[vulnerabilities@epicnova.net](mailto:vulnerabilities@epicnova.net)**.

To help maintainers reproduce and assess the issue, include:

1. **Affected version:** Release, commit SHA, operating system, and relevant Unreal Engine version.
2. **Attack surface:** Installer, offline publisher, CLI, archive/package handling, update mechanism, SHA validation, project configuration, or build invocation.
3. **Reproduction:** Minimal steps and a harmless proof of concept, if possible.
4. **Impact:** What an attacker could read, overwrite, execute, or otherwise influence.
5. **Mitigations:** Any known workaround or suggested fix.

Please use synthetic sample projects and redact private paths, API tokens, credentials, and signing material. If sensitive evidence is essential, ask the maintainers to arrange a secure transfer first.

### 🔐 Coordinated disclosure

- We will review incoming reports and work toward reproducing the issue.
- Maintainers may request clarification or additional evidence.
- Fixes and disclosure timing depend on severity, reproducibility, and affected versions.
- Please coordinate public disclosure with the maintainers so affected users have a reasonable opportunity to update.

**No fixed response or remediation SLA is promised.** We will not claim that an issue is resolved until a fix has been verified.

---

## 🔍 Security-Sensitive Areas

| Area | What to watch for |
| --- | --- |
| 📁 **File installation** | Path traversal, symlink escapes, unintended overwrites, and preservation of unrelated project files |
| 📦 **Package and archive handling** | Malformed payloads, unsafe extraction paths, unexpected executable content |
| 🔏 **Integrity verification** | Missing or mismatched SHA-256 manifests, trust boundaries, and tampered downloads |
| ⚙️ **Build execution** | Untrusted arguments, shell execution, toolchain paths, and unexpected compilation |
| 🧩 **Unreal project integration** | Changes to `.uproject`, plugin descriptors, dependency declarations, and activation behavior |
| 🌐 **Updates and distribution** | Download authenticity, version selection, and update integrity |

> [!WARNING]
> SHA-256 checks can detect changes relative to a trusted manifest, but **a checksum alone does not authenticate the publisher**. Do not run installers or build scripts from untrusted sources.

## 🧰 Guidance for Users

- Download installers from official Epic Nova or Gorgeous Things distribution channels.
- Review the origin and contents of third-party packs before installing them.
- Back up important Unreal Engine projects before applying updates.
- Keep Unreal Engine, development toolchains, and installer builds up to date.
- Do not share logs containing secrets, access tokens, or sensitive local paths.

## 🤝 Scope and Related Policies

This document covers **technical vulnerabilities in Gorgeous Installer**. For harassment or community conduct concerns, use the reporting channel in the [Code of Conduct](CODE_OF_CONDUCT.md), not the security vulnerability inbox.

Additional guidance:

- 📘 [Contributor guidelines](.github/CONTRIBUTING.md)
- 👨‍💻 [Development guide](.github/DEVELOPMENT.md)
- 🧾 [Epic Nova Community Ethic Guideline](https://epicnova.net/legal/community-guidelines/long)
- 🔗 [Epic Nova Network Rules](https://epicnova.net/legal/network-rules/long)

---

## 🙌 Thank You

Responsible vulnerability reporting helps protect developers, projects, and the wider **Gorgeous Things** ecosystem. Thank you for helping keep the installer trustworthy.
