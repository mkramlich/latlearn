// latlearn.go aka LatencyLearner
//     by Mike Kramlich
//
//     started  2023 September
//     last rev 2023 October 21
//
//     contact: groglogic@gmail.com
//     project: https://github.com/mkramlich/latlearn

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
    "time"
)

// IMPORTANT: For now, the latlearn lib assumes only single-thread (one-goroutine-at-a-time) access.

type LatencyLearner struct {
    Name                string
    t1                  time.Time     // NOTE there is no (need for a) t2 field
    Last                time.Duration // int64
    Cumul               time.Duration // int64
    Weight              int
    Min                 time.Duration // int64
    Max                 time.Duration // int64
    pair_underway       bool
    Pair_ever_completed bool
}

type VariantLatencyLearner struct {
    *LatencyLearner
    parent          *LatencyLearner
}

type LatencyLearnerI interface {
    GetLL()         *LatencyLearner
    GetVLL()        *VariantLatencyLearner

    Before()
    After()

    A()

    report( *os.File, string, time.Duration, time.Duration)
}

func ( ll *LatencyLearner)        GetLL()  *LatencyLearner        { return  ll}
func ( ll *LatencyLearner)        GetVLL() *VariantLatencyLearner { return nil}
func (vll *VariantLatencyLearner) GetLL()  *LatencyLearner        { return vll.LatencyLearner}
func (vll *VariantLatencyLearner) GetVLL() *VariantLatencyLearner { return vll}

// tracked_spans built/modified ONLY by the Init and B fns
// it keeps a stable order of keys, for a better UX of the report
var tracked_spans            []string
var Report_fpath             string = "latlearn-report.txt"
var Learners                 map[string]LatencyLearnerI
var init_time                time.Time
var init_completed           bool = false // explicit. we expect this starts false
var Should_report_builtins   bool = true
var Should_subtract_overhead bool = false


// for latlearn's internal use only
func latency_learner( span string) (ll *LatencyLearner, found bool) {
    lli, found                 := Learners[ span]
    if  !found {
        ll                      = new( LatencyLearner)
        ll.Name                 = span
        Learners[ span] = ll
    } else {
        ll                      = lli.GetLL()
    }
    return ll, found
}

// for latlearn's internal use only
func variant_latency_learner( span string) (vll *VariantLatencyLearner, found bool) {
    lli, found                 := Learners[ span]
    if  !found {
        ll                     := new( LatencyLearner)
        ll.Name                 = span
        vll                     = new( VariantLatencyLearner)
        vll.LatencyLearner      =  ll
        Learners[ span] = vll
    } else {
        vll                     = lli.GetVLL()
    }
    return vll, found
}

func Init2( spans_app []string) { // span list should be for LLs (parent spans) not VLLs
    //pre :=      "latlearn.Init2"
    //log.Printf( "%s\n", pre)

    Learners = make( map[string]LatencyLearnerI)

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
    init_time      = time.Now()
    init_completed = true

    //log.Printf( "%s: END\n", pre)
}

func Init() {
     Init2( []string {})
}

func (ll *LatencyLearner) Before() {
    //log.Printf( "LatencyLearner.Before: name %s\n", ll.Name)

    // record the time BEFORE span-of-interest begins
    ll.t1            = time.Now() // type is time.Time
    ll.pair_underway = true
}

func (vll *VariantLatencyLearner) Before() {
    //log.Printf( "LatencyLearner.Before: name %s\n", ll.Name)

    // record the time BEFORE span-of-interest begins
    vll.parent.t1            = time.Now() // we CAN assume safely that vll.parent != nil
    vll.parent.pair_underway = true

    vll.t1                   = vll.parent.t1
    vll.pair_underway        = true
}

func B( name string) *LatencyLearner {
    //log.Printf( "B: name %s\n", name)

    if !init_completed { // allows lazy init of latlearn, upon first call to B
        Init()
    }

    ll, found := latency_learner( name)
    if !found { // if was an ad hoc span? meaning this span was NOT already tracked
        tracked_spans = append( tracked_spans, name) // we assume here that tracked_spans will stay in sych with the set of keys in Learners. except tracked_spans adds extra notion of preserving a stable order to the keys (relied on in the report)
    }
    ll.Before()
    return ll
}

