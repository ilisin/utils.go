package runner

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	RunnerStateWait     = iota // 等待状态
	RunnerStateRunning         // 运行状态
	RunnerStateStopping        // 停止中

	defaultInterval   = 100 * time.Millisecond
	precisionInterval = 1 * time.Millisecond
)

type job struct {
	expectAt time.Time
	call     func() (afterContinue bool, nextInterval time.Duration)
	caller   string
}

type chanJob struct {
	reflectCh   reflect.SelectCase
	expireAt    time.Time
	recv        func(recvData interface{}) (stop bool)
	timeoutCall func()
	stopped     bool
	over        chan struct{}
}

type Runner struct {
	state int32

	id     string
	jobs   []job
	jobsMu sync.Mutex

	chanJobs   []*chanJob
	chanJobsMu sync.Mutex

	notifyCh    chan struct{}
	notifyChSel reflect.SelectCase
	stop        chan struct{}
	stopSel     reflect.SelectCase
}

func NewRunner(id string) *Runner {
	notifyCh := make(chan struct{}, 16)
	return &Runner{
		id:       id,
		state:    RunnerStateWait,
		notifyCh: notifyCh,
		notifyChSel: reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(notifyCh),
		},
	}
}

func (r *Runner) executeJobs() (int64, time.Time) {
	count := int64(0)

	exeJobs := []job(nil)
	r.jobsMu.Lock()
	now := time.Now()
	for i := 0; i <= len(r.jobs); i++ {
		// over
		if i == len(r.jobs) {
			r.jobs = nil
			break
		}
		j := r.jobs[i]
		if j.expectAt.Sub(now) < precisionInterval {
			exeJobs = append(exeJobs, j)
		} else {
			r.jobs = r.jobs[i+1:]
			break
		}
	}
	r.jobsMu.Unlock()

	repeatJobs := []job(nil)
	for _, j := range exeJobs {
		var nextContinue bool
		var nextDuration time.Duration
		var wait = make(chan struct{})
		var call = j.call
		var callAt = time.Now()
		go func(call func() (bool, time.Duration), wait chan struct{}) {
			nextContinue, nextDuration = call()
			wait <- struct{}{}
		}(call, wait)
		var timeout = time.After(500 * time.Millisecond)
		select {
		case <-wait:
			break
		case <-timeout:
			fmt.Println(j.caller)
			<-wait
			fmt.Printf("[WARN] schedule run %v!!\n", time.Since(callAt))
			break
		}
		if nextContinue && nextDuration > precisionInterval {
			repeatJobs = append(repeatJobs, job{
				expectAt: now.Add(nextDuration),
				call:     j.call,
			})
		}
		count++
	}

	nextAt := time.Time{}
	r.jobsMu.Lock()
	if len(repeatJobs) > 0 {
		r.jobs = append(r.jobs, repeatJobs...)
		sort.Slice(r.jobs, func(i, j int) bool {
			return r.jobs[i].expectAt.Before(r.jobs[j].expectAt)
		})
	}
	if len(r.jobs) > 0 {
		nextAt = r.jobs[0].expectAt
	}
	r.jobsMu.Unlock()
	return count, nextAt
}

func (r *Runner) executeWaitToRun(nextAt time.Time) (stop bool) {
	firstAt := nextAt
	cases := []reflect.SelectCase{
		r.notifyChSel,
		r.stopSel,
	}
	jobChStart := len(cases)
	holdChanJobs := []*chanJob(nil)
	r.chanJobsMu.Lock()
	if len(r.chanJobs) > 0 {
		remainJobs := []*chanJob(nil)
		for _, chJob := range r.chanJobs {
			if chJob.stopped {
				continue
			}
			now := time.Now()
			if !chJob.expireAt.IsZero() && chJob.expireAt.Before(now) {
				if chJob.timeoutCall != nil {
					chJob.timeoutCall()
					if chJob.over != nil {
						close(chJob.over)
					}
				}
				continue
			}
			remainJobs = append(remainJobs, chJob)
			holdChanJobs = append(holdChanJobs, chJob)
			cases = append(cases, chJob.reflectCh)
			if !chJob.expireAt.IsZero() && (firstAt.IsZero() || chJob.expireAt.Before(firstAt)) {
				firstAt = chJob.expireAt
			}
		}
		r.chanJobs = remainJobs
	}
	r.chanJobsMu.Unlock()
	if firstAt.IsZero() {
		firstAt = time.Now().Add(defaultInterval)
	}
	if firstAt.Sub(time.Now()) < precisionInterval {
		return
	}
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(firstAt.Sub(time.Now()))),
	})
	chosen, recvDat, recvOk := reflect.Select(cases)
	switch chosen {
	case 1: // stop chan
		atomic.StoreInt32(&r.state, RunnerStateWait)
		return true
	default:
		if chosen >= jobChStart && chosen-jobChStart < len(holdChanJobs) {
			if chosenJob := holdChanJobs[chosen-jobChStart]; chosenJob.recv != nil {
				var originDat interface{}
				if recvOk {
					originDat = recvDat.Interface()
					// 执行完停止
					if chosenJob.recv(originDat) {
						chosenJob.stopped = true
						chosenJob.expireAt = time.Time{}
						if chosenJob.over != nil {
							close(chosenJob.over)
						}
					}
				}
			}
		}
	}
	return false
}

func (r *Runner) Start() bool {
	if !atomic.CompareAndSwapInt32(&r.state, RunnerStateWait, RunnerStateRunning) {
		return false
	}
	r.stop = make(chan struct{})
	r.stopSel = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(r.stop),
	}
