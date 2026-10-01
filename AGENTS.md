# Code Rules

These rules are mandatory for every change, file, package and module in Go code.

Follow the existing repository patterns exactly.
Do not refactor, redesign, optimize, rename, or "improve" existing code unless explicitly required by the task.

## 1. Naming

* Use short, clear, direct names.
* Use lowercase names for unexported identifiers.
* Use PascalCase for exported types, functions, methods, and identifiers.
* Use camelCase for multi-word unexported identifiers.
* Follow Go's standard initialism conventions, such as `ID`, `URL`, `HTTP`, and `API`.
* Avoid unnecessarily long or descriptive names when a simpler name is clear.

Examples:

```go
users := []user{}
names := []string{}
urls := []string{}
```

Avoid unnecessarily long or descriptive names when a simpler name is clear.

## 2. Variables

* Always use `:=`.
* Never use `var`.
* Use `const` for fixed values.
* Constants must use `UPPERCASE_SNAKE_CASE`.

Use:

```go
users := []user{}
```

Do not use:

```go
var users []user
```

## 3. Conditions

* Always use `{}` for `if` blocks, even for a single statement.
* Never use `else`.
* Never use `else if`.
* Never use `&&` in a condition. Split into nested `if` blocks.
* Never compare a boolean against `false`; use `!` instead.
* Prefer guard clauses and early returns.

Use:

```go
if err != nil {
  return err
}

return data
```

Do not use:

```go
if err != nil {
  return err
} else {
  return data
}
```

Do not use:

```go
if valid(x) == false {
  return err
}
```

Use nested checks instead:

```go
if invalid(x) {
  return err
}
```

Do not use:

```go
if a && b {
  return err
}
```

Use nested checks instead:

```go
if a {
  if b {
    return err
  }
}
```

## 4. Null, Undefined, and Empty Values

Use `x == nil` when checking whether a pointer, interface, map, slice or channel is missing.

Use:

```go
if req == nil {
  return errors.New("requisição não encontrada")
}
```

Use `x == ""` for strings and `len(xs) == 0` for slices and maps.

Use:

```go
if id == "" {
  return errors.New("identificador não encontrado")
}
```

And:

```go
if user == nil {
  return errors.New("usuário não encontrado")
}
```

Use `x != nil` when checking whether a pointer, interface, map or slice is present.

Use:

```go
if term != "" {
  query = query.where("name ILIKE $1", term)
}
```

And:

```go
if len(ids) != 0 {
  query = query.where("id = ANY($1)", ids)
}
```

Use `cmp.Equal(x, ...)` when comparing values, and `slices.Equal(xs, ys)` when comparing slices.

Use:

```go
if cmp.Equal(order.status, "cancelled") {
  return errors.New("pedido cancelado não pode ser alterado")
}
```

And:

```go
if slices.Equal(a.item_ids, b.item_ids) {
  return errors.New("item já vinculado")
}
```

Do not compare a value against `nil` when the zero value is the meaning of absence.

Do not use:

```go
if name == nil {
  return errors.New("nome não encontrado")
}
```

Use the zero value check instead:

```go
if name == "" {
  return errors.New("nome não encontrado")
}
```

## 5. Formatting

* Never concatenate strings with `+`.
* Use `fmt.Sprintf` when formatting.

Use:

```go
msg := fmt.Sprintf("limite de %d bytes", max)
```

Do not use:

```go
msg := "limite de " + strconv.Itoa(max) + " bytes"
```

## 6. Default and Fallback Values

* Never use `||`.
* Use `cmp.Or(a, b, ...)` when a fallback is needed.
* Use `const` when the default value is fixed and known.

Use:

```go
bucket := cmp.Or(req.bucket, s.bucket)
```

For fixed defaults:

```go
const MINIMAL = 30

size := cmp.Or(req.size, MINIMAL)
```

Do not use:

```go
if req.Bucket == "" {
  req.Bucket = s.Bucket
}
```

## 7. Functions

* Use `func` declarations for global and standalone functions.
* Never define a global or standalone function as a function literal.
* Use function literals only for callbacks and inline local helpers.
* Keep a single-purpose body: one statement when the literal is a single expression.
* Never declare a function literal and call it immediately.

Use:

