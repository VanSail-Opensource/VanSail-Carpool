package xnode

import wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"

// Called under peer.mu by the receipt consumer. Redundant acceptance
// frames cannot occupy the terminal result's slot; an existing result wins.
// Preserve acceptance before an error so uncertain execution cannot become a
// definite rejection merely because the waiter has not consumed acceptance yet.
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
	var accepted *wire.Message
	for {
		select {
		case old := <-pending:
			if old.Type == "command.result" {
				pending <- old
				return
			}
			if old.Type == "command.accepted" {
				accepted = &old
			}
		default:
			if message.Type == "error" && accepted != nil {
				pending <- *accepted
				// Production command queues have four slots. With a smaller
				// fixture queue, retain acceptance and let its bounded wait
				// report uncertainty rather than inventing a safe retry.
				select {
				case pending <- message:
				default:
				}
				return
			}
			pending <- message
			return
		}
	}
}
