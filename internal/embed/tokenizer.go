package embed

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	metaChar = "▁"
)

type Tokenizer interface {
	Encode(text string, maxLength int) (inputIDs, attentionMask, tokenTypeIDs []int64)
}

type UnigramTokenizer struct {
	vocab       map[string]int32
	scores      map[string]float64
	unkID       int32
	unkScore    float64
	bosID       int32
	eosID       int32
	padID       int32
	maxTokenLen int
}

func NewTokenizer(vocabPath string) (*UnigramTokenizer, error) {
	data, err := os.ReadFile(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer file: %w", err)
	}

	var raw struct {
		Model struct {
			Type  string          `json:"type"`
			Vocab [][]interface{} `json:"vocab"`
			UnkID int             `json:"unk_id"`
		} `json:"model"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse tokenizer.json: %w", err)
	}

	if raw.Model.Type != "Unigram" {
		return nil, fmt.Errorf("expected Unigram model, got %s", raw.Model.Type)
	}

	t := &UnigramTokenizer{
		vocab:  make(map[string]int32, len(raw.Model.Vocab)),
		scores: make(map[string]float64, len(raw.Model.Vocab)),
		unkID:  int32(raw.Model.UnkID),
	}

	maxLen := 0
	for i, entry := range raw.Model.Vocab {
		if len(entry) < 2 {
			continue
		}
		tok, ok1 := entry[0].(string)
		if !ok1 {
			continue
		}
		score, ok2 := entry[1].(float64)
		if !ok2 {
			continue
		}
		id := int32(i)
		t.vocab[tok] = id
		t.scores[tok] = score

		switch tok {
		case "<s>":
			t.bosID = id
		case "</s>":
			t.eosID = id
		case "<pad>":
			t.padID = id
		}

		tokLen := len([]rune(tok))
		if tokLen > maxLen {
			maxLen = tokLen
		}
	}
	t.maxTokenLen = maxLen

	t.unkScore = t.scores[t.unkToken()]

	return t, nil
}

func (t *UnigramTokenizer) unkToken() string {
	for tok, id := range t.vocab {
		if id == t.unkID {
			return tok
		}
	}
	return "<unk>"
}

func (t *UnigramTokenizer) Encode(text string, maxLength int) (inputIDs, attentionMask, tokenTypeIDs []int64) {
	normalized := norm.NFKC.String(strings.ToLower(text))

	preTokens := sentencePiecePreTokenize(normalized)

	var allTokenIDs []int32
	for _, preTok := range preTokens {
		ids := t.viterbi(preTok)
		allTokenIDs = append(allTokenIDs, ids...)
	}

	maxSeq := maxLength - 2
	if len(allTokenIDs) > maxSeq {
		allTokenIDs = allTokenIDs[:maxSeq]
	}

	ids := make([]int64, 0, len(allTokenIDs)+2)
	ids = append(ids, int64(t.bosID))
	for _, id := range allTokenIDs {
		ids = append(ids, int64(id))
	}
	ids = append(ids, int64(t.eosID))

	inputIDs = make([]int64, maxLength)
	attentionMask = make([]int64, maxLength)
	tokenTypeIDs = make([]int64, maxLength)

	for i, id := range ids {
		if i >= maxLength {
			break
		}
		inputIDs[i] = id
		attentionMask[i] = 1
	}

	for i := len(ids); i < maxLength; i++ {
		inputIDs[i] = int64(t.padID)
	}

	return inputIDs, attentionMask, tokenTypeIDs
}

func sentencePiecePreTokenize(text string) []string {
	if len(text) == 0 {
		return nil
	}

	runes := []rune(" " + text)
	var sb strings.Builder
	sb.Grow(len(runes))
	for _, r := range runes {
		if r == ' ' {
			sb.WriteString(metaChar)
		} else {
			sb.WriteRune(r)
		}
	}
	joined := sb.String()

	var preTokens []string
	for _, tok := range splitOnWhitespace(joined) {
		if tok != "" {
			preTokens = append(preTokens, tok)
		}
	}
	return preTokens
}

func splitOnWhitespace(s string) []string {
	var result []string
	var cur []rune
	for _, r := range s {
		if unicode.IsSpace(r) {
			if len(cur) > 0 {
				result = append(result, string(cur))
				cur = cur[:0]
			}
		} else {
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		result = append(result, string(cur))
	}
	return result
}

func (t *UnigramTokenizer) viterbi(input string) []int32 {
	runes := []rune(input)
	n := len(runes)
	if n == 0 {
		return nil
	}

	dp := make([]float64, n+1)
	back := make([]int, n+1)
	backID := make([]int32, n+1)

	for i := 1; i <= n; i++ {
		dp[i] = math.Inf(-1)
	}
	dp[0] = 0

	for i := 0; i < n; i++ {
		if math.IsInf(dp[i], -1) {
			continue
		}
		triedAny := false
		for l := 1; l <= t.maxTokenLen && i+l <= n; l++ {
			sub := string(runes[i : i+l])
			if id, ok := t.vocab[sub]; ok {
				triedAny = true
				score := t.scores[sub]
				newScore := dp[i] + score
				if newScore > dp[i+l] {
					dp[i+l] = newScore
					back[i+l] = l
					backID[i+l] = id
				}
			}
		}
		if !triedAny {
			dp[i+1] = dp[i] + t.unkScore
			back[i+1] = 1
			backID[i+1] = t.unkID
		}
	}

	var tokens []int32
	capacity := 0
	for pos := n; pos > 0; {
		capacity++
		l := back[pos]
		if l == 0 {
			l = 1
		}
		pos -= l
	}
	tokens = make([]int32, 0, capacity)

	for pos := n; pos > 0; {
		l := back[pos]
		if l == 0 {
			tokens = append(tokens, t.unkID)
			pos--
		} else {
			tokens = append(tokens, backID[pos])
			pos -= l
		}
	}

	for i, j := 0, len(tokens)-1; i < j; i, j = i+1, j-1 {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}

	return tokens
}