func B2( name string, variant string) *VariantLatencyLearner {
    //log.Printf( "B2: name %s\n", name)

    if !init_completed { // allows lazy init of latlearn, upon first call to B2
        Init()
    }

    ll, found    := latency_learner( name) // example name: "somefn"
    if !found   { tracked_spans = append( tracked_spans, name)}

    variant_name := fmt.Sprintf( "%s(%s)", name, variant) // example variant_name: "somefn(N=200)"
    vll, found2  := variant_latency_learner( variant_name)
    if  !found2 { tracked_spans = append( tracked_spans, variant_name)}
    vll.parent    = ll // to note this span is a variant of a parent span, and part of a span family
    vll.Before() // the called Before fn here will ALSO in effect call ll.Before()

    return vll
}

// for latlearn's internal use only
func (ll *LatencyLearner) after2( dur time.Duration) { // dur is int64. of ns. legit & precise?
    //log.Printf("LatencyLearner.after2: name %s\n", ll.name)

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

    ll.pair_underway            = false
    ll.Pair_ever_completed      = true
}

func (ll *LatencyLearner) After() {
    // record the time AFTER span-of-interest ended
    t2  := time.Now()
    dur := t2.Sub( ll.t1) // time.Duration. int64. of ns. legit & precise?

    ll.after2( dur)
}

func (vll *VariantLatencyLearner) After() {
    // record the time AFTER span-of-interest ended
    t2  := time.Now()
    dur := t2.Sub( vll.t1) // time.Duration. int64. of ns. legit & precise?

    vll.parent.after2(         dur)
    vll.LatencyLearner.after2( dur)
}

func (ll *LatencyLearner) A() {
    ll.After()
}

func (vll *VariantLatencyLearner) A() {
    vll.After()
}

func LLA( name string) { // pair complement to "func B(name string)"
    if !init_completed { return}

    Learners[ name].After() // TODO map found guard
}

func Latency_measure_self_sample( n int) {
    if !init_completed { return} // TODO auto-init, or, return error

    // below is to (try to) measure/estimate the latency cost of a latlearn measurement
    // but lots of subtle issues and complexity lurk here
    if (n < 0) { n = 1_000_000} // we'll do it 1 million times, hoping to mitigate (somewhat, maybe) the effects of host load spikes, GC runs, etc

    ll    := Learners[ "LL.no-op"]
    for i := 0; i < n; i++ {
        ll.Before()
        ll.After()
    }
}

func Measure_overhead_estimate() (overhead time.Duration, exists bool) {
    if !init_completed              { return -1, false}

    lli, found := Learners[ "LL.no-op"]

    if  !found                      { return -1, false}

    ll_noop    := lli.GetLL()

    if !ll_noop.Pair_ever_completed { return -1, false}

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
        ll     := B( span)
        if err := cmd.Run(); (err != nil) {
            // TODO call variant of ll.A() method with arg to indicate it failed
        }
        ll.A()
    }
}

