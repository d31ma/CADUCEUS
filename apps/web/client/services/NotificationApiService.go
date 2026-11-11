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
	page, failure := BehaviorsAnswer[NotificationPage](service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.notify.Notifications(authorization, cursor, service.pageSize())
	}))
	return page, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) MarkRead(notificationId string) string {
	_, failure := service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.notify.MarkRead(authorization, notificationId)
	})
	return service.problems.OfFailure(failure)
}

func (service *NotificationApiService) Settings() (NotificationSettings, string) {
	settings, failure := BehaviorsAnswer[NotificationSettings](service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.notify.Settings(authorization)
	}))
	return settings, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) SaveSettings(settings NotificationSettings) (NotificationSettings, string) {
	saved, failure := BehaviorsAnswer[NotificationSettings](service.access.Send(func(authorization string) (BehaviorValue, error) {
		return service.notify.SaveSettings(authorization, settings)
	}))
	return saved, service.problems.OfFailure(failure)
}

func (service *NotificationApiService) pageSize() int {
	return 50
}
