# Coding standards

Mechanical rules (formatting, `go vet`, 80-column limit, `errors.AsType`)
are enforced by `just check`. Reviewers skip what `just check` catches.

## All languages

- Keep every line of code at 80 columns or fewer, counting a tab as 4
  columns. Markdown is exempt.
  Wrap long calls, signatures and literals; split a long expression into
  named parts when that reads better than a wrap.

## Go

- Match errors by type with `errors.AsType[T](err)`, not
  `errors.As(err, &target)` or a type assertion on the error:

  ```go
  if exit, ok := errors.AsType[*exec.ExitError](err); ok {
  	// use exit
  }
  ```
