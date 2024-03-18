package latlearn_test

import (
    "fmt"
    "os"
    "testing"
    "time"

    "."
)

func samples_B( t *testing.T, span string, vals []int64) {
    for _, val   := range vals {
        dur_str  := fmt.Sprintf( "%dns", val) // eg. "10ns"
        dur, err := time.ParseDuration( dur_str)
        if   err != nil {
            t.Fatalf( "error parsing dur")
        }

        ssu      := latlearn.B( span)
        ssu.T2    = ssu.T1.Add( dur) // dur is int64 of ns
        ssu.A() // since we set T2 (for test purposes) then ssu.after() will NOT clobber T2 (it wont replace our T2 with the result of Now())
    }
}

func samples_B2( t *testing.T, span string, variant string, vals []int64) {
    for _, val   := range vals {
        dur_str  := fmt.Sprintf( "%dns", val) // eg. "10ns"
        dur, err := time.ParseDuration( dur_str)
        if   err != nil {
            t.Fatalf( "error parsing dur")
        }

        ssu      := latlearn.B2( span, variant)
        ssu.T2    = ssu.T1.Add( dur) // dur is int64 of ns
        ssu.A() // since we set T2 (for test purposes) then ssu.after() will NOT clobber T2 (it wont replace our T2 with the result of Now())
    }
}

func assert_values( t *testing.T, span string, ok bool, pair_ever_completed bool, min int64, max int64, last int64, cumul int64, weight int64, mean int64) {

    rm, ok_got := latlearn.Values( span)

    if ok_got != ok {
        t.Fatalf( "latlearn.Values() ok: want %v, got %v", ok, ok_got)
    }

    if rm.Pair_ever_completed != pair_ever_completed {
        t.Fatalf( "rm.Pair_ever_completed: want %v, got %v", pair_ever_completed, rm.Pair_ever_completed)
    }

    if (int64(rm.Min) != min)         {
        t.Fatalf( "rm.Min: want %d, got %d", min, rm.Min)
    }

    if (int64(rm.Max) != max)         {
        t.Fatalf( "rm.Max: want %d, got %d", max, rm.Max)
    }

    if (int64(rm.Last) != last)         {
        t.Fatalf( "rm.Last: want %d, got %d", last, rm.Last)
    }

    if (int64(rm.Cumul) != cumul)         {
        t.Fatalf( "rm.Cumul: want %d, got %d", cumul, rm.Cumul)
    }

    if (int64(rm.Weight) != weight)         {
        t.Fatalf( "rm.Weight: want %d, got %d", weight, rm.Weight)
    }

    mean_got := int64(rm.Cumul) / int64(rm.Weight)
    if (mean_got != mean)         {
        t.Fatalf( "mean: want %d, got %d", mean, mean_got)
    }
}

func assert_samples_B( t *testing.T, span string, vals []int64, min int64, max int64, last int64, cumul int64, weight int64, mean int64) {

    samples_B( t, span, vals)

    assert_values( t, span, true, true, min, max, last, cumul, weight, mean)
}

func assert_samples_B2( t *testing.T, span string, variant string, vals []int64, min int64, max int64, last int64, cumul int64, weight int64, mean int64) {

    samples_B2( t, span, variant, vals)

    assert_values( t, span, true, true, min, max, last, cumul, weight, mean)
    // TODO assert on Values for the variant entry AND family parent entry
}

func TestBasic( t *testing.T) {

    latlearn.Init()

    for i := 0; i < 10; i++ {
        // These calls should be ignored (become no-op's)
        // due to a sync.Oncer guard inside Init().
        // How to decide correctness? If nothing subsequent to here detects a glitch.
        latlearn.Init()
    }

    //latlearn.init_time >= old_now
    //latlearn.init_completed

    /*if !latlearn.Serve_started {
        t.Fatalf("Serve_started: got false, want true")
    }
    if latlearn.Serve_finished {
        t.Fatalf("Serve_finished: got true, want false")
    }*/

    span := "span1"

    ll := latlearn.B( span)
    ll.A()

    rm, ok := latlearn.Values( span)

    if !ok {
        t.Errorf( "latlearn.Values() ok: want true, got false")
    }

    if !rm.Pair_ever_completed {
        t.Errorf( "rm.Pair_ever_completed: want true, got false")
    }

    if (rm.Weight != 1)         {
        t.Errorf( "rm.Weight: want 1, got %d", rm.Weight)
    }

    ////////////////////

    // Construct synthetic SSU instances (with explicit start & end timestamps), submit them, then assert afterward that the span's metrics are what I expect
    span = "span2"
    ssu := latlearn.B( span)
    dur, err := time.ParseDuration( "10ns")
    if   err != nil { // dur is int64 of ns
        t.Errorf( "error parsing dur")
    }
    ssu.T2 = ssu.T1.Add( dur)
    ssu.A() // since we set T2 (for test purposes) then ssu.after() will NOT clobber T2 (it wont replace our T2 with the result of Now())

    rm2, ok2 := latlearn.Values( span)
    if !ok2 {
        t.Errorf( "latlearn.Values() ok: want true, got false")
    }
    if !rm2.Pair_ever_completed {
        t.Errorf( "rm2.Pair_ever_completed: want true, got false")
    }
    if (rm2.Weight != 1)         {
        t.Errorf( "rm2.Weight: want 1, got %d", rm2.Weight)
    }
    if (rm2.Min    != dur) {
        t.Errorf( "rm2.Min: want %d, got %d",   dur, rm2.Min)
    }
    if (rm2.Last   != dur) {
        t.Errorf( "rm2.Last: want %d, got %d",  dur, rm2.Last)
    }
    if (rm2.Max    != dur) {
        t.Errorf( "rm2.Max: want %d, got %d",   dur, rm2.Max)
    }
    if (rm2.Mean   != int64(dur)) {
        t.Errorf( "rm2.Mean: want %d, got %d",  dur, rm2.Mean)
    }
    if (rm2.Cumul  != dur) {
        t.Errorf( "rm2.Cumul: want %d, got %d", dur, rm2.Cumul)
    }

    /////////////////

    assert_samples_B(  t, "span3",             []int64 {10, 20}, 10, 20, 20, 30, 2, 15)
    assert_samples_B2( t, "span4", "variant1", []int64 {10, 20}, 10, 20, 20, 30, 2, 15)

    // TODO
    // variants & families
    //     A2 variants. (end early cuz early return condition, or panic)
    // never ended spans
    // double ended spans
    // asynch ended spans
    // persisted/resumed ended spans
    // concurrent ended spans
    // confirm that after benchmark finishes there is a no-op span entry
    // sleep for X ms. then later assert that the min sample >= X ms
    // add more permutations of samples?

    /*t.Run("A=1", func(t *testing.T) {
    })
    t.Run("A=2", func(t *testing.T) {
    })*/

    latlearn.Report()

    if f, err := os.Open( "./latlearn-report.txt"); (err != nil) { // assumes exist, opens for read
        t.Errorf( "failed to open report file. error: %v", err)
    } else {
        f.Close()
    }
    // reaching here without crash, panic or hang is a good sign

    if ok := latlearn.Stop(); !ok {
        t.Errorf( "Stop() failed: want true, got false")
    }
    // TODO multiple stop requests
}

//func BenchmarkFoo(b *testing.B) {} // TODO benchmark the overhead cost (no-op)
//func FuzzFoo(f *testing.F) {} // TODO fuzz span names, variant names, metric values
