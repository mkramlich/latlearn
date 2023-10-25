// latlearn.go, aka LatLearn
//     by Mike Kramlich
//
//     started  2023 September
//     last rev 2023 October 25
//
//     contact: groglogic@gmail.com
//     project: https://github.com/mkramlich/LatLearn

package latlearn

import (
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "runtime"
    "runtime/debug"
    "sort"
    "strings"
    "sync"
    "time"
)

// NOTE:
// This instrumentation lib *is* safe for concurrent use by multiple goroutines
// (potentially executing in actual parallel across multiple cores, etc.)
//
// How so?
//
// LatLearn maintains a per-process singleton "state" in terms of its in-memory
// latency statistics collection, and no direct access to it is exported. Instead,
// it enforces an inter-thread communication queue-based architecture (using
// immutable and/or "copy-on-write" messages), for memory safety. The actual
// measuring and collecting of all the stats (and other relevant settings of the
// runtime) happens under-the-hood. Any complex details which enforce this are
// hidden from the app client-side to keep it as simple as possible for them, and
// to let instrumentation be super easy to apply, and reason about.
//
// See the example apps for just how easy.

type latencyLearner struct {
    Name                string        // key into learners map
    Last                time.Duration // int64
    Cumul               time.Duration // int64
    Weight              int
    Min                 time.Duration // int64
    Max                 time.Duration // int64
    pair_underway       bool
    Pair_ever_completed bool
}

type variantLatencyLearner struct {
    *latencyLearner
    parent          *latencyLearner
}

type SpanSampleUnderway struct {
    Name    string    // a function of both Name & Variant fields form the key into learners map
    Variant string
    t1, t2  time.Time // NOTE there is no (need for a) t2 field
}

type ReplyMsg struct {
    ttype               string // values: "values"
    Name                string // of tracked span. and learners key. eg: "LL.no-op" or "somefn(N=100)"
    Pair_ever_completed bool
    Min                 time.Duration
    Last                time.Duration
    Max                 time.Duration
    Mean                int64
    Cumul               time.Duration
    Weight              int
}

type comm_msg struct {
    ttype         string   // values: "A", "values", "benchmarks", "report", "stop"
    params        []string // generic yet app-specific, like for report gen

    // next 4 fields are "value-passed" (or immutable) vars,
    // and which are equiv to a SpanSampleUnderway instance:
    name, variant string
    t1,   t2      time.Time
    ///////////////////////

    done          chan bool
    reply_chan    chan ReplyMsg
}

type latencyLearnerI interface {
    getLL()         *latencyLearner
    getVLL()        *variantLatencyLearner

    after2( dur time.Duration) // dur is int64. of ns. legit & precise?

    report( *os.File, string, time.Duration, time.Duration)
}

type SpanSampleUnderwayI interface {
    before()
    after()
    after_and_submit()

    A() (ok bool)
}

// NOTE: Apps can change the queue capacity dims, but will ONLY take effect if set BEFORE Init called
var Outer_queue_capacity      int = 1_000_000
var Inner_queue_capacity      int = 50

var comm_outer                chan comm_msg = nil // singleton per proc; consumed by 'serve' fn
var comm_inner                chan comm_msg = nil // singleton per proc; consumed by 'serve' fn

var init_oncer                sync.Once
var init_time                 time.Time
var init_completed            bool = false // to be explicit. we rely on this starting false

 // this structure (along with the singleton serve() goroutine) form the "heart" of LatLearn:
var learners                  map[string]latencyLearnerI

// tracked_spans built/modified ONLY by the Init and B fns
// it keeps a stable order of keys, for a better UX of the report
var tracked_spans             []string

var Should_report_builtins    bool = true
var Should_subtract_overhead  bool = false
var Report_fpath              string = "latlearn-report.txt"

var Overhead_samples_started  bool = false
var Overhead_samples_finished bool = false
var Overhead_samples_aborted  bool = false

var Benchmarks_started        bool = false
var Benchmarks_finished       bool = false


func ( ll *latencyLearner)        getLL()  *latencyLearner        { return  ll}
func ( ll *latencyLearner)        getVLL() *variantLatencyLearner { return nil}
func (vll *variantLatencyLearner) getLL()  *latencyLearner        { return vll.latencyLearner}
func (vll *variantLatencyLearner) getVLL() *variantLatencyLearner { return vll}

// for latlearn's internal use only
func latency_learner( span string) (ll *latencyLearner, found bool) {
    lli, found                 := learners[ span]
    if  !found {
        ll                      = new( latencyLearner)
        ll.Name                 = span
        learners[ span] = ll
    } else {
        ll                      = lli.getLL()
    }
    return ll, found
}

// for latlearn's internal use only
func variant_latency_learner( span string) (vll *variantLatencyLearner, found bool) {
    lli, found                 := learners[ span]
    if  !found {
        ll                     := new( latencyLearner)
        ll.Name                 = span
        vll                     = new( variantLatencyLearner)
        vll.latencyLearner      =  ll
        learners[ span] = vll
    } else {
        vll                     = lli.getVLL()
    }
    return vll, found
}

