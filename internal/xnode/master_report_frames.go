package xnode

import (
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func readAgentFrame(socket *websocket.Conn) (wire.Message, error) {
	kind, reader, err := socket.NextReader()
	if err != nil {
		return wire.Message{}, err
	}
	if kind != websocket.TextMessage {
		return wire.Message{}, ErrInvalid
	}
	data, err := wire.ReadMessagePayload(reader)
	if err != nil {
		return wire.Message{}, err
	}
	return wire.Parse(data)
}

func decodeTelemetryFrame(peer *agentPeer, message wire.Message) (wire.TelemetryReport, error) {
	var report wire.TelemetryReport
	if message.Type == "telemetry.report" {
		if wire.Decode(message.Data, &report) != nil {
			return report, ErrInvalid
		}
		return report, nil
	}
	if !peer.compactTelemetry {
		return report, ErrInvalid
	}
	var compact wire.CompactTelemetryReport
	if wire.Decode(message.Data, &compact) != nil {
		return report, ErrInvalid
	}
	report, err := wire.ExpandTelemetry(compact)
	if err != nil {
		return report, ErrInvalid
	}
	return report, nil
}
