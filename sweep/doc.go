// Package sweep runs a service's reclaimers, the bounded passes that remove
// what its libraries leave behind, as one go-core reactor. A reclaimer is a
// library's own, such as blobfs's sweep or go-messaging's retention; this
// package imports none, and each satisfies [Reclaimer] structurally. A
// wiring defect panics, per the database package; a defect in the list of
// reclaimers is an error.
//
// # Construction
//
// [New] builds a [Sweeper] over the service's sqlate session from an
// explicit list of reclaimers, each a [Registration] with its name and its
// [Policy], the interval between its cycles and the budget of each pass.
// The list is the only way in: the package keeps no registry. New refuses
// an empty list, an empty or repeated name, a nil reclaimer, a
// non-positive interval or budget, and a non-positive shutdown timeout,
// before any I/O.
//
// An [Option] configures the rest:
//
//   - [Unlocked] runs without claiming the reclaimers' locks, on a dialect
//     without sqlate.TryLocker or with it. Without it, such a dialect is
//     [ErrNoTryLocker].
//   - [Gated] sets the [Gate] each pass holds shared, such as go-core's
//     reactor.Gate, so an operation that holds it exclusively, such as a
//     schema change, runs with the passes paused.
//   - [OnPass] sets the [Hook] each [Pass] is reported to. Without it, a
//     pass that returned an error is logged at warn on the sweeper's
//     logger, and no other pass is.
//
// # Cycles
//
// The Sweeper's reactor runs on a go-core reactor.Wake source ticking at
// the smallest policy interval. Each delivery runs, one at a time in the
// list's order, every reclaimer whose interval has passed since its last
// cycle and every reclaimer [Sweeper.Nudge] named since then; every
// reclaimer is due at start. A nudge of a name the list does not hold
// panics.
//
// A reclaimer's cycle first claims the advisory lock "sweep.<name>" on a
// pinned connection with sqlate.TryLocker. A lock another process holds
// runs no pass and reports nothing: that process is reclaiming. Once
// claimed, the cycle passes at the policy's budget while a pass reports
// more, reporting each pass. A pass's error is reported and ends neither
// the cycle nor the reactor: the cycle goes on while the pass reported
// more, and a pass that reported none ends it, error or not. Nothing else
// bounds a cycle but the drain, so a reclaimer that reports more on every
// pass, failing or not, holds the reactor, and every reclaimer after it in
// the list, until the drain. The claim is released when the cycle ends, on
// every path: unlocked, then the connection closed. A failure to claim or
// to release the lock is logged at warn, and a connection whose unlock
// failed is discarded rather than pooled, so the session's end releases
// the lock.
//
// # Drain
//
// The Sweeper is a lifecycle Starter, Stopper, ReadinessChecker, and
// Monitored through [Sweeper.Start], [Sweeper.Shutdown], [Sweeper.Ready],
// and [Sweeper.Err], which are its reactor's, so a composition root makes
// it a graph node's value and the coordinator starts, drains, and watches
// it. Once the drain begins, no further pass starts, and a wait on the gate
// ends without one; the pass in flight finishes. A pass still running when
// the reactor cancels its handlers (half the shutdown timeout New is given)
// ends the cycle unreported, and the cancellation is the reactor's error.
// The Sweeper keeps no backlog state: what a drain leaves, the next start
// finds.
package sweep
