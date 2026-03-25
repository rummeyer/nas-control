package main

import (
	"testing"
)

// ---------- buildEchoRequest ----------

func TestBuildEchoRequestLength(t *testing.T) {
	msg := buildEchoRequest(0x1234, 1)
	if len(msg) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(msg))
	}
}

func TestBuildEchoRequestType(t *testing.T) {
	msg := buildEchoRequest(1, 1)
	if msg[0] != 8 {
		t.Fatalf("expected type 8 (echo request), got %d", msg[0])
	}
	if msg[1] != 0 {
		t.Fatalf("expected code 0, got %d", msg[1])
	}
}

func TestBuildEchoRequestID(t *testing.T) {
	id := 0xABCD
	msg := buildEchoRequest(id, 1)
	gotID := int(msg[4])<<8 | int(msg[5])
	if gotID != id {
		t.Fatalf("expected ID 0x%04X, got 0x%04X", id, gotID)
	}
}

func TestBuildEchoRequestSeq(t *testing.T) {
	seq := 42
	msg := buildEchoRequest(1, seq)
	gotSeq := int(msg[6])<<8 | int(msg[7])
	if gotSeq != seq {
		t.Fatalf("expected seq %d, got %d", seq, gotSeq)
	}
}

// ---------- icmpChecksum ----------

func TestIcmpChecksumValid(t *testing.T) {
	// A valid ICMP packet's checksum should verify to zero.
	msg := buildEchoRequest(1, 1)
	if cs := icmpChecksum(msg); cs != 0 {
		t.Fatalf("checksum over valid packet should be 0, got 0x%04X", cs)
	}
}

func TestIcmpChecksumChangesWithData(t *testing.T) {
	a := buildEchoRequest(1, 1)
	b := buildEchoRequest(2, 1)
	// Different IDs should produce different checksums (stored in bytes 2-3).
	if a[2] == b[2] && a[3] == b[3] {
		t.Fatal("different packets should have different checksums")
	}
}

// ---------- isEchoReply ----------

func TestIsEchoReplyDirect(t *testing.T) {
	// Simulate a raw ICMP Echo Reply (no IP header).
	id := 0x1234
	buf := make([]byte, 8)
	buf[0] = 0 // Type: Echo Reply
	buf[1] = 0
	buf[4] = byte(id >> 8)
	buf[5] = byte(id)

	if !isEchoReply(buf, id) {
		t.Fatal("expected isEchoReply to return true for matching reply")
	}
}

func TestIsEchoReplyWithIPHeader(t *testing.T) {
	// Simulate a reply preceded by a 20-byte IPv4 header.
	id := 0x5678
	buf := make([]byte, 28)
	buf[20] = 0 // Type: Echo Reply at offset 20
	buf[24] = byte(id >> 8)
	buf[25] = byte(id)

	if !isEchoReply(buf, id) {
		t.Fatal("expected isEchoReply to detect reply after IP header")
	}
}

func TestIsEchoReplyWrongID(t *testing.T) {
	buf := make([]byte, 8)
	buf[0] = 0
	buf[4] = 0x00
	buf[5] = 0x01

	if isEchoReply(buf, 0x9999) {
		t.Fatal("should not match a different ID")
	}
}

func TestIsEchoReplyWrongType(t *testing.T) {
	// Type 8 = Echo Request, not a reply.
	id := 0x1234
	buf := make([]byte, 8)
	buf[0] = 8
	buf[4] = byte(id >> 8)
	buf[5] = byte(id)

	if isEchoReply(buf, id) {
		t.Fatal("echo request (type 8) should not be treated as reply")
	}
}

func TestIsEchoReplyTooShort(t *testing.T) {
	if isEchoReply([]byte{0, 0}, 1) {
		t.Fatal("buffer too short should return false")
	}
}

// ---------- ping (integration-ish) ----------

func TestPingLocalhost(t *testing.T) {
	// Localhost should always be reachable (via ICMP or TCP fallback).
	// Skip in restricted CI environments if needed.
	if !ping("127.0.0.1", 2e9) {
		t.Skip("ping to localhost failed — may need elevated privileges or network access")
	}
}

func TestPingUnreachable(t *testing.T) {
	// RFC 5737 TEST-NET address — guaranteed not to be routable.
	config.NAS.URL = "http://192.0.2.1:1"
	if ping("192.0.2.1", 500e6) {
		t.Fatal("ping to TEST-NET address should return false")
	}
}

func TestPingInvalidHost(t *testing.T) {
	if ping("this.host.does.not.exist.invalid", 500e6) {
		t.Fatal("ping to unresolvable host should return false")
	}
}
