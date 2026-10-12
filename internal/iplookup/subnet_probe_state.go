package iplookup

var subnetProbePorts = [...]int{80, 443, 8443}
var subnetProbeKeys = [...]string{"tcp", "reality", "cloudflare"}

func prepareSubnetProbes(r *Report) {
	for i := range r.Subnet.Addresses {
		row := &r.Subnet.Addresses[i]
		if row.IP == r.IP && len(r.Refusals) > 0 {
			row.Status = "flagged"
		}
		if row.Status == "flagged" || len(row.Ports) > 0 {
			continue
		}
		for _, number := range subnetProbePorts {
			port := PortProbe{Port: number, Status: "pending", Tasks: []ProbeTask{}, Findings: []ProbeFinding{}}
			for _, key := range subnetProbeKeys {
				port.Tasks = append(port.Tasks, ProbeTask{Key: key, Status: "pending"})
			}
			row.Ports = append(row.Ports, port)
		}
	}
}

func applySubnetProbeFinding(row *SubnetAddress) {
	suspected := false
	for _, port := range row.Ports {
		for _, finding := range port.Findings {
			if finding.Confidence == "confirmed" && finding.Protocol == "cloudflare" {
				row.Status = "flagged"
				return
			}
			suspected = suspected || (finding.Protocol == "reality" && finding.Confidence == "suspected")
		}
	}
	if row.Status != "flagged" && suspected {
		row.Status = "suspected"
	}
}

func subnetProbeFinished(status string) bool {
	return status == "success" || status == "closed" || status == "error" || status == "skipped"
}

func skipSubnetPortTasks(port *PortProbe) {
	for i := range port.Tasks {
		if !subnetProbeFinished(port.Tasks[i].Status) {
			port.Tasks[i].Status = "skipped"
		}
	}
}

func finishSubnetProbes(result *SubnetResult) {
	result.Status = "success"
	for _, row := range result.Addresses {
		if row.Status == "unknown" || row.Proxy == "partial" {
			result.Status = "partial"
		}
		for _, port := range row.Ports {
			if port.Status == "error" {
				result.Status = "partial"
			}
		}
	}
}
