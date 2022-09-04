# Templates
* Receiver methods can be called:
    ```
    {{ if .User.HasPermission }}
    <p>Welcome admin {{ .User.Name }}!</p>
    {{ else }}
    <p>Access denied</p>
    {{ end }}
    ```
Otherwise pass a FuncMap

## Fmt verbs

| Verb  | Description |
|----|----|
| %v  | The value in a default format |
| %T  | A representation of the type of the value |
| %%  | A literal percent sign; consumes no value |
| %t  | Boolean: the word true or false |
| %b  | Integer: base 2 |
| %d  | Integer: base 10 |
| %f  | Floating point: decimal point but no exponent, e.g. 123.456 |
| %s  | String: the uninterpreted bytes of the string or slice |
| %q  | String: a double-quoted string (safely escaped with Go syntax) |

## Comparability

Sources:
* https://go.dev/blog/laws-of-reflection
* https://research.swtch.com/interfaces

Comparability is crucial for map keys, serialization, and anywhere object identity is implicitly
needed. Dynamic dispatch is used for things like interfaces, hence not all comparability errors
can be caught at compile time, and some may cause panics:
```
    type A []byte
    type I interface {
        m()
    }

    func main() {
        var a interface{} = A{}
        var b interface{} = A{}
        fmt.Println("%t", a == b)
    }
```

NOT comparable:
* maps
* slices
* functions (compile time error)

Comparable:
* Arrays: contents are compared as a single value
* Interfaces: interfaces are equal when their dynamic type and dynamic value are both equal, or nil.
* Channels: both are nil or created by the same `make` call
* Structs: both exported and non-exported fields are compared

Library approaches: comparison can also be performed through a number of std=library functions
which are worth knowing for convenience:
* bytes.EqualFold(): case-insensitive byte sequence comparison
* cmp package: maps, slices, etc
* crypto/subtle pkg: provides comparisons which prevent timing attacks
* reflect.DeepEquals(): See docs, a very good write up.
* reflect: also contains funcs for more esoteric functionality, like evaluating channel direction.

The Laws of Reflection are useful knowledge, even if not using the reflect package:
* Reflection goes from interface value to reflection object.
* Reflection goes from reflection object to interface value.
* To modify a reflection object, the value must be settable.

Pitfalls:
* Recall that some comparable native types can poses issues, such as float.NaN never
  being equal to itself, by definition.






