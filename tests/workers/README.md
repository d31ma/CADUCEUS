# tests/workers

Checks that Mesail's `@Schedule` workers stay off in the Lambda profile
(slice K27).

- `deploy/lambda/write-profile.sh` writes `"workers": {"enabled": false}`
  into `server/application-lambda.json`.
- Every worker of a Lambda app runs its work through
  `scheduling.WorkerSwitch` (kit), which reads that setting from
  `YON_APPLICATION_JSON` and then does nothing.
- realtime runs as a container (`YON_PROFILE=container`), so its sweep keeps
  running and is not checked here.
- A new worker file fails `TestEveryWorkerIsListed` until it gets its own
  case here.

Run: `cd tests/workers && go test ./...`. The tests read every file they
check with `os.ReadFile` (the profile script too, passed to `sh` on stdin),
so Go's test cache sees a change to any of them.
