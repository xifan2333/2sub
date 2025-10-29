package elevenlabs

import (
	"github.com/xifan2333/2sub/asr"
)

// parse converts ElevenLabs's raw response to standardized format
func parse(response map[string]interface{}) (*asr.StandardResult, error) {
	// Extract complete text
	text, ok := response["text"].(string)
	if !ok {
		return nil, &ParseError{Message: "missing text field in response"}
	}

	wordsRaw, ok := response["words"].([]interface{})
	if !ok {
		return nil, &ParseError{Message: "missing words field in response"}
	}

	result := &asr.StandardResult{
		Text:  text,
		Words: make([]asr.Word, 0, len(wordsRaw)),
	}

	// Extract language information (if available)
	if langCode, ok := response["language_code"].(string); ok {
		result.Language = langCode
	}

	// Traverse all words
	for _, wordRaw := range wordsRaw {
		word, ok := wordRaw.(map[string]interface{})
		if !ok {
			continue
		}

		wordText, _ := word["text"].(string)
		start, _ := word["start"].(float64)
		end, _ := word["end"].(float64)

		wordTiming := asr.Word{
			Text:  wordText,
			Start: int64(start * 1000), // convert seconds to milliseconds
			End:   int64(end * 1000),
		}

		// Extract speaker information (if available)
		if speakerID, ok := word["speaker_id"].(string); ok && speakerID != "" {
			wordTiming.SpeakerID = speakerID
		}

		result.Words = append(result.Words, wordTiming)
	}

	if len(result.Words) == 0 {
		return nil, &ParseError{Message: "no words found in response"}
	}

	return result, nil
}
