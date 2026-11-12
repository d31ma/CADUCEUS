// @TestSetup
type HealthTestSetup struct{}

func (HealthTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (HealthTestSetup) Health() *HealthController {
	return &HealthController{health: &HealthService{}}
}
