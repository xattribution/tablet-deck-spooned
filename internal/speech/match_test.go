package speech

import "testing"

func TestSplitWake(t *testing.T) {
	for _, c := range []struct {
		in, cmd string
		woke    bool
	}{
		{"Computer, landing gear.", "landing gear", true},
		{"Hey computer deploy the landing gear", "deploy the landing gear", true},
		{"Compute her shields up", "shields up", true},
		{"computer", "", true},
		{"landing gear", "landing gear", false},
		{"my computer is slow", "my computer is slow", false},
	} {
		cmd, woke := SplitWake(c.in, "computer")
		if cmd != c.cmd || woke != c.woke {
			t.Errorf("%q: got %q %v", c.in, cmd, woke)
		}
	}
	if cmd, ok := SplitWake("Jarvis, open the map", "jarvis"); !ok || cmd != "open the map" {
		t.Errorf("custom wake word: %q %v", cmd, ok)
	}
}

func TestMatch(t *testing.T) {
	phrases := []string{"landing gear", "deploy landing gear", "shields up", "shields down", "open star map", "map",
		"quantum drive", "engage quantum", "emote wave", "full power to shields", "request landing"}
	for _, c := range []struct{ said, want string }{
		{"deploy the landing gear please", "deploy landing gear"},
		{"Sheilds up", "shields up"},
		{"open the star map", "open star map"},
		{"engage quantum", "engage quantum"},
		{"full power to the shields", "full power to shields"},
		{"request landing", "request landing"},
	} {
		got, score := Match(c.said, phrases)
		if got != c.want || score < 0.7 {
			t.Errorf("%q: got %q (%.2f), want %q", c.said, got, score, c.want)
		}
	}
	if got, score := Match("what time is it", phrases); score >= 0.7 {
		t.Errorf("unrelated speech matched %q (%.2f)", got, score)
	}
}

func TestForSpeech(t *testing.T) {
	if got := ForSpeech("Shields + to 3"); got != "Shields up to three" {
		t.Errorf("got %q", got)
	}
}