func span_key_form( name string, variant string) (key string) {
    if (variant != "") {
        // example: given span name of "somefn" and variant "N=200" then key is "somefn(N=200)"
        return fmt.Sprintf( "%s(%s)", name, variant)
    } else { return name}
}

// for internal, latlearn-only, use
func handle_ssu_A( name string, variant string, t1 time.Time, t2 time.Time) (ok bool) {
    //pre             := "latlearn.handle_ssu_A"

    dur             := t2.Sub( t1) // time.Duration. int64. of ns. legit & precise

    if     (variant != "") {
        parent_key  := span_key_form(           name, "")
        pll, found  := latency_learner(         parent_key)
        if     (pll == nil) { return false}
        if  !found  { tracked_spans = append( tracked_spans, parent_key)}

        variant_key := span_key_form(           name, variant)
        vll, found2 := variant_latency_learner( variant_key)
        if     (vll == nil) { return false}
        if  !found2 { tracked_spans = append( tracked_spans, variant_key)}

        vll.parent   = pll // indicates this is variant of parent span, part of family

        if (vll.parent         != nil) { vll.parent.after2(         dur)}
        if (vll.latencyLearner != nil) { vll.latencyLearner.after2( dur)}

    } else {
        key         := span_key_form(           name, "")
        ll, found   := latency_learner(         key)
        if      (ll == nil) { return false}
        if !found   { tracked_spans = append( tracked_spans, key)}
        ll.after2( dur)
    }
    return true
}

// for internal, latlearn-only, use
func handle_msg_A( msg comm_msg) (ok bool) {
    return handle_ssu_A( msg.name, msg.variant, msg.t1, msg.t2)
}

// for internal, latlearn-only, use
func handle_msg_values( msg comm_msg) {
    pre :=      "latlearn.handle_msg_values"
    //log.Printf( "%s: msg.name \"%s\", msg.variant \"%s\"\n", pre, msg.name, msg.variant)

    if (msg.reply_chan == nil) {
        log.Printf( "%s: reply_chan is nil so return early without replying\n", pre)
        return
    }

    key            := span_key_form( msg.name, msg.variant)
    //log.Printf( "%s: key will use: \"%s\"\n", pre, key)

    no_entry_found := ReplyMsg {
          ttype:               msg.ttype,
          Name:                key,
          Pair_ever_completed: false,
          Min:                 -1,
          Last:                -1,
          Max:                 -1,
          Mean:                -1,
          Cumul:               -1,
          Weight:              -1}

    lli, found     := learners[ key]
    if  !found {
        log.Printf( "%s: no entry found in learners for this span: key %s\n", pre, key)
        msg.reply_chan <- no_entry_found
        return
    }

    if (lli == nil) {
        log.Printf( "%s: learners entry lookup yielded an lli of nil: key %s\n", pre, key)
        msg.reply_chan <- no_entry_found
        return
    }

    ll := lli.getLL()

    //log.Printf( "%s: before calling ll.values\n", pre)

    name, pair_ever_completed, min, last, max, mean, cumul, weight := ll.values()

    //log.Printf( "%s: before pushing a normal replyMsg into reply_chan\n", pre)

    msg.reply_chan<- ReplyMsg {
          ttype:               msg.ttype,
          Name:                name,
          Pair_ever_completed: pair_ever_completed,
          Min:                 min,
          Last:                last,
          Max:                 max,
          Mean:                mean,
          Cumul:               cumul,
          Weight:              weight}
}

// for internal, latlearn-only, use
func handle_msg_report( msg comm_msg) {
    //log.Printf( "latlearn.handle_msg_report\n")

    report_inner( msg.params)

    if (msg.done != nil) {
        msg.done <- true
    }
}

// for internal, latlearn-only, use
func handle_msg_benchmarks( msg comm_msg) {
    //log.Printf( "latlearn.handle_msg_benchmarks\n")

    benchmarks_inner()

    if (msg.done != nil) {
        msg.done <- true
    }
}

// for internal, latlearn-only, use
func handle_comm_msg( msg comm_msg) (stop bool) {
    //log.Printf("latlearn.handle_comm_msg\n")

    switch   msg.ttype {
        case "A" :     _ = handle_msg_A(          msg)
        case "values":     handle_msg_values(     msg)
        case "benchmarks": handle_msg_benchmarks( msg)
        case "report":     handle_msg_report(     msg)
        case "stop":       return true
    }
    return false
}

// for internal, latlearn-only, use
func serve() {
    log.Printf( "latlearn.serve\n")

    for {
        select {
        case msg1 := <-comm_outer: if handle_comm_msg( msg1) { return} // msgs from outside (ie. apps)
        case msg2 := <-comm_inner: if handle_comm_msg( msg2) { return} // msgs from inside  (latlearn)
        }
    }
}

