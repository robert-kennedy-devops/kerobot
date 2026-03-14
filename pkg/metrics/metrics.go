package metrics

import "sync/atomic"

type Counters struct {
	ActionsOK    uint64
	ActionsError uint64
	Messages     uint64
}

func (c *Counters) IncActionsOK()    { atomic.AddUint64(&c.ActionsOK, 1) }
func (c *Counters) IncActionsError() { atomic.AddUint64(&c.ActionsError, 1) }
func (c *Counters) IncMessages()     { atomic.AddUint64(&c.Messages, 1) }

func (c *Counters) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"actions_ok":    atomic.LoadUint64(&c.ActionsOK),
		"actions_error": atomic.LoadUint64(&c.ActionsError),
		"messages":      atomic.LoadUint64(&c.Messages),
	}
}
