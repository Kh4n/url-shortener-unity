package util

import (
	"testing"
)

func TestBase62Encode(t *testing.T) {
	var enc []byte
	base62Encode(62, &enc)
	if string(enc) != "01" {
		t.Errorf("Expected '01' as output, got: %s\n", string(enc))
	}

	base62Encode(61, &enc)
	if string(enc) != "Z" {
		t.Errorf("Expected 'Z' as output, got: %s\n", string(enc))
	}

	base62Encode(53*1+1*62+5*(62*62)+45*(62*62*62), &enc)
	if string(enc) != "R15J" {
		t.Errorf("Expected 'R15J' as output, got: %s\n", string(enc))
	}
}
