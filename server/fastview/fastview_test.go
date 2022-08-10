package fastview

import (
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
		for range input {
			updates <- []EleUpdate{
				{
					EleId: "123",
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
			_, err := NewViewBuilder[int, string](input).
				WithModel(func(x int) string { return string(x) }).
				WithView(NewTestView).
				Build()
			So(err, ShouldBeNil)

			//     NewViewBuilder[DataModel, ViewModel](source chan DataModel) *ViewBuilder
			//     vb.WithModel(func([]DataModel) []ViewModel)
			//     vb.WithView(chan []ViewModel -> NewValuesGridView(t2_chan))
			//     vb.Build()  <- execute the builder to get views and ele-update chan; delaying execution of stored funcs allows setting up multiplexing
			//						of the @target channel to potentially several view listeners

		})
	})
}
