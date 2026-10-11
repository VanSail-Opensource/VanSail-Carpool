package iplookup

import "strconv"

func appendEnhancedProgress(p *Progress, r Report) {
	if r.Checkpoint == nil || r.Checkpoint.Enhanced == nil {
		return
	}
	e := r.Checkpoint.Enhanced
	p.Phase = e.Phase
	for _, key := range []string{"proxy", "live", "probes", "subnet"} {
		status := "pending"
		switch key {
		case "proxy":
			if r.Proxy != nil {
				status = r.Proxy.Status
			}
		case "live":
			if e.LiveDone {
				status = "success"
			}
		case "probes":
			if len(r.Ports) > 0 {
				status = "success"
				for _, port := range r.Ports {
					if port.Status == "pending" || port.Status == "processing" {
						status = "pending"
						break
					}
					if port.Status == "error" {
						status = "partial"
					}
				}
			}
		case "subnet":
			if r.Subnet != nil {
				status = r.Subnet.Status
			}
		}
		if key == e.Phase && (status == "pending" || status == "processing") {
			status = "active"
		}
		if status == "processing" {
			status = "active"
		}
		if status == "unavailable" {
			status = "error"
		}
		p.Total++
		if status != "pending" && status != "active" {
			p.Completed++
		}
		p.Tasks = append(p.Tasks, ProgressTask{Key: key, Status: status})
	}
	for _, port := range r.Ports {
		for _, task := range port.Tasks {
			if task.Status == "active" {
				p.CurrentProvider = strconv.Itoa(port.Port) + ":" + task.Key
				return
			}
		}
	}
}
