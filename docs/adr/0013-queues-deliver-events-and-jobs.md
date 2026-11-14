# ADR 0013: Queues deliver events and jobs (SQS, SNS, EventBridge Scheduler)

- Status: **Rejected** (2026-11-14, the owner). Kept as a record; nothing
  in it is built. Slice J0 never started.
- Deciders: the owner ("use Floci's SQS, SNS, ECS to facilitate a queuing
  system for jobs"), architecture (the design)
- Related: ADR 0004 (Lambda in Floci, one writer per FYLO root), ADR 0007
  (on-premise), EVENTS.md sections 2 and 4 (the outbox, drain on write,
  scheduled work), SLICES K6, K7, K8, K10, K12

## Rejection (2026-11-14)

The owner's answers, in their words:
- "Should Mesail's own app jobs also move to Floci SQS/SNS queues?":
  "No, build jobs only". The Floci queue the owner wants is for build,
  test and CI jobs on the Mac; TACHYON's planning session owns that, not
  Mesail.
- Q14: "app jobs are not moving to queues, for this project we have to
  assume the app will run on-premise".
- Q15: "again we have to assume this app will run on-premise".
- Q16: "again on-premise".

So:
- Events keep today's design: the outbox in the producer's own document,
  drain on write over HTTP, the relay route for what is left, and the
  requeue route for dead letters (EVENTS.md sections 2 and 4).
- Jobs keep the local scheduler and the call-again loop (`more: true`),
  with K6's bounded steps and K7's fix for slow subscribers.
- Mesail uses no SQS, SNS, EventBridge Scheduler, Pipes or ECS, in Floci
  or elsewhere. `kit/queue` is not built. Wave 10 (J0 to J5) is removed
  from SLICES.md.
- The owner's statement that Mesail must be assumed to run on-premise is
  recorded in [ADR 0007](0007-on-premise-first.md) ("Affirmed
  2026-11-14").

The rest of this ADR is the proposal as written, for the record.

## Context

Today an event is staged in the producer's own FYLO document (the
outbox, EVENTS section 2) in the same write as the data. The producer
pushes it over HTTP to each subscriber at once (drain on write) and a
scheduled relay route pushes what is left. Long jobs are routes that a
small scheduler script calls every minute, again while they answer
`more: true` (K6). Much recent work fights this push model: a slow
subscriber slows the user's request (K7), a busy writer answers 429 and
the event waits for the next sweep, and the scheduler, the router's queue
and the per-call budgets (K3, K6) exist to share one writer fairly.

Checked on the dev host on 2026-11-14 (read-only API calls to
`floci/floci:2.4.0`): SQS, SNS, ECS, EventBridge (rules and Scheduler),
Pipes and Lambda event source mappings all answer. Earlier research
(`realtime-serverless.md`) found SQS FIFO queues, SNS and SQS triggers
for Lambda emulated. Not yet checked: whether Floci keeps queue contents
across its own restart, how an SQS trigger behaves on a function with
reserved concurrency 1, FIFO ordering, redrive to a dead-letter queue,
and whether a container-image Lambda behind the Lambda Web Adapter
receives queue batches. TACHYON has no queue client and no queue marker;
the Lambda Web Adapter passes non-HTTP events (an SQS batch) to an HTTP
route of the app (`/events`), so receiving may need no TACHYON change.

## Proposed decision (rejected)

1. **The outbox stays the record; queues deliver.** A producer still
   stages each event in its own document, in the same write (FYLO has no
   transaction with SQS, so the outbox is what keeps an event from being
   lost or sent without its data). Instead of pushing to subscribers over
   HTTP, the producer publishes each staged event to an SNS topic per
   event type ("publish on write", milliseconds, no subscriber in the
   user's request) and marks it published; the relay route publishes only
   what a crash left unpublished.
2. **One queue per subscriber, per domain.** Each subscribing service has
   an SQS FIFO queue (per domain function, ADR 0004), subscribed to the
   topics it needs; the message group is the document (or conversation),
   so events of one document arrive in order. A dead-letter queue after N
   tries replaces the outbox's dead letters; the existing requeue route
   becomes a redrive.
3. **Consumers stay single writers.** The queue triggers the subscriber's
   own function (the one writer of its root): the Lambda Web Adapter hands
   the batch to the service's `events` route, which answers which messages
   failed (partial batch failure). Each batch stays inside SLICES rule 9's
   budget (3 writes and 1 second on the sign-in path), so a user request
   waits for at most one batch, as with K6.
4. **Jobs are queue messages.** EventBridge Scheduler puts a "tick" message
   on a job queue instead of the scheduler script calling routes; a long
   job (a domain removal, devicebox expiry, mail-out retries) does one
   bounded step per message and sends its next step as a new message (a
   delay where it waits). This replaces K6's call-again loop in Lambda
   form; native `ty start` keeps today's workers.
5. **ECS only for long work that touches no FYLO root.** An ECS task cannot
   write a FYLO root that a Lambda owns (one live process per root, ADR
   0004). So ECS is for future long jobs on files or the network alone
   (for example attachment scanning), not now (section 6, Q15).
6. **Native development keeps HTTP push.** `kit/outbox` gets a second
   pusher; Lambda form (Floci, AWS later) publishes to SNS, native form
   keeps today's HTTP push. Both run the same consumer routes.
7. **On-premise:** Floci's SQS, SNS and Scheduler run inside the install,
   with Floci's state on its data volume; no port is published (ADR 0007).

## What changes

- EVENTS.md section 2 (delivery: publish on write; dead letters in SQS)
  and section 4 (jobs as queue messages); CONTRACTS: each service's
  `events` route also accepts an SQS batch and answers partial failures.
- SLICES: a new Wave 10 (J0 to J5). K6 and K7 finish as planned: their
  limits still hold for native runs and for each consumer batch, and K7
  fixes today's slow requests before queues exist.
- TACHYON: no change needed to receive (the adapter's `/events` route).
  For sending, Mesail signs SQS and SNS calls itself (`kit/queue`, AWS
  SigV4) through a TACHYON HTTP remote to Floci's declared host; a TACHYON
  request for a queue client is optional.
- FYLO: nothing.

## 6. Questions for the owner

- Q14: keep the outbox as the record with queues delivering
  (recommended), replace the outbox with direct queue sends, or no change.
- Q15: ECS now, or only later for long work that touches no database
  (recommended).
- Q16: before or after the first release.

## Risks

- Floci's queue durability across its restart (J0 must prove it, or the
  design keeps HTTP push for events).
- A queue-triggered call and a user request share one writer: the router
  can no longer put the user first for those calls; the per-batch budget
  bounds the wait instead.
- One more moving part per domain (topics, queues, dead-letter queues,
  triggers) in the deploy files.
