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
	out := probeOutcome{status: "success"}
	conn, err := probeDial(ctx, ip, port)
	if err != nil {
		out.status = "error"
		return out
	}
	defer func() { _ = conn.Close() }()
	callCtx, cancel := context.WithTimeout(ctx, 625*time.Second)
	defer cancel()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	alpn := "http/1.1"
	if key == "trojan_alpn" || key == "trojan_short" {
		alpn = "h2"
	}
	client := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{alpn}})
	if err := client.HandshakeContext(callCtx); err != nil {
		out.status = "error"
		return out
	}
	stop := context.AfterFunc(callCtx, func() { _ = conn.Close() })
	defer stop()
	if err := client.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return probeOutcome{status: "error"}
	}
	started := time.Now()
	if err := writeTrojanRequest(client, key); err != nil {
		if key == "trojan_overbuffer" && (errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)) {
			return probeOutcome{status: "success", findings: []ProbeFinding{{Protocol: "trojan", Confidence: "suspected", Evidence: "overbuffer_peer_close"}}}
		}
		return probeOutcome{status: "error"}
	}
	return observeTrojan(client, key, client.ConnectionState().NegotiatedProtocol, started, time.Now, milestone)
}
