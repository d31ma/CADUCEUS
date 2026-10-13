// SettingsTestSetup gives the settings test units no adapters: they cover
// the pure utilities that the settings sections share.
// @TestSetup
type SettingsTestSetup struct{}

func (SettingsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
