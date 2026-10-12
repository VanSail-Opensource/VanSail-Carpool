package iplookup

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"syscall"
	"time"
)

const trojanRepeatedHeader = "User-Agent: Mozilla/5.0\r\nAccept-Charset: utf-8\r\nAccept-Encoding: gzip\r\nConnection: Keep-Alive\r\n"
const trojanHeaderRepeats = 12000

// trojanMilestones retains the research's full incomplete-request observation.
var trojanMilestones = []int{150, 595, 605}

func trojanProbe(ctx context.Context, ip string, port int, key string, milestone func(int) error) probeOutcome {
	out := probeOutcome{status: "success", result: "unavailable"}
	conn, err := probeDial(ctx, ip, port)
	if err != nil {
		return out
	}
	defer func() { _ = conn.Close() }()
	callCtx, cancel := context.WithTimeout(ctx, 625*time.Second)
	defer cancel()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return probeOutcome{status: "error", err: err}
	}
	alpn := "http/1.1"
	if key == "trojan_alpn" || key == "trojan_short" {
		alpn = "h2"
	}
	client := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{alpn}})
	if err := client.HandshakeContext(callCtx); err != nil {
		return out
	}
	stop := context.AfterFunc(callCtx, func() { _ = conn.Close() })
	defer stop()
	if err := client.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return probeOutcome{status: "error", err: err}
	}
	started := time.Now()
	if err := writeTrojanRequest(client, key); err != nil {
		if key == "trojan_overbuffer" && (errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)) {
			return probeOutcome{status: "success", result: "detected", findings: []ProbeFinding{{Protocol: "trojan", Confidence: "observed", Evidence: "overbuffer_peer_close"}}}
		}
		return out
	}
	return finishTrojanOutcome(observeTrojan(client, key, client.ConnectionState().NegotiatedProtocol, started, time.Now, milestone))
}
