package ci

// worker describes a generic worker holding a pointer
// to the Session it belongs to and a wait() method
// that pauses execution until the worker quits.
type worker struct {
	session *Session
	// signal to unblock wait()
	quitChan chan byte
}

func (worker *worker) wait() byte {
	return <-worker.quitChan
}
