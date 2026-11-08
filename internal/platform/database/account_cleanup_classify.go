package database

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

func classifyCleanupObservation(o model.AccountCleanupObservation, prior bool) (reachable, qualifying, valid bool) {
	actions := map[string]bool{"reachable": true, "blocked": true, "deactivated": true, "unreachable": true, "unknown": true}
	chats := map[string]bool{"private": true, "deactivated": true, "unreachable": true, "unknown": true}
	if !actions[o.SendAction] || !chats[o.GetChat] {
		return false, false, false
	}
	if o.SendAction == "reachable" {
		return true, false, true
	}
	if o.SendAction == "unknown" || o.GetChat == "unknown" {
		return false, false, true
	}
	if o.SendAction == "blocked" {
		return false, true, true
	}
	if o.SendAction == "deactivated" {
		return false, o.GetChat != "private", true
	}
	return false, prior && o.GetChat == "unreachable", true
}
