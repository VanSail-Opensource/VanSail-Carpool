package iplookup

import "sync"

type workerRun struct {
	mu    sync.Mutex
	users int
}

func (w *Worker) lockRun(id string) func() {
	w.mu.Lock()
	if w.runs == nil {
		w.runs = map[string]*workerRun{}
	}
	run := w.runs[id]
	if run == nil {
		run = &workerRun{}
		w.runs[id] = run
	}
	run.users++
	w.mu.Unlock()
	run.mu.Lock()
	return func() {
		run.mu.Unlock()
		w.mu.Lock()
		defer w.mu.Unlock()
		run.users--
		if run.users == 0 {
			delete(w.runs, id)
		}
	}
}