// for internal, latlearn-only, use
//
// NOTE: This fn, init_inner, is NOT called by the comm_outer/inner consuming
// "serve" fn singleton goroutine. Why not? Because that "serve" goroutine
// does NOT yet exist, when Init is called. Indeed, it is *created* by Init.
// Rather, this fn will instead be called by a (potentially disposable /
// reusable / generic) goroutine avail to (and assigned by) Golang's
// sync.Once.Do method implementation. (See the top-level Init and Init2 fns.)
//
func init_inner( spans_app []string) (already bool) { // span list should be for LLs (parent spans) not VLLs
    pre :=      "latlearn.init_inner"
    log.Printf( "%s\n", pre)

    if init_completed { return true} // should not be needed, because of init_oncer. thus to be extra sure

    learners = make( map[string]latencyLearnerI)

    // latlearn's built-in benchmark spans
    //     for purposes of comparison with the enduser's reported span metrics
    spans_latlearn_builtin := []string {
        "LL.no-op",                       "LL.fn-call-return",
        "LL.for-iters(n=1000)",           "LL.accum-ints(n=1000)",    "LL.add-int-literals(n=2)",
        "LL.add-str-literals(n=2)",       "LL.map-str-int-set",
        "LL.map-str-int-get(k=100,key0)", "LL.map-str-int-get(k=100,key49)",
        "LL.map-str-int-get(k=100,key99)",
        "LL.span-map-lookup",             "LL.sort-strs(n=10)",       "LL.log-hellos(n=10)",
        "LL.byte-array-make(n=1)",        "LL.byte-array-make(n=1k)", "LL.byte-array-make(n=100k)",
        "LL.exec-command(mac,sysctl)",
        "LL.exec-command(mac,pwd)",
        "LL.exec-command(mac,date)",
        "LL.exec-command(mac,host)",
        "LL.exec-command(mac,hostname)",
        "LL.exec-command(mac,uname)",
        "LL.exec-command(mac,ls)",
        "LL.exec-command(mac,df)",
        "LL.exec-command(mac,kill)",
        "LL.exec-command(mac,sleep=0.01s)",
        "LL.exec-command(mac,sleep=0.001s)",
        "LL.exec-command(mac,sleep=0.0001s)",
        "LL.exec-command(mac,sh-version)",
        "LL.benchmarks-total",            "LL.lat-report"}

    spans       := []string {}

    for _, span := range   spans_app {
        spans    = append( spans, span)
    }

    for _, span := range   spans_latlearn_builtin {
        spans    = append( spans, span)
    }

    //log.Printf( "%s: spans: %#v\n", pre, spans)

    for _, span := range spans {
        latency_learner( span)
    }

    tracked_spans  = spans

    if (Outer_queue_capacity < 100) { Outer_queue_capacity = 100}
    if (Inner_queue_capacity <  10) { Inner_queue_capacity =  10}

    log.Printf(
        "%s: comm queue capacities used: outer %d, inner %d\n",
        pre, Outer_queue_capacity, Inner_queue_capacity)

    comm_outer     = make( chan comm_msg, Outer_queue_capacity)
    comm_inner     = make( chan comm_msg, Inner_queue_capacity)

    init_time      = time.Now()
    init_completed = true

    go serve() // <- in a sense, that thread becomes the "beating heart" of LatLearn

    //log.Printf( "%s: END\n", pre)
    return false
}

func Init() {
    init_oncer.Do( func() {
        init_inner( []string {})
    })
}

func Init2( spans_app []string) { // span list should be for LLs (parent spans) not VLLs
    init_oncer.Do( func() {
        init_inner( spans_app)
    })
}

func (ssu *SpanSampleUnderway) before() {
    ssu.t1 = time.Now()
}

func ssu_before( name string, variant string) *SpanSampleUnderway {
    ssu := &SpanSampleUnderway{ Name:name, Variant:variant}
    ssu.before()
    return ssu
}

func B( name string) *SpanSampleUnderway {
    //log.Printf( "B: name %s\n", name)

    if !init_completed { return nil} // TODO maybe add a separate "ok bool" return param

    return ssu_before( name, "")
}

func B2( name string, variant string) *SpanSampleUnderway {
    //log.Printf( "B2: name %s\n", name)

    if !init_completed { return nil} // TODO maybe add a separate "ok bool" return param

    return ssu_before( name, variant)
}

// for latlearn's internal use only
func (ll *latencyLearner) after2( dur time.Duration) { // dur is int64. of ns. legit & precise?
    //log.Printf("latencyLearner.after2: name %s\n", ll.name)

    //log.Printf( "%s before: %#v ms\n",         ll.name, ll.t1) // lg num printed is ms beyond the sec
    //log.Printf( "%s after : %#v ms\n",         ll.name, t2)
    //log.Printf( "%s dur   : %#v ns precise\n", ll.name, dur) // nanos. (1/1000 of a milli)

    ll.Last    = dur
    ll.Cumul  += dur
    ll.Weight ++

    if ll.Pair_ever_completed {
        if ( dur < ll.Min) {ll.Min = dur}
        if ( dur > ll.Max) {ll.Max = dur}
    } else {
        ll.Min = dur
        ll.Max = dur
    }

    ll.pair_underway       = false
    ll.Pair_ever_completed = true
}

