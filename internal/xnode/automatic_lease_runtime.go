package xnode

import "time"

func (m *Master) runAutomaticLeases() {
	defer m.wg.Done()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
		case <-m.automaticWake:
		}
		m.automaticLeases.expire(m.service.Now())
	}
}
