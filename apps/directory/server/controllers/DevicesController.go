// @RequestRoute("/api/v1/directory/devices")
// @SignedIn
type DevicesController struct {
	// @Needs
	devices *DeviceRouteService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *DevicesController) PostDevice(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	add DeviceAdd,
) (Device, error) {
	return controller.devices.Add(authorization, add)
}

// @GetRoute
func (controller *DevicesController) GetDevices(
	// @FromHeader("authorization")
	authorization string,
	// @FromQuery("personIds")
	personIds string,
) ([]Device, error) {
	return controller.devices.List(authorization, personIds)
}

// @DeleteRoute("/{device_id}")
// @ResponseStatus(204)
func (controller *DevicesController) DeleteDevice(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("device_id")
	deviceId string,
) error {
	return controller.devices.Remove(authorization, deviceId)
}

// @PostRoute("/owners")
// @Allow(Service)
func (controller *DevicesController) PostOwners(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	query DeviceOwnersQuery,
) (DeviceOwners, error) {
	return controller.devices.Owners(authorization, query)
}
