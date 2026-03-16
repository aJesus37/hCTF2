# Contributing to hCTF

Thank you for your interest in contributing to hCTF! This document provides guidelines for contributing to the project.

## Table of Contents

- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Commit Messages](#commit-messages)

## Development Setup

### Prerequisites

- Go 1.24 or later
- Task (task runner) - [installation guide](https://taskfile.dev/installation/)

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/ajesus37/hCTF.git
cd hctf

# Install dependencies
task deps

# Start development server (migrations run automatically on startup)
task run
```

The server will be available at http://localhost:8090

Pass `--admin-email` and `--admin-password` flags (or use `task run` which sets defaults).

## Making Changes

1. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```

2. **Make your changes** following the code style guidelines below.

3. **Test your changes** locally (see Testing section).

4. **Commit** with a descriptive message following conventional commits format.

## Testing

### Running Tests

```bash
# Run all tests
task test

# Run with coverage
task test-coverage

# Run smoke tests (requires running server)
task smoke-test

# Run E2E tests (requires running server and Playwright)
task e2e-test
```

### Manual Testing Checklist

Before submitting a PR, please verify:

- [ ] `task build` completes successfully
- [ ] `task test` passes all tests
- [ ] Application starts with `task run`
- [ ] Core functionality works (login, challenges, scoreboard)
- [ ] Admin dashboard loads and functions correctly
- [ ] No new errors in server logs

### Adding Tests

For new features, please add tests:

- **Handler tests**: Add to `handlers_test.go` for HTTP endpoint testing
- **Unit tests**: Create `*_test.go` files in the relevant package
- **E2E tests**: Add to `scripts/e2e-test.sh` or `scripts/browser-automation-tests.sh`

## Pull Request Process

1. **Update documentation** if your changes affect usage or configuration.

2. **Update CHANGELOG.md** under the `[Unreleased]` section following Keep a Changelog format.

3. **Ensure all tests pass** before submitting.

4. **Fill out the PR template** completely. Link any related issues.

5. **Request review** from maintainers.

6. **Address feedback** promptly.

## Code Style

### Go Code

We follow standard Go conventions:

- Use `go fmt` to format code
- Use `go vet` to catch common issues
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Run `golangci-lint` before submitting:
  ```bash
  golangci-lint run ./...
  ```

### Key Conventions

- **Error handling**: Always check errors, use meaningful error messages
- **Naming**: Use camelCase for unexported, PascalCase for exported
- **Comments**: Document all exported functions and types
- **SQL**: Always use parameterized queries (never string concatenation)
- **HTTP handlers**: Follow existing pattern with proper error responses

### Template/HTML

- Use semantic HTML5 elements
- Include proper ARIA labels for accessibility
- Maintain dark/light theme compatibility
- Use Tailwind CSS utility classes consistently

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Code style changes (formatting, semicolons, etc.)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or correcting tests
- `chore`: Changes to build process or auxiliary tools

### Examples

```
feat(auth): add OAuth2 GitHub login support

fix(scoreboard): correct rank calculation for tied scores

docs(api): update OpenAPI spec for team endpoints

refactor(db): optimize scoreboard query with CTE
```

### Scope

Common scopes: `auth`, `api`, `db`, `ui`, `admin`, `scoreboard`, `challenges`, `teams`, `deps`

## Getting Help

- **Bug reports**: [Open an issue](../../issues/new?template=bug_report.md)
- **Feature requests**: [Open an issue](../../issues/new?template=feature_request.md)
- **Security issues**: See [SECURITY.md](SECURITY.md)
- **General questions**: [Start a discussion](../../discussions)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
