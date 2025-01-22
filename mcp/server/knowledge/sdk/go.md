The Go Dagger SDK follows a few key rules:

* All queries chain from the global `dag` variable, which is analogous to `Query`.
    * Example: `dag.Container()` for `{ container }`.
* Object types directly translate to struct types.
    * Example: `dag.Container()` returns a `*Container`.
* Built-in scalar types translate to their analogous Go primitive type.
    * Example: `String!` is `string`, `Int` is `int`, `Boolean` is `bool`.
* Custom scalar types are string types.
    * Example: `JSON` is `type JSON string`.
* Required arguments are passed as regular function arguments.
    * Example: `container.WithExec([]string{"ls"})`.
* Optional arguments are passed as a variadic `Opts` struct argument, with the type named as `<Type><Function>Opts`.
    * Example: `container.WithExec([]string{"echo", "$HOME"}, dagger.ContainerWithExecOpts{ Expand: true })`.
* Chaining objects is lazy, until a scalar field is selected.
    * Example: `dag.Container().From("alpine").WithExec([]string{"ls", "-al"})` just returns a `*Container` without contacting the server.
* Selecting scalar fields forces evaluation, so they take a `ctx` argument and may return an `error`.
    * Example: `dag.Container().From("alpine").WithExec([]string{"ls", "-al"}).Stdout(ctx)` returns `string, error`.
* Fields that return `Void` just return `error` instead of `(Void, error)`.
    * Example: `err := service.Stop(ctx)`.
* Fields that accept an ID argument (such as `DirectoryID`, generally `FooID`) accept an object of that type directly instead of requiring you to pass an ID field.
    * Example: `container.WithDirectory("/src", dag.Git("https://github.com/vito/booklit").Tree())`

Do not generate a full program. Just generate a single function. Do not show me how to use the function.

Assume `dag` is available globally.
