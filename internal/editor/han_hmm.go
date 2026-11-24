package editor

import (
	"bufio"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// State order in CppJieba's hmm_model.utf8.
const (
	hanBegin = iota
	hanEnd
	hanMiddle
	hanSingle
	hanStateCount
)

var hanPreviousStates = [hanStateCount][2]uint8{
	hanBegin:  {hanEnd, hanSingle},
	hanMiddle: {hanBegin, hanMiddle},
	hanEnd:    {hanBegin, hanMiddle},
	hanSingle: {hanEnd, hanSingle},
}

type hanHMM struct {
	start      [hanStateCount]float64
	transition [hanStateCount][hanStateCount]float64
	emissions  []hanEmission
}

type hanEmission struct {
	char   rune
	scores [hanStateCount]float32
}

func loadHanHMM(path string) *hanHMM {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// The upstream model stores every character for a state on one line.
	scanner.Buffer(nil, 1<<20)
	line := func() string {
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text != "" && !strings.HasPrefix(text, "#") {
				return text
			}
		}
		return ""
	}
	model := &hanHMM{}
	if !parseHanScores(line(), &model.start) {
		return nil
	}
	for state := range model.transition {
		if !parseHanScores(line(), &model.transition[state]) {
			return nil
		}
	}
	weights := make(map[rune][hanStateCount]float32)
	impossible := float32(math.Inf(-1))
	for state := range hanStateCount {
		for entry := range strings.SplitSeq(line(), ",") {
			char, value, ok := strings.Cut(entry, ":")
			r, n := utf8.DecodeRuneInString(char)
			score, err := strconv.ParseFloat(value, 32)
			if !ok || r == utf8.RuneError || n != len(char) || err != nil || math.IsNaN(score) {
				return nil
			}
			scores, present := weights[r]
			if !present {
				scores = [hanStateCount]float32{impossible, impossible, impossible, impossible}
			}
			scores[state] = float32(score)
			weights[r] = scores
		}
	}
	model.emissions = make([]hanEmission, 0, len(weights))
	for r, scores := range weights {
		model.emissions = append(model.emissions, hanEmission{char: r, scores: scores})
	}
	sort.Slice(model.emissions, func(i, j int) bool { return model.emissions[i].char < model.emissions[j].char })
	return model
}

func parseHanScores(text string, dest *[hanStateCount]float64) bool {
	fields := strings.Fields(text)
	if len(fields) != len(dest) {
		return false
	}
	for i, field := range fields {
		score, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(score) {
			return false
		}
		if score <= -3.14e100 {
			score = math.Inf(-1)
		}
		dest[i] = score
	}
	return true
}

func (m *hanHMM) observation(r rune) [hanStateCount]float64 {
	index := sort.Search(len(m.emissions), func(i int) bool { return m.emissions[i].char >= r })
	if index == len(m.emissions) || m.emissions[index].char != r {
		// Unseen characters are constrained to single-character words.
		impossible := math.Inf(-1)
		return [hanStateCount]float64{impossible, impossible, impossible, 0}
	}
	var scores [hanStateCount]float64
	for state, score := range m.emissions[index].scores {
		scores[state] = float64(score)
	}
	return scores
}

// refine uses Viterbi decoding to merge singletons in a run. A failed decode
// leaves the dictionary's boundaries intact.
func (m *hanHMM) refine(runes []rune, offsets []uint8) {
	previous := m.observation(runes[0])
	for state := range previous {
		previous[state] += m.start[state]
	}
	trace := make([][hanStateCount]uint8, len(runes))
	for i := 1; i < len(runes); i++ {
		scores := m.observation(runes[i])
		for state, allowed := range hanPreviousStates {
			from, alternative := allowed[0], allowed[1]
			best := previous[from] + m.transition[from][state]
			if score := previous[alternative] + m.transition[alternative][state]; score >= best {
				from, best = alternative, score
			}
			trace[i][state] = from
			scores[state] += best
		}
		previous = scores
	}
	state := uint8(hanSingle)
	if previous[hanEnd] > previous[hanSingle] {
		state = hanEnd
	}
	if math.IsInf(previous[state], -1) {
		return
	}
	end := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		switch state {
		case hanEnd:
			end = i + 1
		case hanBegin:
			if end-i <= maxHanWordRunes {
				for j := i + 1; j < end; j++ {
					offsets[j] = uint8(j - i)
				}
			}
		}
		state = trace[i][state]
	}
}
