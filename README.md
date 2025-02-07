# testctx

> `*testing.T` with `context.Context` and middleware

Package `testctx` aims to represent what `*testing.T` might have looked like if
`context.Context` existed at the time it was created.

The main benefit of joining these two packages is support for test middleware,
which can greatly cut down on repetition (`t.Parallel()`) and allow for
transparent test instrumentation.

## differences from Go 1.24 `t.Context()`

Go 1.24 adds a `t.Context()` accessor which provides a `context.Context` that
will be canceled when the test completes, just before `t.Cleanup`-registered
funcs are executed. However, it does _not_ provide any way to modify the
context.

By contrast, this package provides both a `t.Context()` and a
`t.WithContext(ctx)` for modifying the context that is passed to sub-tests, and
additionally passes the `t.Context()` value as a `ctx` argument to tests.
