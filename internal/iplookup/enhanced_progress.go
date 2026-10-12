package iplookup

import "strconv"

func appendEnhancedProgress(p *Progress, r Report) {
	if r.Checkpoint == nil || r.Checkpoint.Enhanced == nil {
		return
	}
	e := r.Checkpoint.Enhanced
	if e.Phase != "checking" {
		p.Phase = e.Phase
	}
	inventoryTasks := []ProgressTask{}
	for _, key := range []string{"proxy", "subnet", "live", "probes"} {
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
				for _, row := range r.Subnet.Addresses {
					for _, port := range row.Ports {
						for _, task := range port.Tasks {
							p.Total++
							if subnetProbeFinished(task.Status) {
								p.Completed++
							} else {
								status = "pending"
							}
							if task.Status == "active" {
								p.CurrentProvider = strconv.Itoa(port.Port) + ":" + task.Key
							}
						}
					}
				}
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
		task := ProgressTask{Key: key, Status: status}
		if key == "proxy" || key == "subnet" {
			inventoryTasks = append(inventoryTasks, task)
		} else {
			p.Tasks = append(p.Tasks, task)
		}
	}
	p.Tasks = append(inventoryTasks, p.Tasks...)
	for _, port := range r.Ports {
		for _, task := range port.Tasks {
			if task.Status == "active" {
				p.CurrentProvider = strconv.Itoa(port.Port) + ":" + task.Key
				return
			}
		}
	}
}