```go
func users() []user {
  return users
}
```

And:

```go
slices.SortFunc(users, func(a, b user) int { return cmp.Compare(a.name, b.name) })
```

And:

```go
slices.IndexFunc(users, func(u user) bool { return cmp.Equal(u.id, id) })
```

Do not use:

```go
var users = func() []user {
  return users
}
```

Do not use:

```go
slices.SortFunc(users, func(a, b user) int {
  return cmp.Compare(a.name, b.name)
})
```

## 8. Loops

* Never use `for {}`.
* Never use incomplete or malformed `for` clauses.
* Prefer `for _, x := range xs` for iteration.
* Always use a complete and explicit loop clause.

Use:

```go
for _, x := range xs {
  process(x)
}
```

And:

```go
for i := 0; i < n; i++ {
  process(xs[i])
}
```

Do not use:

```go
for {
  process(x)
}
```

## 9. Error Handling

* Never ignore errors, rejected operations, return values, or exceptions.
* Use the repository's established error-handling pattern.
* Wrap errors with `fmt.Errorf("...: %w", err)`.
* Compare wrapped errors with `errors.Is` and `errors.As`.
* Never use `_ = err`, blank returns, or `panic` in library code.

Use:

```go
value, err := operation()
if err != nil {
  return fmt.Errorf("operation: %w", err)
}
```

And:

```go
if errors.Is(err, err_not_found) {
  return nil
}
```

Or:

```go
value, _ := operation()
```

## 10. Comments

Never write comments.

This includes:

* Inline comments
* Block comments
* Explanatory comments
* TODO comments
* Documentation comments

Do not write:

```go
// Get the user
user, err := Get()
```

The code itself must be clear enough to follow the repository's conventions.

## 11. SQL Formatting

SQL inside raw string literals passed to functions such as `raw(...)` or `select_raw(...)` must follow this format:

* The opening backtick must be alone on its line.
* SQL must be indented inside the raw string literal.
* The closing backtick must be alone on its line.

Use:

```go
q := `
  SELECT
    id,
    device,
    created_at
  FROM sessions
  WHERE revoked_at IS NULL
    AND user_id = $1
`
```

Or:

```go
q := `SELECT id FROM sessions`
```

## 12. SQL Readability

Break long SQL statements into multiple lines.

* Put one column or field per line.
* Break long `WHERE` clauses into separate lines.
* Keep large `SELECT` statements readable.
* Do not put a large list of columns on one line.

Use:

```go
q := `
  SELECT
    id,
    external_id,
    name,
    status,
    settings,
    active,
    created_at,
    updated_at
  FROM orders
  WHERE id = $1
    AND account_id = $2
`
```

## 13. Quick Reference

Always:

* Use existing repository patterns.
* Use `:=`.
* Use lowercase names for unexported identifiers.
* Use `PascalCase` for exported types, functions, methods, and identifiers.
* Use `camelCase` for multi-word unexported identifiers.
* Follow Go's standard initialism conventions, such as `ID`, `URL`, `HTTP`, and `API`.
* Use `UPPERCASE_SNAKE_CASE` for constants.
* Use `{}` with every `if`.
* Use guard clauses and early returns.
* Use `x == nil`, `x == ""` and `len(xs) == 0` for absence.
* Use `x != nil` and `len(xs) != 0` for presence.
* Use `cmp.Equal(...)` and `slices.Equal(...)`.
* Use `cmp.Or(...)` for fallbacks.
* Use `fmt.Sprintf(...)` for formatting.
* Use `switch` when it fits.
* Use `for _, x := range xs`.
* Use `func` declarations for global and standalone functions.
* Use function literals only for callbacks.
* Use `fmt.Errorf("...: %w", err)`.
* Use `errors.Is(...)` and `errors.As(...)`.
* Keep SQL formatted and readable.

Never:

* Use `var`.
* Use `for {}`.
* Use `else`.
* Use `else if`.
* Use `&&` in a condition.
* Compare a boolean against `false`.
* Use `||`.
* Use string concatenation with `+`.
* Use `? :` or any immediately-invoked function literal as a ternary.
* Use incomplete `for` clauses.
* Use comments.
* Leave errors or rejected operations unhandled.
* Refactor or "improve" unrelated code.