// for latlearn-internal use only
func (ssu *SpanSampleUnderway) after() {
    ssu.t2 = time.Now()
}

// like after_and_submit but does NOT use channels, just updates the LL in the map directly
func (ssu *SpanSampleUnderway) after_and_update() (ok bool) {
    if !init_completed { return false}

    ssu.after()

    return handle_ssu_A( ssu.Name, ssu.Variant, ssu.t1, ssu.t2)
}

// for latlearn-internal use only
func (ssu *SpanSampleUnderway) after_and_submit( comm chan comm_msg) (ok bool) {
    if !init_completed { return false}

    ssu.after()

    comm <- comm_msg{
                ttype:   "A",
                name:    ssu.Name,
                variant: ssu.Variant,
                t1:      ssu.t1,
                t2:      ssu.t2}
    return true
}

func (ssu *SpanSampleUnderway) A() (ok bool) {
    return ssu.after_and_submit( comm_outer)
}

func Latency_measure_self_sample( n int) (ok bool) {
    if !init_completed { return false}

    // The purpose of this fn is to (try to) measure/estimate the latency cost
    // of a LatLearn measurement. In other words, learn the overhead that our
    // instrumentation imposes upon each covered span. However, there are lots
    // of subtle issues and (de facto) asymptotic complexities which lurk here.
    //
    // For "best" results, try to call this fn under "normal" runtime conditions
    // for your app. Also, note that the larger the "n" param (the larger the
    // total number of "LL-no-op" samples we collect, overall), the more
    // "accurate" and helpful your deductions about its meaning will be.

    if (n < 0) { n = 1_000_000} // we'll do it 1 million times, hoping to mitigate (somewhat, maybe) the effects of host load spikes, GC runs, etc

    Overhead_samples_started  = true
    Overhead_samples_finished = false
    Overhead_samples_aborted  = false
    for i := 0; i < n; i++ {
        ssu   := ssu_before( "LL.no-op", "")
        // ... some app-specific code (of latency measurement interest) would normally be here ...
        if ok := ssu.A(); !ok {
            Overhead_samples_finished = true
            Overhead_samples_aborted  = true
            return false
        }
    }
    Overhead_samples_finished = true

    return true
}

// for internal, latlearn-only, use
func measure_overhead_estimate() (overhead time.Duration, exists bool) {
    if !init_completed              { return -1, false}

    lli, found := learners[ "LL.no-op"]

    if  !found                      { return -1, false}

    ll_noop    := lli.getLL()

    if !ll_noop.Pair_ever_completed { return -1, false}

    // TODO also check for these conditions:
    //      Overhead_samples_finished is true
    //      Overhead_samples_aborted  is false

    return ll_noop.Min, true
}

// for latlearn's internal use only
func noop_fn_for_benchmark_calls() {
}

// for latlearn's internal use only
func benchmark_exec( name_sub string, exe string, args []string) { // name_sub like "mac,sysctl"
    for i      := 0; i < 1000; i++ {
        cmd    := exec.Command( exe, args...)
        span   := fmt.Sprintf( "LL.exec-command(%s)", name_sub)
        ll     := ssu_before( span, "")
        if err := cmd.Run(); (err != nil) {
            // TODO call variant of after method (and/or with arg) to indicate it failed
        }
        ll.after_and_update()
    }
}

