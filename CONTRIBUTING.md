# Contributing to the Kaiten CLI

Thank you for contributing to the Kaiten CLI.

This project is licensed under the Apache License, Version 2.0.

## Developer Certificate of Origin

Kaiten uses the Developer Certificate of Origin 1.1 (DCO) for contributions.

Every commit must include a `Signed-off-by` trailer matching the commit author.

Use:

```bash
git commit -s -m "Describe your change"
```

This produces:

```text
Signed-off-by: Jane Doe <jane@example.com>
```

`git commit -s` is a **DCO sign-off**: it certifies the contribution under
[DCO.md](./DCO.md). `git commit -S`, with a capital S, is a **cryptographic
commit signature** made with a GPG or SSH key. They are not the same thing, and
the sign-off is what this project requires. There is nothing to install.

### Fixing a missing sign-off

A check named **DCO** verifies every commit of every pull request, and it is the
final word. If it fails, it names the commits to fix.

For the latest commit:

```bash
git commit --amend --signoff --no-edit
git push --force-with-lease
```

For several commits, sign off every commit of your branch at once:

```bash
git rebase --signoff origin/main
git push --force-with-lease
```

## Commit messages and pull request titles

Versions and the changelog are computed from the commits that land on `main`, so they
follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
fix(output): keep the token plaintext under --output table
feat: add the instances restart command
feat!: rename --tenant to --customer
```

`fix:` releases a patch; `feat:` a minor version; `!` after the type, or a
`BREAKING CHANGE:` footer, a major one (a minor one before 1.0.0). `docs:`, `chore:`,
`ci:`, `test:`, `refactor:` and `style:` release nothing on their own.

A squash merge uses the pull request title as the commit message, so the **PR title**
check holds the title to the same rules. A single-commit pull request's commit message
must match its title. README's Releasing section describes the rest of the release.

## Contribution rules

Please:

- keep pull requests focused;
- add or update tests when behavior changes;
- update documentation when relevant;
- do not include secrets, customer data, or confidential information;
- do not submit code or assets that you do not have the right to contribute;
- preserve required third-party license and attribution notices.

Accepted contributions are contributed under Apache-2.0.

## Security

Do not report unpatched vulnerabilities in public issues.

See [SECURITY.md](./SECURITY.md).
