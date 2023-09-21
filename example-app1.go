// latlearn/example-app1.go
//     project: https://github.com/mkramlich/latlearn

package main

import (
    "fmt"
)

func fn1() {
    span :=      "fn1"
    ll   := llB( span)
    fmt.Printf(  "%s\n", span)
    ll.A()
}

func fn2() {
    ll := llB(  "fn2")
    ll.A()
}

func main() {
    // For all the following, we assume latlearn.go is a local file. In your compile path & brought into your own module's namespace. You can look in ./buildrun.sh to see this example app's buildtime and runtime assumptions.

    latlearn_init2( []string { "print-yo", "fn1", "fn2"}) // spans of yours it should expect. in report order

    ll := llB(  "print-yo")
    fmt.Printf( "yo\n")
    ll.A()

    fn1()

    // let's print a basic report:
    latlearn_report()

    // We might want to include some context params in our report. If so, give the report generating function a list of strings. A context parameter might be some fact or metadata that the audience would feel is relevant, especially to make a correct interpretation of the metrics, and a correct deduction about their own next course of action.

    params := []string { "ver=1.2", "commit=whatever", "N=2", "cores=2"}
    latlearn_report2( params)

    // It just wrote a report (on latency stats) into a file at "./latlearn-report.txt"

    // If you look at the report you'll see a lot of ??? for some LL benchmark spans
    // because they were not performed. They are purely optional.

    // If you wish to perform them do this:
    latlearn_benchmarks()

    // Now let's write a new report. but let us put it in a different file

    // To change the report's file path, do like:
    latlearn_report_fpath = "latlearn-report2.txt"
    // There were no changes to any context param. we just want to see the benchmark metrics which should now appear in the report:
    latlearn_report2( params)

    // NOTE: All of LL's built-in spans (like for benchmarks) have names starting with "LL."
    // By the way, latlearn also measures the latency of its own report generation. It names that span "LL.lat-report"

    // Let's do some more measurements
    for i := 0; i < 100234; i++ {
        fn2()
    }

    // The below may start to feel a little verbose (boilerplatey) but we express it like this to reinforce that you can continually adjust the report params and file path as you go. which is helpful for testing, and for doing comparisons while doing troubleshooting or tuning work

    latlearn_report_fpath = "latlearn-report3.txt"
    latlearn_report2( params)

    // In this report you'll see metrics for fn2 reported for the first time.
    // Note that fn2's AVG latency is (almost certaily) smaller (faster) than fn1. That is because its code body is similar, except it lacks any Printf call. Typically, turning off any output writes (esp to log files) yields a significant reduction in latency (on a typical machine, ayway.) Though in this case fn2 contributes to much more of the program's overall total run latency, measured from process start to end. Because, unlike fn1, it was called 100,234 times.

    for i := 0; i < 2; i++ {
        ll = llB(  "totally-adhoc") // this was not in the initial set of tracked_spans
        // but this ad hoc span measurement will work correctly, anyway
        ll.A()
    }
    latlearn_report_fpath = "latlearn-report4.txt"
    latlearn_report() // next time you gen any report, that span will be added to end

    latlearn_should_report_builtins = false
    latlearn_report() // this report will NOT include any of latlearn's built-in spans
}
