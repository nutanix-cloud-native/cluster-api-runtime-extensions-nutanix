+++
title = "Releasing"
icon = "fa-solid fa-gift"
+++

This project uses [release-please] to automate changelog updates per release. Due to security restrictions[^1] in the
`nutanix-cloud-native` GitHub organization, the release process is a little more complex than just using the
[release-please-action].

When a release has been cut, a new release PR can be created manually using the `release-please` CLI locally. This needs
to be run by someone with write permissions to the repository. Create the `release-please` branch and PR:

```shell
make release-please
```

This will create the branch and release PR. From this point on until a release is ready, the `release-please-action`
will keep the PR up to date (GHA workflows are only not allowed to create the original PR, they can keep the PR up to
date).

When a release is ready, the commits in the release PR will need to be signed (again, this is a security requirement).
To do this, check out the PR branch locally:

```shell
gh pr checkout <RELEASE_PR_NUMBER>
```

Sign the previous commit:

```shell
git commit --gpg-sign --amend
```

And force push:

```shell
git push --force-with-lease
```

The PR will then need the standard 2 reviewers and will then be auto-merged, triggering the release jobs to run and push
relevant artifacts and images.

[^1]: Specifically, GitHub Actions workflows are not allowed to create or approve PRs due to a potential security flaw.
    See [this blog post][cider-sec] for more details, as well as the [Security Hardening for GitHub Actions
    docs][gha-security-hardening].

[release-please]: https://github.com/googleapis/release-please/
[release-please-action]: https://github.com/google-github-actions/release-please-action
[cider-sec]: https://medium.com/cider-sec/bypassing-required-reviews-using-github-actions-6e1b29135cc7
[gha-security-hardening]: https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions
