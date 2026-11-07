# Contributing to Dipper

Thanks for helping. This page covers the legal side of contributing and the
checks a change is expected to pass.

## License of contributions

Dipper is released under the MIT License; see [LICENSE](LICENSE). By submitting
a contribution you agree that it is licensed under the MIT License, and you
certify the [Developer Certificate of Origin 1.1](https://developercertificate.org/).

Add a sign-off line to every commit:

```sh
git commit -s -m "area: what the user now sees"
```

This appends:

```text
Signed-off-by: Your Name <you@example.com>
```

Use a real name and a reachable address. No separate contributor license
agreement is required, and you keep the copyright in your contribution.

## Before changing code

Read [AGENTS.md](AGENTS.md), [LESSONS.md](LESSONS.md), and
[docs/code-standards.md](docs/code-standards.md). The subsystem pages under
[docs/subsystems](docs/subsystems/README.md) describe the invariants a change
must preserve.

## What a change needs

- For code changes, a test that fails without the change, judged with
  [cmd/redcheck](cmd/redcheck). Documentation-only changes need no new test.
- `scripts/build.ps1 -Test` on Windows, or `scripts/build.sh --test` elsewhere.
- Documentation updated with the behavior it describes.
- User-visible strings remain Chinese; documentation and comments are English.

## Reporting a problem

Open an issue at <https://github.com/steampixel007/Dipper/issues>. Do not
include credentials, personal paths, or copyrighted media in a report.
