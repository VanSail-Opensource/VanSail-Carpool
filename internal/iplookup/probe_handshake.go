package iplookup

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

func handshakeProbe(ctx context.Context, ip string, port int, kind string) probeOutcome {
	out := probeOutcome{status: "success"}
	conn, err := probeDial(ctx, ip, port)
	if err != nil {
		out.status = "error"
		return out
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		out.status = "error"
		return out
	}
	payload := []byte{5, 2, 0, 2}
	if kind == "socks4" {
		payload = []byte{4, 0, 0, 0, 0, 0, 0, 0, 0}
	}
	if kind == "http" {
		payload = []byte("CONNECT probe.invalid:0 HTTP/1.1\r\nHost: probe.invalid:0\r\nConnection: close\r\n\r\n")
	}
	if _, err := conn.Write(payload); err != nil {
		return out
	}
	if kind == "http" {
		response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(conn, 8192)), nil)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 407 && response.Header.Get("Proxy-Authenticate") != "" {
				out.findings = []ProbeFinding{{Protocol: "http", Confidence: "confirmed", Evidence: "authentication_required"}}
			} else {
				out.findings = probeWeb(ctx, ip, port, "http", true)
			}
		}
		return out
	}
	response := make([]byte, 8)
	length := 2
	if kind == "socks4" {
		length = 8
	}
	_, err = io.ReadFull(conn, response[:length])
	if err != nil {
		return out
	}
	if kind == "socks5" && response[0] == 5 && (response[1] == 0 || response[1] == 2) {
		evidence := "no_authentication"
		if response[1] == 2 {
			evidence = "authentication_required"
		}
		out.findings = []ProbeFinding{{Protocol: kind, Confidence: "confirmed", Evidence: evidence}}
	}
	if kind == "socks4" && response[0] == 0 && response[1] >= 90 && response[1] <= 93 {
		out.findings = []ProbeFinding{{Protocol: kind, Confidence: "confirmed", Evidence: "protocol_response"}}
	}
	return out
}

func shadowsocksProbe(ctx context.Context, ip string, port int) probeOutcome {
	// Length-dependent close/reset behavior from IMC20 is a fingerprint, not proof.
	lengths := []int{7, 8, 9, 15, 16, 17, 32, 33, 34, 48, 49, 50}
	closed := map[int]bool{}
	for _, length := range lengths {
		conn, err := probeDial(ctx, ip, port)
		if err != nil {
			return probeOutcome{status: "error"}
		}
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		payload := make([]byte, length)
		if _, err := randomRead(payload); err != nil {
			_ = conn.Close()
			return probeOutcome{status: "error"}
		}
		if _, err := conn.Write(payload); err != nil {
			_ = conn.Close()
			continue
		}
		response := make([]byte, 256)
		n, err := conn.Read(response)
		_ = conn.Close()
		if n > 0 {
			return probeOutcome{status: "success"}
		}
		closed[length] = err == io.EOF || (err != nil && strings.Contains(strings.ToLower(err.Error()), "reset"))
		if ctx.Err() != nil {
			return probeOutcome{status: "error"}
		}
	}
	boundaries := 0
	for _, middle := range []int{8, 16, 33, 49} {
		if closed[middle-1] != closed[middle+1] {
			boundaries++
		}
	}
	out := probeOutcome{status: "success"}
	if boundaries >= 2 {
		out.findings = []ProbeFinding{{Protocol: "shadowsocks", Confidence: "suspected", Evidence: "length_dependent_close"}}
	}
	return out
}
