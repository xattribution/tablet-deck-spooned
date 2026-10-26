package speech

import (
	"regexp"
	"strings"
)

// Normalize lowercases text and keeps only letters, digits and apostrophes, with
// numbers spelled out where speech models write digits.
func Normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	space := true
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '\''
		if !ok {
			if !space {
				b.WriteByte(' ')
				space = true
			}
			continue
		}
		b.WriteRune(r)
		space = false
	}
	words := strings.Fields(b.String())
	for i, w := range words {
		if n, ok := digitWords[w]; ok {
			words[i] = n
		}
	}
	return strings.Join(words, " ")
}

var digitWords = map[string]string{"0": "zero", "1": "one", "2": "two", "3": "three", "4": "four", "5": "five",
	"6": "six", "7": "seven", "8": "eight", "9": "nine", "10": "ten"}

// Wake words: "computer", and the ways speech models tend to mishear it.
var wakeRe = regexp.MustCompile(`^(hey |ok |okay )?(computer|computers|compute her|komputer|computa|computor)\b\s*`)

// SplitWake returns the command after the wake word, and whether there was one.
// wake is the configured wake word ("computer" uses the variants above).
func SplitWake(text, wake string) (string, bool) {
	t := Normalize(text)
	wake = Normalize(wake)
	if wake == "" || wake == "computer" {
		if loc := wakeRe.FindStringIndex(t); loc != nil {
			return strings.TrimSpace(t[loc[1]:]), true
		}
		return t, false
	}
	for _, p := range []string{"hey " + wake, "ok " + wake, wake} {
		if t == p || strings.HasPrefix(t, p+" ") {
			return strings.TrimSpace(strings.TrimPrefix(t, p)), true
		}
	}
	return t, false
}

// Match finds the phrase closest to what was said. It scores each phrase by edit
// distance on the whole text and by shared words, so "deploy the landing gear"
// still finds "deploy landing gear" and "sheilds up" finds "shields up".
func Match(said string, phrases []string) (best string, score float64) {
	s := Normalize(said)
	if s == "" {
		return "", 0
	}
	sw := strings.Fields(stripFiller(s))
	for _, p := range phrases {
		pn := Normalize(p)
		sc := similarity(s, pn)
		if f := stripFiller(s); f != s {
			sc = max(sc, similarity(f, pn))
		}
		sc = max(sc, wordScore(sw, strings.Fields(pn)))
		if sc > score || (sc == score && len(pn) > len(best)) {
			best, score = p, sc
		}
	}
	return best, score
}

var filler = map[string]bool{"the": true, "a": true, "an": true, "please": true, "my": true, "our": true, "to": true, "now": true, "can": true, "you": true, "would": true, "could": true}

func stripFiller(s string) string {
	var out []string
	for _, w := range strings.Fields(s) {
		if !filler[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// similarity is 1 - edit distance / longer length.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[lb])/float64(max(la, lb))
}

// wordScore: how well the phrase's words are covered by what was said (each word
// fuzzily), less a little for extra words said.
func wordScore(said, phrase []string) float64 {
	if len(said) == 0 || len(phrase) == 0 {
		return 0
	}
	var got float64
	for _, pw := range phrase {
		bestW := 0.0
		for _, sw := range said {
			bestW = max(bestW, similarity(sw, pw))
		}
		if bestW >= 0.75 {
			got += bestW
		}
	}
	cover := got / float64(len(phrase))
	extra := float64(max(0, len(said)-len(phrase))) * 0.06
	return max(0, cover-extra)
}
