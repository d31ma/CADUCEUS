// @Setup
type CategoriesSetup struct{}

func (CategoriesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