func benchmarks_inner() (performed bool) {
    pre :=      "latlearn.benchmarks_inner"
    log.Printf( "%s\n", pre)

    if !init_completed { return false}

    Benchmarks_started = true

    ll_bt     := ssu_before( "LL.benchmarks-total","")

    for i     := 0; i < 1000; i++ {
        ll    := ssu_before( "LL.fn-call-return","")
        noop_fn_for_benchmark_calls()
        ll.after_and_update()
    }

    for i     := 0; i < 1000; i++ {
        ll    := ssu_before( "LL.for-iters(n=1000)","")
        for j := 0; j < 1000; j++ {
        }
        ll.after_and_update()
    }

    for i     := 0; i < 1000; i++ {
        ll    := ssu_before( "LL.accum-ints(n=1000)","")
        v     := 0
        for j := 0; j < 1000; j++ {
            v += j
        }
        ll.after_and_update()
    }

    for i    := 0; i < 1000; i++ {
        ll   := ssu_before( "LL.add-int-literals(n=2)","")
        a    := (1 + 2)
        _     = a // make closer to real, and compiler happy
        ll.after_and_update()
    }

    for i    := 0; i < 1000; i++ {
        ll   := ssu_before( "LL.add-str-literals(n=2)","")
        c    := ("a" + "b")
        _     = c // make closer to real, and compiler happy
        ll.after_and_update()
    }

    for i     := 0; i < 1000; i++ {
        m     := make( map[string]int)
        ll    := ssu_before( "LL.map-str-int-set","")
        m[ "foo"] = 5
        ll.after_and_update()
    }

    keys    := []string {}
    for   k := 0; k < 100; k++ {
        keys = append( keys, fmt.Sprintf( "key%d",k))
    } // now have a common set of 100 keys, each with a distinct string value. suitable for map
    for i     := 0; i < 1000; i++ {
        m     := make( map[string]int)
        for _, key := range keys {
            m[ key] = 5
        } // we've populated the map with 100 entries
        ll    := ssu_before( "LL.map-str-int-get(k=100,key0)","")
        _ = m[ "key0"]
        ll.after_and_update()
        ll     = ssu_before( "LL.map-str-int-get(k=100,key49)","")
        _ = m[ "key49"]
        ll.after_and_update()
        ll     = ssu_before( "LL.map-str-int-get(k=100,key99)","")
        _ = m[ "key99"]
        ll.after_and_update()
    }

    for i    := 0; i < 1000; i++ {
        ll   := ssu_before( "LL.span-map-lookup","")
        a, b := learners[ "LL.no-op"]
        ll.after_and_update()
        _     = a // yes, is reason why we are doing this
        _     = b // ditto
    }

    strs         := []string { "Zelda", "Hoth", "Abro",  "Daneel", "Tempest", "Cthulhu", "Bonk", "Arky","Ys", "Jude Law"}
    for i        := 0; i < 1000; i++ {
        strs2    := []string {}
        for _, s := range strs {
            strs2 = append( strs2, s)
        }
        ll   := ssu_before( "LL.sort-strs(n=10)","")
        sort.Strings( strs2) // sorts the given slice in-place
        ll.after_and_update()
    }

    ll       := ssu_before( "LL.log-hellos(n=10)","")
    for i    := 0; i < 10; i++ {
        log.Printf( "%s: log measure test\n", pre)
    }
    ll.after_and_update()

    for i     := 0; i < 1000; i++ {
        ll    := ssu_before( "LL.byte-array-make(n=1)","")
        array := make( []byte,      1)
        _      = array // to quiet the compiler
        ll.after_and_update()

        ll     = ssu_before( "LL.byte-array-make(n=1k)","")
        array  = make( []byte,   1000)
        _      = array // to quiet the compiler
        ll.after_and_update()

        ll     = ssu_before( "LL.byte-array-make(n=100k)","")
        array  = make( []byte, 100000)
        _      = array // to quiet the compiler
        ll.after_and_update()
    }

    if (runtime.GOOS == "darwin") {
        benchmark_exec( "mac,sysctl",        "/usr/sbin/sysctl", []string {})
        benchmark_exec( "mac,pwd",           "/bin/pwd",         []string {})
        benchmark_exec( "mac,date",          "/bin/date",        []string {})
        benchmark_exec( "mac,host",          "/usr/bin/host",    []string {})
        benchmark_exec( "mac,hostname",      "/bin/hostname",    []string {})
        benchmark_exec( "mac,uname",         "/usr/bin/uname",   []string {})
        benchmark_exec( "mac,ls",            "/bin/ls",          []string {})
        benchmark_exec( "mac,df",            "/bin/df",          []string {})
        benchmark_exec( "mac,kill",          "/bin/kill",        []string {})
        benchmark_exec( "mac,sleep=0.01s",   "/bin/sleep",       []string {"0.01",})
        benchmark_exec( "mac,sleep=0.001s",  "/bin/sleep",       []string {"0.001",})
        benchmark_exec( "mac,sleep=0.0001s", "/bin/sleep",       []string {"0.0001",})
        benchmark_exec( "mac,sh-version",    "/bin/sh",          []string {"--version",})
    }

    ll_bt.after_and_update()

    Benchmarks_finished = true

    return true
}

func (ll *latencyLearner) values() ( name string, pair_ever_completed bool, min time.Duration, last time.Duration, max time.Duration, mean int64, cumul time.Duration, weight int) {
    mean, weight = ll.mean()
    return ll.Name, ll.Pair_ever_completed, ll.Min, ll.Last, ll.Max, mean, ll.Cumul, weight
}

func (ll *latencyLearner) mean() ( mean_latency int64, weight int) {
    mean_latency      = int64( -1)
    weight            = ll.Weight
    if (weight        > 0) {
        cumul        := ll.Cumul.Nanoseconds()
        mean_latency  = cumul / int64( weight)
    }
    return mean_latency, weight
}

// for latlearn's internal use only
func to_file(              f *os.File, txt string) {
    _, _ = io.WriteString( f,          txt + "\n") // TODO error handling
}

