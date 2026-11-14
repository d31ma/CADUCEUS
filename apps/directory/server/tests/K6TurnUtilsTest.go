import (
	"errors"
	"fmt"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type K6TurnUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *K6TurnUtilsTest) TestVisitStartsAtTheOffsetInIdOrder() {
	visited := []int{}
	err := TurnUtils{}.Visit(test.ids(), nil, 1, func(index int) error {
		visited = append(visited, index)
		return nil
	})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("[2 0 1]", fmt.Sprint(visited))
}

func (test *K6TurnUtilsTest) TestVisitKeepsEveryFailure() {
	err := TurnUtils{}.Visit(test.ids(), nil, 0, func(index int) error { return errors.New(test.ids()[index]) })
	Assert.Equals("a\nb\nc", err.Error())
}

func (test *K6TurnUtilsTest) TestVisitReportsAFailedListFirst() {
	err := TurnUtils{}.Visit([]string{}, errors.New("list failed"), 5, func(index int) error { return nil })
	Assert.Equals("list failed", err.Error())
}

func (test *K6TurnUtilsTest) TestTheOffsetWrapsAroundTheIds() {
	Assert.Equals("[0 1 2]", fmt.Sprint(TurnUtils{}.from(test.ids(), 5)))
}

func (test *K6TurnUtilsTest) TestNoIdsGiveNoTurns() {
	Assert.Equals("[]", fmt.Sprint(TurnUtils{}.from([]string{}, 7)))
}

func (test *K6TurnUtilsTest) ids() []string {
	return []string{"c", "a", "b"}
}
