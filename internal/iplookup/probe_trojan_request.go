package iplookup

import "io"

func writeTrojanRequest(client io.Writer, key string) error {
	payload := "GET / HTTP/1.1\r\nHost: www.test.com\r\n\r\n"
	switch key {
	case "trojan_short":
		payload = "GET / HTTP/1.1"
	case "trojan_incomplete":
		payload = "GET / HTTP/1.1\r\n"
	case "trojan_close":
		payload = "GET / HTTP/1.1\r\nHost: www.test.com\r\nConnection: close\r\n\r\n"
	case "trojan_overbuffer":
		payload = "GET / HTTP/1.1\r\nHost: www.test.com\r\n"
	}
	if _, err := io.WriteString(client, payload); err != nil {
		return err
	}
	if key == "trojan_overbuffer" {
		for range trojanHeaderRepeats {
			if _, err := io.WriteString(client, trojanRepeatedHeader); err != nil {
				return err
			}
		}
	}
	return nil
}
