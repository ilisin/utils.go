package runner

import (
    "fmt"
    "math/rand"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

func TestMQ(t *testing.T) {
    runner := NewRunner("id")
    wg := sync.WaitGroup{}

    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    threads := r.Intn(30) + 10
    // threads := 2

    count := int32(0)

    maxDur := 5
    // maxDur := 10
    minDur := 1

    wg.Add(1)
    go func() {
        defer wg.Done()
        runner.Start()
    }()

    runner.TimerRun(5*time.Second, -1, func() {
        fmt.Printf("timer\n")
        if c := atomic.LoadInt32(&count); c >= int32(threads) {
            runner.Stop()
        }
    })

    max := time.Duration(0)
    fmt.Printf("threads: %v\n", threads)
    for i := 0; i < threads; i++ {
        go func(i int) {
            seconds := time.Duration(r.Intn(maxDur-minDur+1)+minDur) * time.Second
            // if i == 0 {
            //	seconds = 2 * time.Second
            // }
            // fmt.Printf("[info]%v seconds:%v\n", i, seconds)
            now := time.Now()
            runner.WaitAfterRun(seconds, func() {
                newNow := time.Now()
                expectNow := now.Add(seconds)
                bios := newNow.Sub(expectNow)
                if bios > max {
                    max = bios
                }
                fmt.Printf("[timer]%v %v expect run at:%v, result run at:%v\n", i, bios, expectNow, newNow)
            })
            newNow := time.Now()
            runner.Run(func() {
                resultNow := time.Now()
                bios := resultNow.Sub(newNow)
                if bios > max {
                    max = bios
                }
                fmt.Printf("[instant]%v %v expect run at:%v, result run at:%v\n", i, bios, newNow, resultNow)
                atomic.AddInt32(&count, 1)
            })
        }(i)
    }

    ch := make(chan string, 8)
    runner.ChanRun(10*time.Second, ch, func(i interface{}) bool {
        fmt.Printf("--- %v chan receive: %v\n", time.Now(), i)
        return false
    }, func() {
        fmt.Printf("--- %v chan timeout\n", time.Now())
    })
    go func() {
        time.Sleep(5 * time.Second)
        ch <- "5 seconds later"
        time.Sleep(1 * time.Second)
        ch <- "more than 1 seconds later"
    }()

    runner.QueueRun(func() (stop bool) {
        fmt.Printf("1 at %v\n", time.Now())
        time.Sleep(time.Second)
        return false
    }, func() (stop bool) {
        fmt.Printf("2 at %v\n", time.Now())
        time.Sleep(time.Second * 2)
        return true
    }, func() (stop bool) {
        fmt.Printf("3 at %v\n", time.Now())
        time.Sleep(time.Second * 3)
        return false
    }, func() (stop bool) {
        fmt.Printf("4 at %v\n", time.Now())
        time.Sleep(time.Second * 4)
        return true
    })

    wg.Wait()
    time.Sleep(15 * time.Second)
    fmt.Printf("end at :%v\n", len(runner.jobs))
    for i, j := range runner.jobs {
        fmt.Printf("%v %v\n", i, j.expectAt)
    }

    t.Logf("max %v", max)
}

func TestTimeAfter(t *testing.T) {
    rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
    for i := 0; i < 20; i++ {
        interval := time.Duration(rnd.Intn(10000)+500) * time.Millisecond
        expect := time.Now().Add(interval)
        fmt.Printf("%v: expect %v\n", i, expect)
        time.Sleep(interval)
        // select {
        // case <-time.After(interval):
        //	break
        // }
        now := time.Now()
        fmt.Printf("%v: result %v interval:%v\n", i, now, now.Sub(expect))
    }
}

func TestRunner(t *testing.T) {
    c := 3
    expectCount := 0
    r := rand.New(rand.NewSource(time.Now().UnixNano()))

    runner := NewRunner("id")

    ch := make(chan struct{})
    go func() {
        runner.Start()
        ch <- struct{}{}
    }()

    var counter int64

    for i := 0; i < c; i++ {
        secs := time.Duration(r.Intn(2000)+100) * time.Millisecond
        now := time.Now()
        expectAt := now.Add(secs)
        fmt.Printf("[%d] add timer at %v, expect run at %v\n", i, now, expectAt)
        runner.TimerRun(secs, 1, func() {
            fmt.Printf("[%d]timer %v %v\n", i, secs, time.Now().Sub(expectAt))
            atomic.AddInt64(&counter, 1)
        })
        expectCount++
        secs = time.Duration(r.Intn(2000)+50) * time.Millisecond
        time.Sleep(secs)
        now = time.Now()
        fmt.Printf("[%d] add instant timer at %v\n", i, now)
        runner.Run(func() {
            fmt.Printf("[%d]run %v %v\n", i, secs, time.Now().Sub(now))
            atomic.AddInt64(&counter, 1)
        })
        expectCount++
    }

    var timerID int64
    timerID = runner.TimerRunWithID(5*time.Second, 3, func() {
        fmt.Printf("[REPEATE]timer run\n")
        atomic.AddInt64(&counter, 1)
        if timerID > 0 {
            fmt.Printf("[REPEATE]cancel timer\n")
            runner.CancelTimer(timerID)
            timerID = 0
            expectCount -= 2
        }
    })
    expectCount += 3

    for {
        if v := atomic.LoadInt64(&counter); v == int64(expectCount) {
            break
        }
    }
    runner.Stop()
    <-ch

    fmt.Printf("END:%v\n", len(runner.jobs))
    time.Sleep(time.Second)
    fmt.Printf("END:%v\n", len(runner.jobs))
    time.Sleep(5 * time.Second)
    fmt.Printf("END:%v\n", len(runner.jobs))
}