func Benchmarks() {
    if !init_completed { return} // TODO auto-init, or, return error

    pre      :=      "latlearn_benchmarks"
    ll_bt    := B( "LL.benchmarks-total")

    Latency_measure_self_sample( -1) // defaults to 1M

    for i     := 0; i < 1000; i++ {
        ll    := B( "LL.fn-call-return")
        noop_fn_for_benchmark_calls()
        ll.A()
    }

    for i     := 0; i < 1000; i++ {
        ll    := B( "LL.for-iters(n=1000)")
        for j := 0; j < 1000; j++ {
        }
        ll.A()
    }

    for i     := 0; i < 1000; i++ {
        ll    := B( "LL.accum-ints(n=1000)")
        v     := 0
        for j := 0; j < 1000; j++ {
            v += j
        }
        ll.A()
    }

    for i    := 0; i < 1000; i++ {
        ll   := B( "LL.add-int-literals(n=2)")
        a    := (1 + 2)
        _     = a // make closer to real, and compiler happy
        ll.A()
    }

    for i    := 0; i < 1000; i++ {
        ll   := B( "LL.add-str-literals(n=2)")
        c    := ("a" + "b")
        _     = c // make closer to real, and compiler happy
        ll.A()
    }

    for i     := 0; i < 1000; i++ {
        m     := make( map[string]int)
        ll    := B( "LL.map-str-int-set")
        m[ "foo"] = 5
        ll.A()
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
        ll    := B( "LL.map-str-int-get(k=100,key0)")
        _ = m[ "key0"]
        ll.A()
        ll     = B( "LL.map-str-int-get(k=100,key49)")
        _ = m[ "key49"]
        ll.A()
        ll     = B( "LL.map-str-int-get(k=100,key99)")
        _ = m[ "key99"]
        ll.A()
    }

    for i    := 0; i < 1000; i++ {
        ll   := B( "LL.span-map-lookup")
        a, b := Learners[ "LL.no-op"]
        ll.A()
        _     = a // yes, is reason why we are doing this
        _     = b // ditto
    }

    strs         := []string { "Zelda", "Hoth", "Abro",  "Daneel", "Tempest", "Cthulhu", "Bonk", "Arky","Ys", "Jude Law"}
    for i        := 0; i < 1000; i++ {
        strs2    := []string {}
        for _, s := range strs {
            strs2 = append( strs2, s)
        }
        ll   := B( "LL.sort-strs(n=10)")
        sort.Strings( strs2) // sorts the given slice in-place
        ll.A()
    }

    ll       := B( "LL.log-hellos(n=10)")
    for i    := 0; i < 10; i++ {
        log.Printf( "%s: log measure test\n", pre)
    }
    ll.A()

    for i     := 0; i < 1000; i++ {
        ll    := B( "LL.byte-array-make(n=1)")
        array := make( []byte,      1)
        _      = array // to quiet the compiler
        ll.A()

        ll     = B( "LL.byte-array-make(n=1k)")
        array  = make( []byte,   1000)
        _      = array // to quiet the compiler
        ll.A()

        ll     = B( "LL.byte-array-make(n=100k)")
        array  = make( []byte, 100000)
        _      = array // to quiet the compiler
        ll.A()
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

    ll_bt.A()
}

func (ll *LatencyLearner) values() ( string, time.Duration, time.Duration, int, time.Duration, time.Duration, bool, bool) {
    return ll.Name, ll.Last, ll.Cumul, ll.Weight, ll.Min, ll.Max, ll.pair_underway, ll.Pair_ever_completed
}

func (ll *LatencyLearner) Mean() ( mean_latency int64, weight int) {
    mean_latency      = int64( -1)
    weight            = ll.Weight
    if weight         > 0 {
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
func overhead_comp( metric_in int64, overhead int64) (metric_out int64) { // "comp" for compensate
    if Should_subtract_overhead {
        metric_out = (metric_in - overhead)
    } else {
        metric_out = metric_in
    }
    return metric_out
}

// for latlearn's internal use only
func (ll *LatencyLearner) report( f *os.File, name_field string, since_init time.Duration, overhead time.Duration) { // time.Duration is int64 ns
    line := ""

    if ll.Pair_ever_completed {
        min              := int64( ll.Min)
        if (overhead != -1) && (ll.Name != "LL.no-op") {
            min           = overhead_comp( int64( ll.Min),          int64( overhead))
        }
        min_txt          := fmt.Sprintf( "%15s", number_grouped( int64( min), ","))

        last             := overhead_comp( int64(ll.Last), int64(overhead))
        last_txt         := fmt.Sprintf( "%15s", number_grouped( int64( last), ","))

        max              := overhead_comp( int64(ll.Max),          int64(overhead))
        max_txt          := fmt.Sprintf( "%15s", number_grouped( int64( max), ","))

        mean_txt         := "???,???,???,???"
        weight_txt       :=     "???,???,???"
        tf_txt           :=        "????????"
        weight           := ll.Weight

        if weight         > 0 {
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
        //log.Printf( "latearn report's mac sysctl exec failed. output: %s\n", string( out))
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

func Report2( params []string) {
    if !init_completed { return}
    pre := "latlearn_report2"

    ll      := B( "LL.lat-report")

    f, err  := os.Create( Report_fpath)
    if err  != nil {
        msg := fmt.Sprintf( "latlearn/%s could not create file for report: path '%s', err %#v", pre, Report_fpath, err)
        log.Printf( "%s\n", msg)
        panic( msg)
    }
    defer f.Close()

    io.WriteString( f, "Latency Report (https://github.com/mkramlich/latlearn)\n\n")
    io.WriteString( f, fmt.Sprintf("Should_subtract_overhead: %v\n", Should_subtract_overhead))
    if Should_subtract_overhead {
        io.WriteString( f, "metric treated as overhead: LL.no-op, min\n")
    }

    t2         := time.Now()         // time.Time
    since_init := t2.Sub( init_time) // time.Duration. int64. ns. legit/precise?
    si_txt     := number_grouped( int64( since_init), ",")
    time_param := fmt.Sprintf( "since LL init: %s ns\n\n", si_txt)
    io.WriteString( f, time_param)

    io.WriteString( f, fmt.Sprintf( "Go ver:         %s\n", runtime.Version()))
    io.WriteString( f, fmt.Sprintf( "GOARCH:         %s\n", runtime.GOARCH))
    io.WriteString( f, fmt.Sprintf( "GOOS:           %s\n", runtime.GOOS))
    io.WriteString( f, fmt.Sprintf( "NumCPU:         %d\n", runtime.NumCPU()))
    io.WriteString( f, fmt.Sprintf( "GOMAXPROCS:     %d\n", runtime.GOMAXPROCS( -1)))
    io.WriteString( f, fmt.Sprintf( "NumGoroutine:   %d\n", runtime.NumGoroutine()))
    //io.WriteString( f, fmt.Sprintf( "NumCgoCall:   %d\n", runtime.NumCgoCall()))

    mem_limit     := debug.SetMemoryLimit( -1)
    mem_limit_str := number_grouped( mem_limit, ",")
    io.WriteString( f, fmt.Sprintf( "SetMemoryLimit: %s bytes\n", mem_limit_str))

    gogc := ""
    if val, ok := os.LookupEnv(     "GOGC"); ok {           gogc = val}
    io.WriteString( f, fmt.Sprintf( "GOGC:           %s\n", gogc))

    if (runtime.GOOS == "darwin") {
        write_info_about_mac_host_to_report( f)
    }

    term_rows  := "?"
    term_cols  := "?"
    if val, ok := os.LookupEnv(     "LINES");   ok {        term_rows = val}
    if val, ok := os.LookupEnv(     "COLUMNS"); ok {        term_cols = val}
    io.WriteString( f, fmt.Sprintf( "LINES:          %s\n", term_rows))
    io.WriteString( f, fmt.Sprintf( "COLUMNS:        %s\n", term_cols))

    host       := ""
    if val, ok := os.LookupEnv(     "HOST"); ok {           host = val}
    io.WriteString( f, fmt.Sprintf( "HOST:           %s\n", host))

    term       := ""
    if val, ok := os.LookupEnv(     "TERM"); ok {           term = val}
    io.WriteString( f, fmt.Sprintf( "TERM:           %s\n", term))

    io.WriteString( f, "\n")

    // Context Params (which may impact interpretation of the reported span metrics)
    for i, param     := range params {
        txt          := ""
        if        i  == 0 {
            txt       = param
        } else if ((i != 0) && ((i % 4) == 0)) { // TODO do this better
              txt     = "\n" + param
        } else if i > 0 {
              txt     = ", " + param
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
        overhead, _ = Measure_overhead_estimate()
    }

    for _, span := range tracked_spans {
        if !Should_report_builtins && strings.HasPrefix( span,"LL.") { continue}
        Learners[ span].report( f, name_field, since_init, overhead) // TODO add found-in-map guard
    }

    ll.A()
}

func Report() {
     Report2( []string {})
}

