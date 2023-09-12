// latlearn.go
//     by Mike Kramlich. in 2023 September
//     contact: groglogic@gmail.com
//     project: https://github.com/mkramlich/latlearn

package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "sort"
    "time"
)

type LatencyLearner struct { // assumes only single-thread/one-goroutine-at-a-time access
    name                    string
    t1                      time.Time // NOTE there is no (need for a) t2 field
    latency_recent          time.Duration
    cumul_latency           time.Duration
    weight_of_cumul_latency int
    after_called            bool
}

func latency_measure_self_sample() {
    // Below is to help measure (1) the cost of measuring itself, and
    // and (2) to do so around the time of a real measure occurring
    ll_noop := latency_learners[ "no-op"]
    ll_noop.before()
    ll_noop.after()
}

func latency_measure_of_various_benchmark_tasks() {
    pre   := "latency_measure_of_various_benchmark_tasks"

    // TODO also: accum to an int 1000 times. do 1000 fn calls. some arg-driven O(n) variants
    // TODO variants with logging on/off, etc.

    ll    := llB( "log-10-hellos") // TODO ensure we have 2 variants: logging_enabled on vs off
    for i := 0; i < 10; i++ {
        log.Printf( "%s: log measure test\n", pre)
    }
    ll.A()

    strs  := []string { "Zelda", "Hoth", "Abro",  "Daneel", "Tempest", "Cthulhu", "Bonk", "Arky","Ys", "Jude Law"}
    ll     = llB( "sort-10-strs")
    sort.Strings( strs)
    ll.A()
}

func (ll *LatencyLearner) before() {
    // Capture Time Before
    ll.t1 = time.Now() // type is time.Time
}

func (ll *LatencyLearner) B() {
    // since call wrapped with another call, overhead latency impact a tiny bit higher. but gives a smaller code-on-screen footprint at point-of-instrumentation, for dev to parse
    // tradeoffs, haha
    ll.before()
}

func (ll *LatencyLearner) after() {

    // Capture Time After
    t2  := time.Now()
    dur := t2.Sub( ll.t1) // in ns, precise

    log.Printf( "%s before: %#v ms\n",         ll.name, ll.t1) // lg num printed is ms beyond the sec
    log.Printf( "%s after : %#v ms\n",         ll.name, t2)
    log.Printf( "%s dur   : %#v ns precise\n", ll.name, dur) // nanos. (1/1000 of a milli)

    ll.latency_recent           = dur
    ll.cumul_latency           += dur
    ll.weight_of_cumul_latency ++

    ll.after_called = true
}

func (ll *LatencyLearner) A() {
    ll.after()
    latency_measure_self_sample()
}

func llB( name string) *LatencyLearner {
    ll := latency_learners[ name]
    ll.before()
    return ll
}

func llA( name string) {
    latency_learners[ name].after()
    latency_measure_self_sample()
}

func (ll *LatencyLearner) values() ( string, time.Duration, time.Duration, int, bool) {
    return ll.name, ll.latency_recent, ll.cumul_latency, ll.weight_of_cumul_latency, ll.after_called
}

func (ll *LatencyLearner) avg_latency() ( avg_latency int64, weight int) {
    avg_latency      = int64( -1)
    weight           = ll.weight_of_cumul_latency
    if weight        > 0 {
        cumul       := ll.cumul_latency.Nanoseconds()
        avg_latency  = cumul / int64( weight)
    }
    return avg_latency, weight
}

func to_file( f *os.File, r int, c int, txt string) { // TODO r,c use or delete
    _, _ = io.WriteString( f, txt + "\n") // TODO error handling
}

func (ll *LatencyLearner) report( f *os.File, r int, c int) {

    if !ll.after_called { // TODO handle more gracefully case when this happens
        log.Printf( "LatencyLearner.report: returning early, not drawing anything, cuz after_called is false\n")
        return
    }

    // this latency metric var is a time.Duration:
    txt := fmt.Sprintf(    "%9d ns LAST %s", ll.latency_recent, ll.name)
    to_file( f, r, c, txt)

    weight       := ll.weight_of_cumul_latency
    if weight     > 0 {
        cum_ns   := ll.cumul_latency.Nanoseconds() // int64 of ns
        avg_lat  := cum_ns / int64( weight)
        t2       := time.Now()
        play_dur := t2.Sub( play_started) // time.Duration. int64. of ns, precise
        my_frac  := float64( cum_ns) / float64( play_dur) // yields float64, of a fraction

        txt = fmt.Sprintf( "%9d ns AVG   w%-5d (playfrac %8f)", avg_lat, weight, my_frac)
    } else {
        txt =              "????????? ns AVG   w0     (playfrac ????????"
    }
    to_file( f, r+1, c, txt)
}

