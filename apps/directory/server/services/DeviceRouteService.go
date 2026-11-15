import "cmp"

// @Service
type DeviceRouteService struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	callers *CallerService
	// @Needs
	devices *DeviceService
}

func (service *DeviceRouteService) Add(authorization string, add DeviceAdd) (Device, error) {
	return service.addUnless(service.admission.Admit(authorization), add)
}

func (service *DeviceRouteService) List(authorization string, personIds string) ([]Device, error) {
	return service.listUnless(service.admission.Admit(authorization), personIds)
}

func (service *DeviceRouteService) Remove(authorization string, deviceId string) error {
	return service.removeUnless(service.admission.Admit(authorization), deviceId)
}

func (service *DeviceRouteService) Owners(authorization string, query DeviceOwnersQuery) (DeviceOwners, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.ownersUnless(cmp.Or(refused, service.callers.ServiceFor(caller, query.DomainId)), query)
}

func (service *DeviceRouteService) addUnless(admission Admission, add DeviceAdd) (Device, error) {
	if admission.Refusal != nil {
		return Device{}, admission.Refusal
	}
	return service.devices.Add(admission.Caller.MemberId, add)
}

func (service *DeviceRouteService) listUnless(admission Admission, personIds string) ([]Device, error) {
	if admission.Refusal != nil {
		return []Device{}, admission.Refusal
	}
	return service.devices.List(admission.Caller.DomainId, admission.Caller.MemberId, personIds)
}

func (service *DeviceRouteService) removeUnless(admission Admission, deviceId string) error {
	if admission.Refusal != nil {
		return admission.Refusal
	}
	return service.devices.Remove(admission.Caller.MemberId, deviceId)
}

func (service *DeviceRouteService) ownersUnless(refused error, query DeviceOwnersQuery) (DeviceOwners, error) {
	if refused != nil {
		return DeviceOwners{Owners: map[string]string{}}, refused
	}
	return service.devices.Owners(query.DomainId, query.DeviceIds)
}
