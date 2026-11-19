# Security Policy

## Supported Versions

Security fixes are provided for the latest release of Melisa. Please upgrade to
the latest version before reporting an issue. Older releases are not patched
separately.

## Reporting a Vulnerability

Please do not disclose potential vulnerabilities in public GitHub issues.

Use [GitHub's private vulnerability reporting](https://github.com/snmp161/melisa/security/advisories/new)
to report a vulnerability privately.

Please include:

- the affected Melisa version or commit;
- operating system, architecture, and kernel version;
- relevant command-line options and collection profile;
- steps to reproduce the issue;
- expected and actual behavior;
- potential security impact.

Do not include private keys or other sensitive configuration data. Reports and
logs may contain sensitive host, process, and network information; redact them
unless specific details are strictly necessary to reproduce the issue.

## Scope

Melisa may require elevated privileges for BCC/eBPF collection. Review report
contents before sharing them and validate suggested tuning commands before
applying them to production systems. Melisa does not apply recommendations
automatically.