// for latlearn's internal use only
func reverse_string_array( in []string) (out []string) {
    // NOTE: compiler refuses to let one use: sort.Reverse( in)
    // TODO  I should not need to write my own fn to do this

    out     = []string {}
    for i  := (len( in) - 1); i >= 0; i-- {
        out = append( out, in[i])
    }
    return out
}

// for latlearn's internal use only
// TODO rewrite: try again to replace with impl using Golang stdlib
func number_grouped( val int64, sep string) string { // sep value like "," or " "
    // for the "digit grouping" formatted print of a number
    // example of results:
    //      12 (with any sep)    to        "12"
    //    1234 (with sep of ",") to     "1,234"
    // 1222333 (with sep of " ") to "1 222 333"

    s      := fmt.Sprintf(   "%d", val)
    s_pcs  := strings.Split( s,    "")

    // reverse it
    s_pcs2 := reverse_string_array( s_pcs)

    // add comma delim, every 3 digits, to group them for easier human reading:
    s2         := ""
    for i, pc  := range s_pcs2 {
        if (i  != 0) && ((i % 3) == 0) {
            s2 += sep
        }
        s2     += pc
    }

    s_pcs3 := strings.Split(        s2,     "")
    s_pcs4 := reverse_string_array( s_pcs3)
    s3     := strings.Join(         s_pcs4, "")

    return s3
}

// for latlearn's internal use only
func overhead_comp( metric_in int64, overhead int64) (metric_out int64) { // "comp" means compensate
    if Should_subtract_overhead {
        metric_out = (metric_in - overhead)
        if (metric_out < 0) { metric_out = 0} // We apply this minimum cap on metric_out because its possible for our "best" discovered LL.no-op min field value (in a particular process session) to not reflect the absolute truest minimum value possible during that run. In those (rare) edge cases, if we did NOT apply this adjustment, the reported latency could appear as a (usually small) negative number of ns. Since that is obviously nonsense (ie. impossible, in reality), we "patch" it here to ensure the reported value is never *less* than 0 ns. In other words, our premise/bias is that *almost* everything takes *some* time, and that any negative "measured" latency can be due *only* to either a bug or a calculation quirk, caused by bad math or imperfect/incomplete effort at evidence gathering.
    } else {
        metric_out =  metric_in
    }
    return metric_out
}

// for latlearn's internal use only
func (ll *latencyLearner) report( f *os.File, name_field string, since_init time.Duration, overhead time.Duration) { // time.Duration is int64 ns
    line := ""

    if ll.Pair_ever_completed {
        min              := int64( ll.Min)
        if (overhead != -1) && (ll.Name != "LL.no-op") {
            min           = overhead_comp( int64( ll.Min),  int64( overhead))
        }
        min_txt          := fmt.Sprintf( "%15s", number_grouped( int64( min), ","))

        last             := overhead_comp( int64( ll.Last), int64( overhead))
        last_txt         := fmt.Sprintf( "%15s", number_grouped( int64( last), ","))

        max              := overhead_comp( int64( ll.Max),  int64( overhead))
        max_txt          := fmt.Sprintf( "%15s", number_grouped( int64( max), ","))

        mean_txt         := "???,???,???,???"
        weight_txt       :=     "???,???,???"
        tf_txt           :=        "????????"
        weight           := ll.Weight

        if (weight        > 0) {
            cum_ns       := ll.Cumul.Nanoseconds() // int64. ns

            lat_mean     := cum_ns / int64( weight)

            mean         := overhead_comp(         lat_mean, int64(overhead))
            mean_txt      = number_grouped( int64( mean),   ",")

            weight_txt    = number_grouped( int64( weight), ",")
            my_frac      := float64( cum_ns) / float64( since_init) // float64. fraction
            tf_txt        = fmt.Sprintf( "%8f", my_frac)
        }

        rest_fields := "%15s | %15s | %15s | %15s | w %11s | tf %8s | %-21s"
        format      := name_field + ": " + rest_fields
        line         = fmt.Sprintf(
                           format,
                           ll.Name,    min_txt, last_txt, max_txt, mean_txt,
                           weight_txt, tf_txt,  ll.Name)
    } else {
        // min, last, max, mean, weight of mean (# of calls for this span), time fraction (of current time difference since Iinit, in/under this span)
        rest_fields := "???,???,???,??? | ???,???,???,??? | ???,???,???,??? | ???,???,???,??? | w ???,???,??? | tf ???????? | %-21s"
        format      := name_field + ": " + rest_fields
        line         = fmt.Sprintf(
                           format,
                           ll.Name, ll.Name)
    }

    to_file( f, line)
}

// for latlearn's internal use only
func mac_sysctl( key string) (value string) {
    cmd := exec.Command( "/usr/sbin/sysctl", key)

    var out []byte
    var err error

    if  out, err = cmd.CombinedOutput(); (err != nil) {
        //log.Printf( "latlearn report's mac sysctl exec failed. output: %s\n", string( out))
        //log.Fatal( err)
        return ""
    }
    out_str := string( out)
    value_to_line_end := strings.TrimPrefix( out_str, key + ": ")
    return strings.TrimSpace( value_to_line_end)
}

