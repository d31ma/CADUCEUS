import "time"

// TeamService lists everyone who shares a domain with the signed-in user,
// with their presence from /presence/team, refreshed every 20 seconds while
// the Team page is open.
// @Service
type TeamService struct {
	// @Needs
	presenceClient            *PresenceClient
	applicationEventPublisher ApplicationEventPublisher
	Members                   []TeamMember
	Count                     int
	Loading                   bool
	token                     string
}

// @OnStart
func (team *TeamService) Start() {
	team.Members = []TeamMember{}
	team.Loading = true
}

// HearSession loads the roster for the signed-in user and keeps it fresh.
// @OnEvent({"replay":true})
func (team *TeamService) HearSession(changed SessionChanged) {
	team.token = changed.Token
	FlowUtils{}.When(changed.SignedIn, team.begin)()
}

// HearTimer refreshes the roster.
// @OnEvent
func (team *TeamService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(elapsed.Name == team.timer(), team.refresh)()
}

func (team *TeamService) begin() {
	team.refresh()
	team.applicationEventPublisher.PublishEvent(TimerRequested{Name: team.timer(), DelayMs: 20000, Repeat: true}, false)
}

// refresh pulls the latest roster and online state; a failure shows an empty
// roster, like the page did.
func (team *TeamService) refresh() {
	teammates, _ := team.presenceClient.Team("Bearer " + team.token)
	team.Members = TeamUtils{}.Members(teammates, time.Now())
	team.Count = len(team.Members)
	team.Loading = false
	team.applicationEventPublisher.PublishEvent(TeamRefreshed{Count: team.Count}, false)
}

func (TeamService) timer() string { return "team" }
