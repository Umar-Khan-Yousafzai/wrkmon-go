package commands

import "testing"

func TestParseSeek(t *testing.T) {
	cases := []struct {
		in      string
		want    SeekSpec
		wantErr bool
	}{
		{"1:23", SeekSpec{SeekAbsolute, 83}, false},
		{"01:02:03", SeekSpec{SeekAbsolute, 3723}, false},
		{"0:00", SeekSpec{SeekAbsolute, 0}, false},
		{"83", SeekSpec{SeekAbsolute, 83}, false},
		{"83.5", SeekSpec{SeekAbsolute, 83.5}, false},
		{"50%", SeekSpec{SeekPct, 50}, false},
		{"0%", SeekSpec{SeekPct, 0}, false},
		{"100%", SeekSpec{SeekPct, 100}, false},
		{"+30", SeekSpec{SeekRelative, 30}, false},
		{"-30", SeekSpec{SeekRelative, -30}, false},
		{"", SeekSpec{}, true},
		{"abc", SeekSpec{}, true},
		{"1:99", SeekSpec{}, true},    // seconds field must be < 60
		{"1:2:3:4", SeekSpec{}, true}, // too many colon fields
		{"150%", SeekSpec{}, true},    // percent must be 0–100
		{"-1:00", SeekSpec{}, true},   // sign + colon form not allowed
		{"-5", SeekSpec{SeekRelative, -5}, false},
		{"nan", SeekSpec{}, true},
		{"NaN%", SeekSpec{}, true},
		{"inf", SeekSpec{}, true},
		{"-inf", SeekSpec{}, true},
		{"+Inf", SeekSpec{}, true},
		{"+1:00", SeekSpec{}, true}, // sign + colon form not allowed
	}
	for _, c := range cases {
		got, err := ParseSeek(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseSeek(%q): expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSeek(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSeek(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
