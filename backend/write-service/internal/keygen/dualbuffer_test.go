package keygen

import (
	"testing"
)

func TestFeistelCipherObfuscate(t *testing.T) {
	id1 := uint64(1)
	id2 := uint64(2)

	obf1 := FeistelCipherObfuscate(id1)
	obf2 := FeistelCipherObfuscate(id2)

	if obf1 == id1 {
		t.Errorf("FeistelCipherObfuscate(1) = %d; want different value", obf1)
	}

	if obf1 == obf2 {
		t.Errorf("FeistelCipherObfuscate(1) and FeistelCipherObfuscate(2) produced same value: %d", obf1)
	}

	// Verify it's within 64-bit space (always true for uint64, but let's check it's not zero if input is not zero)
	if obf1 == 0 {
		t.Errorf("FeistelCipherObfuscate(1) = 0; want non-zero")
	}
}

func TestBase62Encode(t *testing.T) {
	tests := []struct {
		id   uint64
		want string
	}{
		{0, "00000000000"},
		{1, "00000000001"},
		{61, "0000000000z"},
		{62, "00000000010"},
		{12345, "000000003D7"},
		{18446744073709551615, "LygHa16AHYF"},
	}

	for _, tt := range tests {
		if got := Base62Encode(tt.id); got != tt.want {
			t.Errorf("Base62Encode(%d) = %s; want %s", tt.id, got, tt.want)
		}
	}
}

func TestBase62Decode(t *testing.T) {
	tests := []struct {
		s    string
		want uint64
	}{
		{"00000000000", 0},
		{"00000000001", 1},
		{"0000000000z", 61},
		{"00000000010", 62},
		{"000000003D7", 12345},
		{"LygHa16AHYF", 18446744073709551615},
	}

	for _, tt := range tests {
		got, err := Base62Decode(tt.s)
		if err != nil {
			t.Errorf("Base62Decode(%s) error: %v", tt.s, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Base62Decode(%s) = %d; want %d", tt.s, got, tt.want)
		}
	}
}
