package core

import "testing"

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := map[string]string{"type": "offer", "sdp": "v=0"}
	enc, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err := Decode(enc, &out); err != nil {
		t.Fatal(err)
	}
	if out["sdp"] != "v=0" || out["type"] != "offer" {
		t.Fatalf("unexpected: %v", out)
	}
}

func TestDecodeInvalid(t *testing.T) {
	var out map[string]string
	if err := Decode("!!!not-base64", &out); err == nil {
		t.Fatal("expected base64 error")
	}
	if err := Decode("bm90LWpzb24=", &out); err == nil { // "not-json"
		t.Fatal("expected json error")
	}
}
