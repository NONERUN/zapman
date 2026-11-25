package main

import "testing"

func TestSetTag(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"add the fields",
			"{\n  \"$schema\": \"../schema/strategy.json\",\n  \"spec\": 2,\n  \"rules\": []\n}\n",
			"{\n  \"$schema\": \"../schema/strategy.json\",\n  \"spec\": 2,\n  \"upstream\": \"1.10.5\",\n  \"rev\": 1,\n  \"rules\": []\n}\n",
		},
		{
			"change the fields, CRLF",
			"{\r\n  \"spec\": 2,\r\n  \"upstream\": \"1.10.4\",\r\n  \"rev\": 3,\r\n  \"rules\": []\r\n}\r\n",
			"{\r\n  \"spec\": 2,\r\n  \"upstream\": \"1.10.5\",\r\n  \"rev\": 1,\r\n  \"rules\": []\r\n}\r\n",
		},
	}
	for _, c := range cases {
		got, err := setTag(c.in, "1.10.5")
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	if _, err := setTag("{\n  \"spec\": 2,\n  \"rev\": 2,\n  \"rules\": []\n}\n", "1.10.5"); err == nil {
		t.Error("rev without upstream: no error")
	}
}
