package fastview

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type TestView struct{}

func (tv *TestView) WithView(vm_data <-chan string) *View {

}

func TestFastView(t *testing.T) {
	Convey("Happy path builder", t, func() {
		Convey("When builder succeeds", func() {

			input := make(chan int)
			vc, err := NewViewBuilder[int, string](input).
				WithModel(func(x int) string { return string(x) }, nil).
				WithView().
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
