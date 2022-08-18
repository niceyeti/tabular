package fastview

import (
	"fmt"
	"html/template"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type TestView struct {
	updates chan []EleUpdate
}

func NewTestView(
	input <-chan string,
	done <-chan struct{},
) ViewComponent {
	updates := make(chan []EleUpdate)
	go func() {
		for datum := range input {
			updates <- []EleUpdate{
				{
					EleId: datum,
					Ops: []Op{
						{
							Key:   "foo",
							Value: "bar",
						},
					},
				},
			}
		}
	}()

	return &TestView{
		updates: updates,
	}
}

func (tv *TestView) Template(
	funcs template.FuncMap,
) (t *template.Template, err error) {
	return nil, nil
}

func (tv *TestView) Updates() <-chan []EleUpdate {
	return tv.updates
}

func TestFastView(t *testing.T) {
	Convey("Happy path builder", t, func() {
		Convey("When builder succeeds", func() {
			input := make(chan int)
			views, err := NewViewBuilder[int, string](input).
				WithModel(func(x int) string { return fmt.Sprintf("%d", x) }).
				WithView(NewTestView).
				Build()
			So(err, ShouldBeNil)
			So(len(views), ShouldEqual, 1)

			// Send a value and make sure it is sent across
			go func() {
				input <- 1337
			}()
			update := <-views[0].Updates()
			So(len(update), ShouldEqual, 1)
			So(update[0].EleId, ShouldEqual, "1337")
		})
	})
}
