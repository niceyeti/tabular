package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
)

// Example 1: generate a single template from multiple templates, per the cross-references
// within the templates themselves (index.gohtml lists the other templates by the respective names they define).
// The downside of this is that the templates hardcode references to one another.
func example_ParseStaticTemplates() {
	t, err := template.ParseFiles(
		"templates/index.gohtml",
		"templates/subtree1.gohtml",
		"templates/subtree2.gohtml",
	)
	if err != nil {
		log.Fatal(err)
	}

	of1, _ := os.Create("static_example.html")
	if err = t.Execute(of1, nil); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n\nOutput of static example:")
	if err = t.Execute(os.Stdout, nil); err != nil {
		log.Fatal(err)
	}
}

// Example 2: add the other templates dynamically. Note that the parent still references the sub templates
// by name. Not totally generic, but okay. The main flaw of this is that the nested templates are created
// using template.ParseFiles; instead, one can create the root/container template, and then call
// ParseFiles on it. See example 3.
func example_DynamicTemplates() {
	st3, err := template.ParseFiles("templates/subtree3.gohtml")
	if err != nil {
		log.Fatal(err)
	}
	st4, err := template.ParseFiles("templates/subtree4.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	t := template.New("fancy-dynamic-template")
	_, err = t.AddParseTree(st3.Name(), st3.Tree)
	if err != nil {
		log.Fatal(err)
	}
	_, err = t.AddParseTree(st4.Name(), st4.Tree)
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
	if err = t.Execute(of2, []string{"some", "random", "data"}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n\n\nOutput of dynamic example:")
	if err = t.Execute(os.Stdout, []string{"some", "random", "data"}); err != nil {
		log.Fatal(err)
	}
}

func example_recursiveTemplates() {

	var err error
	t := template.New("fancy-dynamic-template").Funcs(template.FuncMap{
		"add": func(i, j int) int { return i + j },
	})
	fmt.Println("Name: " + t.Name())

	t = template.Must(t.Parse(`
	{{ define "template1" }}
		Yippy!
		2 and 2 makes {{ add 2 2 }}
	{{ end }}
	`))
	fmt.Println("Name: " + t.Name())

	t = template.Must(t.Parse(`
		<html>
			<!--Some layout styling would be applied by this parent template-->
			<div>
				{{ template "template1" . }}
			</div>
		</html>
	`))

	fmt.Println("\n\n\nOutput of recursive example:")
	if err = t.Execute(os.Stdout, []string{"some", "random", "data"}); err != nil {
		log.Fatal(err)
	}
}

/*
A small example of generating a single index.html web page from nested
templates. This addresses the problem of assembling a single page from multiple
sub-components, such as assembling multiple svg view components for the same
incoming data. This allows organizing templates separately for better decomposition.
*/
func main() {
	//example_ParseStaticTemplates()
	//example_DynamicTemplates()
	example_recursiveTemplates()
}