// for latlearn's internal use only
func mac_sysctl_report_line( key string, f *os.File) {

    value := mac_sysctl( key)
    line  := fmt.Sprintf( "%-27s: %s\n", key, value)
    io.WriteString( f, line)
}

// for latlearn's internal use only
func write_info_about_mac_host_to_report( f *os.File) {

    mac_sysctl_report_line( "kern.ostype",                 f)
    mac_sysctl_report_line( "kern.osproductversion",       f)
    mac_sysctl_report_line( "kern.osrelease",              f)
    mac_sysctl_report_line( "kern.osrevision",             f)
    mac_sysctl_report_line( "kern.version",                f)
    mac_sysctl_report_line( "user.posix2_version",         f)
    mac_sysctl_report_line( "machdep.cpu.brand_string",    f)
    mac_sysctl_report_line( "machdep.cpu.core_count",      f)
    mac_sysctl_report_line( "machdep.cpu.thread_count",    f)
    mac_sysctl_report_line( "machdep.memmap.Conventional", f)
    mac_sysctl_report_line( "hw.memsize",                  f)
    mac_sysctl_report_line( "hw.pagesize",                 f)
    mac_sysctl_report_line( "hw.cpufrequency",             f)
    mac_sysctl_report_line( "hw.busfrequency",             f)
}

func report_inner( params []string) (ok bool) {
    pre :=      "latlearn.report_inner"
    //log.Printf( "%s\n", pre)

    if !init_completed { return false}

    ssu := ssu_before( "LL.lat-report", "")

    f,  err := os.Create( Report_fpath)
    if (err != nil) {
        log.Printf(
            "%s: could not create file for report: path '%s', err %#v\n",
            pre, Report_fpath, err)
        return false
    }
    defer func() { if (f != nil) { f.Close()}}()

    io.WriteString( f, "Latency Report (https://github.com/mkramlich/latlearn)\n\n")

    io.WriteString( f, fmt.Sprintf( "Outer_queue_capacity:        %d\n", Outer_queue_capacity))
    io.WriteString( f, fmt.Sprintf( "Inner_queue_capacity:        %d\n", Inner_queue_capacity))

    io.WriteString( f, fmt.Sprintf( "Overhead_samples_started:    %v\n", Overhead_samples_started))
    io.WriteString( f, fmt.Sprintf( "Overhead_samples_finished:   %v\n", Overhead_samples_finished))
    io.WriteString( f, fmt.Sprintf( "Overhead_samples_aborted:    %v\n", Overhead_samples_aborted))

    io.WriteString( f, fmt.Sprintf( "Benchmarks_started:          %v\n", Benchmarks_started))
    io.WriteString( f, fmt.Sprintf( "Benchmarks_finished:         %v\n", Benchmarks_finished))

    io.WriteString( f, fmt.Sprintf( "Should_report_builtins:      %v\n", Should_report_builtins))

    io.WriteString( f, fmt.Sprintf( "Should_subtract_overhead:    %v\n", Should_subtract_overhead))
    if Should_subtract_overhead {
        io.WriteString( f,          "metric treated as overhead:  LL.no-op, min\n")
    }

    t2         := time.Now()         // time.Time
    since_init := t2.Sub( init_time) // time.Duration. int64. ns. legit/precise?
    si_txt     := number_grouped( int64( since_init), ",")
    time_param := fmt.Sprintf(      "since LL init:               %s ns\n\n", si_txt)
    io.WriteString( f, time_param)

    io.WriteString( f, fmt.Sprintf( "Go ver:                      %s\n", runtime.Version()))
    io.WriteString( f, fmt.Sprintf( "GOARCH:                      %s\n", runtime.GOARCH))
    io.WriteString( f, fmt.Sprintf( "GOOS:                        %s\n", runtime.GOOS))
    io.WriteString( f, fmt.Sprintf( "NumCPU:                      %d\n", runtime.NumCPU()))
    io.WriteString( f, fmt.Sprintf( "GOMAXPROCS:                  %d\n", runtime.GOMAXPROCS( -1)))
    io.WriteString( f, fmt.Sprintf( "NumGoroutine:                %d\n", runtime.NumGoroutine()))

    mem_limit     := debug.SetMemoryLimit( -1)
    mem_limit_str := number_grouped( mem_limit, ",")
    io.WriteString( f, fmt.Sprintf( "SetMemoryLimit:              %s bytes\n", mem_limit_str))

    gogc := ""
    if val, ok := os.LookupEnv(     "GOGC"); ok {           gogc = val}
    io.WriteString( f, fmt.Sprintf( "GOGC:                        %s\n", gogc))

    if (runtime.GOOS == "darwin") {
        write_info_about_mac_host_to_report( f)
    }

    term_rows  := "?"
    term_cols  := "?"
    if val, ok := os.LookupEnv(     "LINES");   ok { term_rows = val}
    if val, ok := os.LookupEnv(     "COLUMNS"); ok { term_cols = val}
    io.WriteString( f, fmt.Sprintf( "LINES:                       %s\n", term_rows))
    io.WriteString( f, fmt.Sprintf( "COLUMNS:                     %s\n", term_cols))

    host       := ""
    if val, ok := os.LookupEnv(     "HOST"); ok { host = val}
    io.WriteString( f, fmt.Sprintf( "HOST:                        %s\n", host))

    term       := ""
    if val, ok := os.LookupEnv(     "TERM"); ok { term = val}
    io.WriteString( f, fmt.Sprintf( "TERM:                        %s\n", term))

    io.WriteString( f, "\n")

    // Context Params (which may impact interpretation of the reported span metrics)
    // TODO prob change the params printout to just one K:V pair per line
    for i, param      := range params {
        txt           := ""
        if         (i == 0) {
            txt        = param
        } else if ((i != 0) && ((i % 4) == 0)) { // TODO do this better
            txt        = "\n" + param
        } else if  (i  > 0) {
            txt        = ", " + param
        }
        io.WriteString( f, txt)
    }
    if len( params) > 0 {
        io.WriteString( f, "\n")
    }
    io.WriteString(     f, "\n")

    longest_name := -1
    for _, name  := range tracked_spans {
        n        := len( name)
        if (longest_name    == -1) {
            longest_name     = n
        } else {
            if (n > longest_name) {
                longest_name = n
            }
        }
    }

    name_field  := fmt.Sprintf( "%%-%ds", longest_name)
    rest_fields := "%15s | %15s | %15s | %15s | %13s | %11s | %-21s"
    format      := name_field + ": " + rest_fields

    // write a report entry (to the file) for the latency stats on each tracked span:
    header      := fmt.Sprintf(
                       format,
                       "span", "min (ns)", "last (ns)", "max (ns)", "mean (ns)",
                       "weight (B&As)", "time frac", "span")
    to_file( f, header)

    var overhead time.Duration = -1 // this value signals that we have no usable estimate
    if Should_subtract_overhead {
        overhead, _ = measure_overhead_estimate()
    }

    for _, span := range tracked_spans {
        if !Should_report_builtins && strings.HasPrefix( span,"LL.") { continue}
        learners[ span].report( f, name_field, since_init, overhead) // TODO add found-in-map guard
    }

    ok = ssu.after_and_update()
    return ok
}

