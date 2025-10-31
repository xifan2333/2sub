package jianying

import (
	"strings"

	"github.com/xifan2333/2sub/pkgs/asr"
)

// parse converts JianYing's raw response to standardized format
func parse(response map[string]interface{}) (*asr.StandardResult, error) {
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		return nil, &ParseError{Message: "missing data field in response"}
	}

	utterancesRaw, ok := data["utterances"].([]interface{})
	if !ok {
		return nil, &ParseError{Message: "missing utterances field in data"}
	}

	result := &asr.StandardResult{
		Words:     make([]asr.Word, 0),
		Sentences: make([]asr.Sentence, 0),
	}

	// Extract language information (if available)
	if attr, ok := data["attribute"].(map[string]interface{}); ok {
		if extra, ok := attr["extra"].(map[string]interface{}); ok {
			if lang, ok := extra["language"].(string); ok {
				result.Language = lang
			}
		}
	}

	var textParts []string

	// Traverse all utterances
	for _, uttRaw := range utterancesRaw {
		utt, ok := uttRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract text
		text, _ := utt["text"].(string)
		startTimeUtt, _ := utt["start_time"].(float64)
		endTimeUtt, _ := utt["end_time"].(float64)

		if text != "" {
			textParts = append(textParts, text)

			// Add sentence-level information
			sentence := asr.Sentence{
				Text:  text,
				Start: int64(startTimeUtt), // already in milliseconds
				End:   int64(endTimeUtt),
			}

			// Extract speaker for utterance level (if available)
			if attr, ok := utt["attribute"].(map[string]interface{}); ok {
				if speaker, ok := attr["speaker"].(string); ok && speaker != "" {
					sentence.SpeakerID = speaker
				}
			}

			result.Sentences = append(result.Sentences, sentence)
		}

		// Extract words
		wordsRaw, ok := utt["words"].([]interface{})
		if !ok {
			continue
		}

		for _, wordRaw := range wordsRaw {
			word, ok := wordRaw.(map[string]interface{})
			if !ok {
				continue
			}

			wordText, _ := word["text"].(string)
			startTime, _ := word["start_time"].(float64)
			endTime, _ := word["end_time"].(float64)

			wordTiming := asr.Word{
				Text:  wordText,
				Start: int64(startTime), // already in milliseconds
				End:   int64(endTime),
			}

			// Extract speaker information for word level (if available)
			if attr, ok := word["attribute"].(map[string]interface{}); ok {
				if speaker, ok := attr["speaker"].(string); ok && speaker != "" {
					wordTiming.SpeakerID = speaker
				}
			}

			result.Words = append(result.Words, wordTiming)
		}
	}

	result.Text = strings.Join(textParts, "")

	if len(result.Words) == 0 {
		return nil, &ParseError{Message: "no words found in response"}
	}

	return result, nil
}
