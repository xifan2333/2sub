package bijian

import (
	"strings"

	"github.com/xifan2333/2sub/pkgs/asr"
)

// parse converts Bijian's raw response to standardized format
func parse(response map[string]interface{}) (*asr.StandardResult, error) {
	utterancesRaw, ok := response["utterances"].([]interface{})
	if !ok {
		return nil, &ParseError{Message: "missing utterances field in response"}
	}

	result := &asr.StandardResult{
		Words:     make([]asr.Word, 0),
		Sentences: make([]asr.Sentence, 0),
	}

	var textParts []string

	// Traverse all utterances
	for _, uttRaw := range utterancesRaw {
		utt, ok := uttRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract text
		transcript, _ := utt["transcript"].(string)
		startTimeUtt, _ := utt["start_time"].(float64)
		endTimeUtt, _ := utt["end_time"].(float64)

		if transcript != "" {
			textParts = append(textParts, transcript)

			// Add sentence-level information
			result.Sentences = append(result.Sentences, asr.Sentence{
				Text:  transcript,
				Start: int64(startTimeUtt), // already in milliseconds
				End:   int64(endTimeUtt),
			})
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

			label, _ := word["label"].(string)
			startTime, _ := word["start_time"].(float64)
			endTime, _ := word["end_time"].(float64)

			result.Words = append(result.Words, asr.Word{
				Text:  label,
				Start: int64(startTime), // already in milliseconds
				End:   int64(endTime),
			})
		}
	}

	result.Text = strings.Join(textParts, "")

	if len(result.Words) == 0 {
		return nil, &ParseError{Message: "no words found in response"}
	}

	return result, nil
}
