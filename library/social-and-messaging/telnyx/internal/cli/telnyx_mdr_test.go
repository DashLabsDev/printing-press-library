package cli

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"2075551234":     "+12075551234",
		"1-207-555-1234": "+12075551234",
		"(207) 555-1234": "+12075551234",
		"+442071234567":  "+442071234567",
		"":               "",
	}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSMSSegments(t *testing.T) {
	long := make([]byte, 161)
	for i := range long {
		long[i] = 'a'
	}
	cases := []struct {
		text string
		want int
	}{
		{"hi", 1},
		{string(long[:160]), 1},
		{string(long), 2},
		{"héllo", 1},
		{string([]rune("é")) + string(long[:70]), 2},
	}
	for _, c := range cases {
		if got := smsSegments(c.text); got != c.want {
			t.Errorf("smsSegments(len %d) = %d, want %d", len(c.text), got, c.want)
		}
	}
}

func TestDecodeMDRDirection(t *testing.T) {
	in, ok := decodeMDR("a", `{"direction":"inbound","cli":"+15550001","cld":"+15559999","status":"received","cost":"0.0040","parts":2}`)
	if !ok || in.Ours != "+15559999" || in.Counterpart != "+15550001" || in.Parts != 2 || in.Cost != 0.004 {
		t.Fatalf("inbound decode wrong: %+v", in)
	}
	out, ok := decodeMDR("b", `{"direction":"outbound","cli":"+15559999","cld":"+15550001","status":"delivery_failed","errors":[{"code":"40008"}]}`)
	if !ok || out.Ours != "+15559999" || out.Counterpart != "+15550001" || len(out.ErrorCodes) != 1 || out.ErrorCodes[0] != "40008" {
		t.Fatalf("outbound decode wrong: %+v", out)
	}
	if _, ok := decodeMDR("c", "not json"); ok {
		t.Fatal("expected decode failure on bad json")
	}
}

func TestIsFailedStatus(t *testing.T) {
	for _, s := range []string{"failed", "delivery_failed", "SENDING_FAILED", "delivery_unconfirmed"} {
		if !isFailedStatus(s) {
			t.Errorf("%s should be failed", s)
		}
	}
	for _, s := range []string{"delivered", "received", "sent", "queued"} {
		if isFailedStatus(s) {
			t.Errorf("%s should not be failed", s)
		}
	}
}