func Report() (ok bool) {
    //log.Printf( "latlearn.Report\n")

    if !init_completed { return false}

    done_chan  := make( chan bool, 1)
    comm_outer <- comm_msg{ ttype: "report", done:done_chan}
    <- done_chan
    return true
}

func Report2( params []string) (ok bool) {
    //log.Printf( "latlearn.Report2\n")

    if !init_completed { return false}

    done_chan  := make( chan bool, 1)
    comm_outer <- comm_msg{ ttype: "report", params:params, done:done_chan}
    <- done_chan
    return true
}

func Benchmarks() (ok bool) {
    //log.Printf( "latlearn.Benchmarks\n")

    if !init_completed { return false}

    done_chan  := make( chan bool, 1)
    comm_outer <- comm_msg{ ttype: "benchmarks", done:done_chan}
    <- done_chan
    return true
}

func Values( span string) (values ReplyMsg, ok bool) {
    //.pre :=      "latlearn.Values"
    //log.Printf( "%s:\n", pre)

    if !init_completed { return ReplyMsg{}, false}

    reply_chan := make( chan ReplyMsg, 1)

    //log.Printf( "%s: before pushing comm_msg into comm_outer\n", pre)

    comm_outer <- comm_msg{ ttype: "values", name: span, reply_chan: reply_chan}

    //log.Printf( "%s: before pulling replyMsg out of reply_chan\n", pre)

    values       = <-reply_chan

    //log.Printf( "%s: after  pulling replyMsg out of reply_chan\n", pre)

    return values, true
}

func Overhead() (overhead ReplyMsg, ok bool) {
    //log.Printf( "latlearn.Overhead\n")

    // This fn's caller should 1st check if our response is ok.
    // Then (if ok true), check ALSO if overhead.pair_ever_completed is true.
    // If so, then use the value of overhead.min for what latlearn considers
    // its per-span overhead cost.
    //
    // It might also be valuable for the app to confirm Overhead_samples_finished
    // is true. And that Overhead_samples_aborted is false. By default we try to
    // capture 1M samples of LL.no-op span. These two flags ensure that all 1M
    // samples were carried out and finished, without error.

    return Values( "LL.no-op")
}

func Stop() (ok bool) {
    log.Printf( "latlearn.Stop\n")

    if !init_completed { return false}

    comm_outer <- comm_msg{ ttype: "stop"}

    return true
}

