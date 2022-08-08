# Templates
* Functions can be called as methods of the objects:
    ```
    {{ if .User.HasPermission }}
    <p>Welcome admin {{ .User.Name }}!</p>
    {{ else }}
    <p>Access denied</p>
    {{ end }}
    ```
Otherwise pass a FuncMap
* 