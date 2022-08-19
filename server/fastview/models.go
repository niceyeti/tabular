// fastview implements a builder pattern to implement simple views:
// given an input data format, apply a transformation to a view-model,
// and then  multiplex that data to one or more views.
package fastview

import (
	"html/template"
)

// TODO: move models around, reorg

// EleUpdate is an element identifier and a set of operations to apply to its attributes/content.
type EleUpdate struct {
	// The id by which to find the element
	EleId string
	// Op keys are attrib keys or 'textContent', values are the strings to which these are set.
	// Example: ('x','123') means 'set attribute 'x' to 123. 'textContent' is a reserved key:
	// ('textContent','abc') means 'set ele.textContent to abc'.
	Ops []Op
}

// Op is a key and value. For example an html attribute and its new value.
type Op struct {
	Key   string
	Value string
}

// ViewComponent implements server side views: Write to allow writing their initial form
// to an output stream and Updates to obtain the chan by which ele-updates are notified.
type ViewComponent interface {
	Updates() <-chan []EleUpdate
	// Parse parses the view-component and adds it to the passed parent template, thus inheriting
	// or possibly extending its definition (func-map, etc). This allows recursively definition
	// view-components. Not sure this is the best design, but 'works' a posteriori.
	Parse(*template.Template) (string, error)
}
