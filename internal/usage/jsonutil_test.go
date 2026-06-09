package usage

import "testing"

func TestBasenameSessionStripsCodexRolloutTimestamp(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/x/sessions/2026/06/08/rollout-2026-06-08T15-02-11-0196f3a1-9d2c-4b1e-8f00-aa11bb22cc33.jsonl", "0196f3a1-9d2c-4b1e-8f00-aa11bb22cc33"},
		{"rollout-2026-05-18T09-24-00-abc.jsonl", "abc"},
		// Non-rollout names pass through with only the extension stripped.
		{"/x/project/session.jsonl", "session"},
		{"plain.json", "plain"},
		// Too few parts to be a rollout timestamp: left alone.
		{"rollout-extra.jsonl", "rollout-extra"},
	}
	for _, c := range cases {
		if got := basenameSession(c.path); got != c.want {
			t.Errorf("basenameSession(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
