# Contributing to Superphenix

Welcome! We're glad you want to contribute to Superphenix.

- [Ways to Contribute](#ways-to-contribute)
- [Find an Issue](#find-an-issue)
- [Ask for Help](#ask-for-help)
- [Pull Request Lifecycle](#pull-request-lifecycle)
- [Development Environment Setup](#development-environment-setup)
- [Sign Your Commits](#sign-your-commits)
- [Running Tests Locally](#running-tests-locally)
- [Pull Request Checklist](#pull-request-checklist)

As you get started, you are in the best position to give us feedback on areas that need help, including:

* Problems found during setting up a new developer environment
* Gaps in our Quickstart Guide or documentation
* Bugs in our automation scripts

If anything doesn't make sense or doesn't work when you run it, please open an issue and let us know.

## Ways to Contribute

We welcome many different types of contributions:

* New features
* Builds, CI/CD
* Bug fixes
* Documentation
* Issue triage
* Answering questions (e.g., in GitHub Discussions or issue comments)
* Release management

Not everything happens through a GitHub pull request. Feel free to open an issue or start a discussion to propose ideas or ask how you can help.

## Find an Issue

We use the [good first issue](https://github.com/super-phenix/superphenix/labels/good%20first%20issue) and [help wanted](https://github.com/super-phenix/superphenix/labels/help%20wanted) labels to highlight work suitable for new and existing contributors.

Once you see an issue you'd like to work on, please post a comment saying that you want to work on it (e.g., "I want to work on this") so we can avoid duplicate effort.

If you want to contribute but don't know where to start or can't find a suitable issue, open an issue with the tag `contribution-wanted` and describe your interests—we'll help find something.

## Ask for Help

The best ways to get help when contributing:

* **GitHub Discussions** – for questions, ideas, and general discussion
* **GitHub Issues** – for bugs and feature requests; you can also ask clarifying questions on the relevant issue
* **Pull request comments** – for feedback on your specific change

## Pull Request Lifecycle

1. **Fork** the repository and create a branch from `main`.
2. **Make your changes** and add tests/docs as needed.
3. **Sign off** your commits (see [Sign Your Commits](#sign-your-commits)).
4. **Open a pull request** against `main`. Fill in the PR template and link any related issues.
5. **Address review feedback.** Maintainers may request changes; update the PR accordingly.
6. Once approved and the required CI checks pass, a maintainer will merge your PR.

We use "[lazy consensus](https://community.apache.org/committers/lazyConsensus.html)": if there are no objections after a few days and the PR looks good, it can be merged.

## Development Environment Setup

(To be expanded as the codebase grows. For now, see the main [README](./README.md) for high-level setup and deployment.)

* Ensure you have a Kubernetes cluster (or kind/k3s for local development).
* Clone the repo: `git clone https://github.com/super-phenix/superphenix && cd superphenix`
* Follow any `make` or script targets documented in the README for building or deploying.

If you run into setup issues, please open an issue so we can improve the docs.

## Sign Your Commits

We use the [Developer Certificate of Origin (DCO)](https://probot.github.io/apps/dco/) to certify that you wrote or have the right to submit the code you are contributing.

You must sign off each commit by adding the following to your commit message (your sign-off must match the git user and email for the commit):

```
Signed-off-by: Your Name <your.email@example.com>
```

Using `git commit -s` will add this automatically:

```bash
git commit -s -m 'Your commit message'
```

If you forgot to sign off and haven't pushed yet:

```bash
git commit --amend -s
```

The `DCO` pull-request check verifies the sign-off on every commit, including commits added when updating a PR.

## Running Tests Locally

```bash
while IFS= read -r module_dir; do
  packages="$(cd "$module_dir" && go list ./... | awk '!/(^|\/)e2e(\/|$)/')"
  if [ -n "$packages" ]; then
    (cd "$module_dir" && go test -v -skip '^TestControllers$' $packages)
  fi
done <<< "$(go list -m -f '{{.Dir}}')"
```

## Pull Request Checklist

Before submitting your pull request, please ensure:

* [ ] Your commit messages are clear and signed off (DCO).
* [ ] You have added or updated tests (and docs) as appropriate for your change.
* [ ] Your code follows the project's style and conventions (e.g., formatting, lint).
* [ ] You have run any relevant linters or tests locally.
* [ ] You have updated the README or other docs if you changed behavior or added features.
* [ ] The PR description explains the change and links to any related issues.

Thank you for contributing to Superphenix.
