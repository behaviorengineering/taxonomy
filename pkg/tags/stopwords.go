package tags

import (
	"sync"

	"github.com/OpenSystemsLab/stopwords"
)

const stopwordLang = "en"

var (
	stopwordsOnce sync.Once
	stopwordsErr  error
	extraStop     = map[string]struct{}{}
)

func ensureEnglishStopwords() error {
	stopwordsOnce.Do(func() {
		if err := stopwords.RegisterLanguage(stopwordLang); err != nil {
			stopwordsErr = newErr("tags.ensureEnglishStopwords", CodeFailed, "register language", err)
		}
	})
	return stopwordsErr
}

func isStopword(token string) bool {
	token = normalizeToken(token)
	if token == "" {
		return false
	}
	if _, ok := extraStop[token]; ok {
		return true
	}
	if err := ensureEnglishStopwords(); err != nil {
		return false
	}
	return stopwords.IsStopWord(stopwordLang, token)
}
