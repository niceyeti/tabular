package main

import (
	"html/template"
	"log"
	"os"
)

/*
	A small example of generating a single index.html web page from nested
	templates. This addresses the problem of assembling a single page from multiple
	sub-components, such as assembling multiple svg view components for the same
	incoming data. This allows organizing templates separately for better decomposition.
*/
func main() {
	// Example 1: generate a single template from multiple templates, per the cross-references
	// within the templates themselves (index.gohtml lists the other templates by the respective names they define).
	// The downside of this is that the templates hardcode references to one another.
	t, err := template.ParseFiles("templates/index.gohtml", "templates/subtree1.gohtml", "templates/subtree2.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	of1, _ := os.Create("static_example.html")
	if err = t.Execute(of1, nil); err != nil {
		log.Fatal(err)
	}

	// Example 2: add the other templates dynamically. Note that the parent still references the sub templates
	// by name. Not totally generic, but okay.
	st3, err := template.ParseFiles("templates/subtree3.gohtml")
	if err != nil {
		log.Fatal(err)
	}
	st4, err := template.ParseFiles("templates/subtree4.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	t = template.New("fancy-dynamic-template")
	_, err = t.AddParseTree(st3.Name(), st3.Tree)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = t.AddParseTree(st4.Name(), st4.Tree)
	if err != nil {
		log.Fatal(err)
	}

	t, err = t.Parse(`
		<html>
			<!--Some layout styling would be applied by this parent template-->
			<div>
				{{ template "` + st3.Name() + `" . }}
			</div>
			<div>
				{{ template "` + st4.Name() + `" . }}
			</div>
		</html>
	`)
	if err != nil {
		log.Fatal(err)
	}

	of2, _ := os.Create("dynamic_example.html")
	if err = t.Execute(of2, nil); err != nil {
		log.Fatal(err)
	}
}
