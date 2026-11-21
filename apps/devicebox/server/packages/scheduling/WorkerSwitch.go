package scheduling

import "encoding/json"

type WorkerSwitch struct {
	ApplicationJson string
}

func (workerSwitch WorkerSwitch) Run(work func()) {
	workerSwitch.runWhenOn(workerSwitch.isOn(), work)
}

func (WorkerSwitch) runWhenOn(on bool, work func()) {
	if !on {
		return
	}
	work()
}

func (workerSwitch WorkerSwitch) isOn() bool {
	settings := workerSettings{}
	json.Unmarshal([]byte(workerSwitch.ApplicationJson), &settings)
	return settings.Yon.Workers.Enabled == nil || *settings.Yon.Workers.Enabled
}
