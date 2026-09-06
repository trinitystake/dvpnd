# Security policy

## Reporting a vulnerability

Please do **not** open a public issue for security problems.

Use GitHub's private vulnerability reporting on this repository
("Security" tab → "Report a vulnerability"). Reports reach the maintainer
(@trinitystake) directly and stay private until a fix is available.

Include what you can: affected version or commit, environment, reproduction
steps, and impact. You will get an acknowledgement within a few days.

## Supported versions

Only the `main` branch and the latest tagged release receive fixes.

## Scope notes

This software drives WireGuard and V2Ray as external processes and talks to a
public blockchain. Vulnerabilities in those upstream projects should be
reported to them; issues in how this node configures or invokes them belong
here.
