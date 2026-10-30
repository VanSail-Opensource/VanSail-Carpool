package xnode

import wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"

// Called under peer.mu by the single control consumer. Redundant acceptance
// frames cannot occupy the terminal result's slot; an existing result wins.
func offerCommandFrame(pending chan wire.Message, message wire.Message) {
	if pending == nil {
		return
	}
	if message.Type == "command.accepted" {
		if len(pending) > 0 {
			return
		}
		select {
		case pending <- message:
		default:
		}
		return
	}
	for {
		select {
		case old := <-pending:
			if old.Type == "command.result" {
				pending <- old
				return
			}
		default:
			pending <- message
			return
		}
	}
}