LOOP:
	for {
		_, nextAt := r.executeJobs()
		if stop := r.executeWaitToRun(nextAt); stop {
			break LOOP
		}
	}
	return true
}

func (r *Runner) addJob(j job) {
	r.jobsMu.Lock()
	if len(r.jobs) > 0 && r.jobs[len(r.jobs)-1].expectAt.Before(j.expectAt) {
		r.jobs = append(r.jobs, j)
	} else {
		r.jobs = append(r.jobs, j)
		sort.Slice(r.jobs, func(i, j int) bool {
			return r.jobs[i].expectAt.Before(r.jobs[j].expectAt)
		})
	}
	r.jobsMu.Unlock()
}

func (r *Runner) addInstantJob(call func(), over chan struct{}, caller string) {
	runAt := time.Now()
	r.addJob(job{
		expectAt: runAt,
		call: func() (afterContinue bool, nextInterval time.Duration) {
			call()
			if over != nil {
				close(over)
			}
			return false, 0
		},
		caller: caller,
	})
}

func (r *Runner) addTimerJob(interval time.Duration, repeats int, startNow bool, call func(), over chan struct{}, caller string) {
	counter := 0
	expectCount := repeats
	at := time.Now()
	if !startNow {
		at = at.Add(interval)
	}
	r.addJob(job{
		expectAt: at,
		call: func() (afterContinue bool, nextInterval time.Duration) {
			call()
			counter++
			if expectCount < 0 || counter < expectCount {
				return true, interval
			}
			if over != nil {
				close(over)
			}
			return false, 0
		},
		caller: caller,
	})
}

func (r *Runner) addChanJob(timeout time.Duration, ch interface{}, recv func(interface{}) bool, timeoutCall func(), over chan struct{}) {
	val := reflect.ValueOf(ch)
	if val.Kind() != reflect.Chan {
		panic("param ch must be a channel")
	}
	r.chanJobsMu.Lock()
	r.chanJobs = append(r.chanJobs, &chanJob{
		reflectCh: reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: val,
		},
		expireAt:    time.Now().Add(timeout),
		recv:        recv,
		timeoutCall: timeoutCall,
		over:        over,
		stopped:     false,
	})
	r.chanJobsMu.Unlock()
}

func (r *Runner) tick() {
	r.notifyCh <- struct{}{}
}

func getCaller(skip int) string {
	var caller string
	pc, file, line, ok := runtime.Caller(skip)
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		caller = fmt.Sprintf("%v:%v %v", file, line, funcName)
	}
	return caller
}

func (r *Runner) Run(call func()) {
	caller := getCaller(3)
	go func() {
		r.addInstantJob(call, nil, caller)
		r.tick()
	}()
}

func (r *Runner) QueueRun(calls ...func() bool) {
	if len(calls) == 0 {
		return
	}
	caller := getCaller(3)
	var wait chan struct{}
	var run = func(int) {}
	run = func(index int) {
		call := calls[index]
		afterStop := false
		if index < len(calls)-1 {
			wait = make(chan struct{})
			go func(wait chan struct{}, index int) {
				<-wait
				if !afterStop {
					run(index)
				}
			}(wait, index+1)
		} else {
			wait = nil
		}
		r.addInstantJob(func() {
			afterStop = call()
		}, wait, caller)
		r.tick()
	}
	go run(0)
}

func (r *Runner) WaitRun(call func()) {
	caller := getCaller(3)
	wait := make(chan struct{})
	go func(wait chan struct{}) {
		r.addInstantJob(call, wait, caller)
		r.tick()
	}(wait)
	<-wait
}

func (r *Runner) AfterRun(interval time.Duration, call func()) {
	caller := getCaller(3)
	if interval <= 0 {
		r.Run(call)
	} else {
		go r.addTimerJob(interval, 1, false, call, nil, caller)
	}
}

func (r *Runner) WaitAfterRun(interval time.Duration, call func()) {
	caller := getCaller(3)
	wait := make(chan struct{})
	if interval <= 0 {
		go func(wait chan struct{}) {
			r.addInstantJob(call, wait, caller)
			r.tick()
		}(wait)
	} else {
		go r.addTimerJob(interval, 1, false, call, wait, caller)
	}
	<-wait
}

func (r *Runner) TimerRun(interval time.Duration, repeat int, call func()) {
	caller := getCaller(3)
	if interval <= 0 {
		go func() {
			r.addInstantJob(call, nil, caller)
			r.tick()
		}()
	} else {
		go r.addTimerJob(interval, repeat, false, call, nil, caller)
	}
}

func (r *Runner) ChanRun(timeout time.Duration, ch interface{}, recv func(interface{}) bool, timeoutCall func()) {
	go func() {
		r.addChanJob(timeout, ch, recv, timeoutCall, nil)
		r.tick()
	}()
}

func (r *Runner) ChanRunFinal(timeout time.Duration, ch interface{}, recv func(interface{}) bool, timeoutCall, finalCall func()) {
	go func() {
		wait := make(chan struct{})
		go func() {
			<-wait
			finalCall()
		}()
		r.addChanJob(timeout, ch, recv, timeoutCall, wait)
		r.tick()
	}()
}

func (r *Runner) WaitChanRun(timeout time.Duration, ch interface{}, recv func(interface{}) bool, timeoutCall func()) {
	if ch == nil {
		return
	}
	wait := make(chan struct{})
	go func() {
		r.addChanJob(timeout, ch, recv, timeoutCall, wait)
		r.tick()
	}()
	<-wait
}

func (r *Runner) Stop() bool {
	if atomic.CompareAndSwapInt32(&r.state, RunnerStateRunning, RunnerStateStopping) {
		close(r.stop)
		return true
	}
	return false
}
