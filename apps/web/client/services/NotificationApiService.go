// @Service
type NotificationApiService struct {
	// @Needs
	notify *NotifyClient
	// @Needs
	access *NotificationTokenService
	// @Needs
	problems *NotificationProblemService
}

func (service *NotificationApiService) Page(cursor string) (NotificationPage, string) {
	authorization, failure := service.access.Authorization()
	page, failure := service.page(authorization, cursor, failure)
	return page, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) MarkRead(notificationId string) string {
	authorization, failure := service.access.Authorization()
	return service.problems.OfFailure(service.markRead(authorization, notificationId, failure))
}

func (service *NotificationApiService) Settings() (NotificationSettings, string) {
	authorization, failure := service.access.Authorization()
	settings, failure := service.settings(authorization, failure)
	return settings, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) SaveSettings(settings NotificationSettings) (NotificationSettings, string) {
	authorization, failure := service.access.Authorization()
	saved, failure := service.saveSettings(authorization, settings, failure)
	return saved, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) page(authorization string, cursor string, failure error) (NotificationPage, error) {
	if failure != nil {
		return NotificationPage{}, failure
	}
	return BehaviorsAnswer[NotificationPage](service.notify.Notifications(authorization, cursor, service.pageSize()))
}

func (service *NotificationApiService) markRead(authorization string, notificationId string, failure error) error {
	if failure != nil {
		return failure
	}
	_, failure = service.notify.MarkRead(authorization, notificationId)
	return failure
}

func (service *NotificationApiService) settings(authorization string, failure error) (NotificationSettings, error) {
	if failure != nil {
		return NotificationSettings{}, failure
	}
	return BehaviorsAnswer[NotificationSettings](service.notify.Settings(authorization))
}

func (service *NotificationApiService) saveSettings(authorization string, settings NotificationSettings, failure error) (NotificationSettings, error) {
	if failure != nil {
		return NotificationSettings{}, failure
	}
	return BehaviorsAnswer[NotificationSettings](service.notify.SaveSettings(authorization, settings))
}

func (service *NotificationApiService) pageSize() int {
	return 50
}
