package fastview

import (
	"fmt"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type TestView struct {
	updates chan []EleUpdate
}

func NewTestView(input <-chan string) Viewer {
	updates := make(chan []EleUpdate)
	go func() {
		for datum := range input {
			updates <- []EleUpdate{
				{
					EleId: datum,
					Ops: []Op{
						{Key: "foo", Value: "bar"},
					},
				},
			}
		}
	}()

	return &TestView{
		updates: updates,
	}
}

func (tv *TestView) Write(writer io.Writer) (err error) {
	_, err = writer.Write([]byte("blah"))
	return
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
