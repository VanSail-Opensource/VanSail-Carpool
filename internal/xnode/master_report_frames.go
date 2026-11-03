package xnode

import (
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func readAgentFrame(socket *websocket.Conn, decoders ...*wire.ReportDecoder) (wire.Message, error) {
	kind, reader, err := socket.NextReader()
	if err != nil {
		return wire.Message{}, err
	}
	var decoder *wire.ReportDecoder
	if len(decoders) > 0 {
		decoder = decoders[0]
	}
	if kind != websocket.TextMessage && (kind != websocket.BinaryMessage || decoder == nil) {
		return wire.Message{}, ErrInvalid
	}
	var data []byte
	if kind == websocket.BinaryMessage {
		data, err = decoder.Read(reader)
	} else {
		data, err = wire.ReadMessagePayload(reader)
	}
	if err != nil {
		return wire.Message{}, err
	}
	message, err := wire.Parse(data)
	if err != nil {
		return wire.Message{}, err
	}
	if kind == websocket.BinaryMessage && !wire.IsReport(message.Type) || kind == websocket.TextMessage && decoder != nil && wire.IsReport(message.Type) {
		return wire.Message{}, ErrInvalid
	}
	return message, nil
}

func decodeTelemetryFrame(peer *agentPeer, message wire.Message) (wire.TelemetryReport, error) {
	var report wire.TelemetryReport
	if message.Type == "telemetry.report" {
		if wire.Decode(message.Data, &report) != nil || wire.ValidateActivity(report.Users) != nil {
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
