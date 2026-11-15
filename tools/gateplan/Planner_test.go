package main

import (
	"errors"
	"testing"
)

func testFolders() AppFolders {
	return AppFolders{
		{Name: "channels", KitPackages: []string{"transport", "events", "outbox"}},
		{Name: "identity", KitPackages: []string{"transport", "addresses"}},
		{Name: "mail-out", KitPackages: []string{"events", "smtpclient", "smtpclient/sink"}},
		{Name: "web"},
	}
}

func planFor(paths ...string) string {
	return Planner{Folders: testFolders()}.Plan(ChangeList{Paths: paths, Base: "origin/develop"}).Text()
}

func expectPlan(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPlanChoosesTheAppWhoseOwnFilesChanged(t *testing.T) {
	expectPlan(t, planFor("apps/identity/server/controllers/LoginController.go"), "identity\town files: apps/identity/server/controllers/LoginController.go\n")
}

func TestPlanChoosesAnAppThroughItsKitCopy(t *testing.T) {
	expectPlan(t, planFor("apps/channels/server/packages/events/Catalog.go"), "channels\town files: apps/channels/server/packages/events/Catalog.go\n")
}

func TestPlanChoosesEveryAppWhoseKitListNamesTheChangedPackage(t *testing.T) {
	expectPlan(t, planFor("kit/addresses/Address.go"), "identity\tuses kit/addresses: kit/addresses/Address.go\n")
}

func TestPlanChoosesEveryAppThatListsEventsWhenTheCatalogChanges(t *testing.T) {
	expectPlan(t, planFor("kit/events/Catalog.go"), "channels\tuses kit/events: kit/events/Catalog.go\nmail-out\tuses kit/events: kit/events/Catalog.go\n")
}

func TestPlanMatchesANestedKitPackageExactly(t *testing.T) {
	expectPlan(t, planFor("kit/smtpclient/sink/Sink.go"), "mail-out\tuses kit/smtpclient/sink: kit/smtpclient/sink/Sink.go\n")
}

func TestPlanChoosesNoAppForAKitPackageNoAppCopies(t *testing.T) {
	expectPlan(t, planFor("kit/mime/testdata/plain.eml"), "-\tno app uses the 1 changed files: docs and shared code only\n")
}

func TestPlanLeavesAVendoredModuleToTheSharedSteps(t *testing.T) {
	expectPlan(t, planFor("edge/calls/vendor/mesail.dev/packages/transport/Client.go"), "-\tno app uses the 1 changed files: docs and shared code only\n")
}

func TestPlanChoosesWebWhenTheResponsiveTestChanges(t *testing.T) {
	expectPlan(t, planFor("tests/responsive/Checks.go"), "web\ttests/responsive checks it: tests/responsive/Checks.go\n")
}

func TestPlanChoosesNoAppForDocsOnly(t *testing.T) {
	expectPlan(t, planFor("docs/mesail/SLICES.md", "CHANGELOG.md"), "-\tno app uses the 2 changed files: docs and shared code only\n")
}

func TestPlanJoinsAppsAndKeepsTheFirstReason(t *testing.T) {
	expectPlan(t, planFor("apps/channels/client/app.ts", "apps/web/client/index.html", "kit/transport/Client.go"), "channels\town files: apps/channels/client/app.ts\nidentity\tuses kit/transport: kit/transport/Client.go\nweb\town files: apps/web/client/index.html\n")
}

func TestPlanIgnoresAnAppThatNoLongerExists(t *testing.T) {
	expectPlan(t, planFor("apps/removed/server/Old.go"), "-\tno app uses the 1 changed files: docs and shared code only\n")
}

func TestPlanRunsEverythingWhenTheGateScriptChanges(t *testing.T) {
	expectPlan(t, planFor("apps/web/client/index.html", "scripts/gates.sh"), "*\tscripts/gates.sh changed\n")
}

func TestPlanRunsEverythingWhenTheGoCacheScriptChanges(t *testing.T) {
	expectPlan(t, planFor("scripts/go-cache.sh"), "*\tscripts/go-cache.sh changed\n")
}

func TestPlanRunsEverythingWhenTheKitSyncScriptChanges(t *testing.T) {
	expectPlan(t, planFor("scripts/kit-sync.sh"), "*\tscripts/kit-sync.sh changed\n")
}

func TestPlanRunsEverythingWhenACiScriptChanges(t *testing.T) {
	expectPlan(t, planFor("scripts/ci/tools.lock"), "*\tscripts/ci/tools.lock changed\n")
}

func TestPlanRunsEverythingWhenTheVendoredDuvayChanges(t *testing.T) {
	expectPlan(t, planFor("scripts/vendor/duvay.sh"), "*\tscripts/vendor/duvay.sh changed\n")
}

func TestPlanRunsEverythingWhenAToolChanges(t *testing.T) {
	expectPlan(t, planFor("tools/controllers/Check.go"), "*\ttools/controllers/Check.go changed\n")
}

func TestPlanRunsEverythingWhenAWorkflowChanges(t *testing.T) {
	expectPlan(t, planFor(".github/workflows/ci.yml"), "*\t.github/workflows/ci.yml changed\n")
}

func TestPlanRunsEverythingWhenAGoModChanges(t *testing.T) {
	expectPlan(t, planFor("kit/go.mod"), "*\tkit/go.mod changed\n")
}

func TestPlanRunsEverythingWhenAnAppGoSumChanges(t *testing.T) {
	expectPlan(t, planFor("apps/channels/server/packages/go.sum"), "*\tapps/channels/server/packages/go.sum changed\n")
}

func TestPlanRunsEverythingForAnUnknownPath(t *testing.T) {
	expectPlan(t, planFor("scripts/onprem/egress-check.sh"), "*\tscripts/onprem/egress-check.sh is outside every path the gate plan knows\n")
}

func TestPlanRunsEverythingWhenNothingChanged(t *testing.T) {
	expectPlan(t, planFor(), "*\tno file differs from origin/develop\n")
}

func TestPlanRunsEverythingWhenTheChangesCannotBeListed(t *testing.T) {
	plan := Planner{Folders: testFolders()}.Plan(ChangeList{Paths: []string{"docs/a.md"}, Failure: errors.New("git diff: exit status 128\nfatal"), Base: "origin/develop"})
	expectPlan(t, plan.Text(), "*\tthe changed files against origin/develop could not be listed: git diff: exit status 128 fatal\n")
}
