# CloseCheck

`closecheck` is a static analysis tool for Go, which checks `Close` methods are called on types that require it.

This tool is incomplete, a failed experiment where too many edge cases exist to do this effectively.

# Epilogue

Initially the project's goal was to find variables that implemented the `io.Closer` interface, and check to ensure they are
closed. Eventually it was going to have a flag to check all closers or just a few select standard library types that
*really* need to be closed (such as `os.File`, `http.Response`, etc).

The main problem comes with tracking these variables across function boundaries. If the variable is used exclusively
within a function, it's trivial to use `go/types` to ensure it is closed. However, it's harder to track the usage of the
variable if it's returned or was a function argument.

There was an attempt to track these variables across functions, and using some `x/tools/go` packages, some tracking was
possible (see `69fd5ec96187029badecb3788c6d280ac19f9e6c`). However, this had some limitations and was later scrapped
during a rewrite for the sake of simplicity (but should be reviewed). Future versions may use the same method, or a
similar callgraph/DAG and be more precise.

But tracking arguments was the downfall, checking if a type was passed to a function is trivial, but if it's wrapped in
a composite literal, it's harder or any other countless ways (sent on a channel, wrapped in another function etc).

Even if these could be excluded correctly - you'd be creating a checker with so many limitations, it would rarely be
effective.

Finally, the current approach wasn't the most performant, so a refactor would be necessary to use less memory (it
tracked *all* function/return arguments - not just those being tracked). This could likely be fixed by looking for
functions, then walking each function checking for types to track and then exclude those that need to be. This wouldn't
be faster, but by walking each function, you could discard any results that are no longer necessary - decreasing memory,
but keeping processing time about the same.

It's still possible to achieve the project's goals, I just cannot spare more time to this (30+ hours so far).
