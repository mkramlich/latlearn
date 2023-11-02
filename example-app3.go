// latlearn/example-app3.go
//     project: https://github.com/mkramlich/LatLearn

package main

import (
    "log"
    "sync"

    "./latlearn"
)

// PURPOSE:
//
// Demonstrating to users that they *can* apply LatLearn instrumentation
// in a program with multiple (2+) goroutines, and thus under potentially *highly*
// concurrent conditions, *without* problems. Specifically, that it leads to *no*
// known corruption or deadlock vulnerabilities, by design.
//
// It is important to note that the amount of actual concurrency or physically-
// parallel execution possible at runtine depends on a *variety* of factors, most
// outside the control of LatLearn or this example app. Namely, both the host
// hardware's capabilities, the OS type, the OS version/build & its current
// configuration, and also how the Golang runtime has been configured (eg. via
// the so-called environment variables, or, calls to Golang runtime APIs.) On a
// typical modern machine there will be a *minimum* of 2 to 4 general-purpose CPU
// cores and execution threads avail. And generally Golang tries by default to
// make a number of goroutines avail which matches what the host appears to
// support, at process startup. If it ever matters, and for clarity, LatLearn
// *does* probe the relevant runtime capabilities observed on the host, and then
// includes a snapshot of these values in every report it generates.
//

func fn( id int, wg *sync.WaitGroup) {
    defer wg.Done()

    ll := latlearn.B( "example-app3/fn")
    //log.Printf(       "fn %d\n", id)
    ll.A()

    if ((id % 2) == 0) { latlearn.Report()} // ~50% of these fn calls will ALSO request report gen

    // Though it does not seem like it here -- studying only this code in isolation -- the above calls to latlearn.Report() will cause ALL their requested report gens to happen under a *singleton* latlearn-managed goroutine. And thus it WILL NOT race with (or otherwise clobber) any OTHER concurrent request to gen a report. In other words, no more than 1 LatLearn report (well, *per* underway process of a LatLearn-instrumented app) will ever be in the *middle* of being written, at any moment in time. If multiple LatLearn-instrumented processes *must* be able to run, without error, on a single host (more precisely: all mounted with a *single* shared file system) then it is the user's responsibility to ensure they are each configured to write to distinct report file paths.
}

func main() {
    pre :=      "example-app3/main"
    log.Printf( "%s\n", pre)

    latlearn.Init()

    latlearn.Report_fpath             = "latlearn-report-conc.txt"
    latlearn.Should_report_builtins   = true
    latlearn.Should_subtract_overhead = true

    latlearn.Latency_measure_self_sample(-1) // default attempts capture of 1M samples of OVERHEAD_SPAN

    ll := latlearn.B( "log-foo")
    log.Printf(       "%s: foo\n", pre)
    ll.A()

    n  := 1_000
    log.Printf( "%s: concurrent calls to fn: %d\n", pre, n)
    wg := &sync.WaitGroup{}
    wg.Add( n)

    for i := 0; i < n; i++ {
        go fn( i, wg)
    }

    wg.Wait()
    // by this point, all of the above per-fn goroutines have ended

    ok  := latlearn.Report()
    ok2 := latlearn.Report() // why 2nd call to Report here? just cuz
    ok3 := latlearn.Stop()

    log.Printf( "%s: finished. report ok %v, ok %v, stop ok %v\n", pre, ok, ok2, ok3)
}
