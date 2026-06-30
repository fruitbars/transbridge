package main

// Request/response types matching the server's API

type SimpleTranslateRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type SimpleTranslateResponse struct {
	Code       int    `json:"code"`
	Data       string `json:"data"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type DeepLTranslateRequest struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
	SourceLang string   `json:"source_lang,omitempty"`
}

type DeepLTranslation struct {
	Text                   string `json:"text"`
	DetectedSourceLanguage string `json:"detected_source_language,omitempty"`
}

type DeepLTranslateResponse struct {
	Translations []DeepLTranslation `json:"translations"`
}

type BatchTranslateRequest struct {
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
	TextList   []string `json:"text_list"`
}

type BatchTranslateItem struct {
	Index              int    `json:"index"`
	DetectedSourceLang string `json:"detected_source_lang"`
	Text               string `json:"text"`
	Error              string `json:"error,omitempty"`
}

type BatchTranslateResponse struct {
	Code         int                   `json:"code"`
	Translations []*BatchTranslateItem `json:"translations"`
}

type OpenAIChatRequest struct {
	Model    string                   `json:"model"`
	Messages []OpenAIChatMessage      `json:"messages"`
	Stream   bool                     `json:"stream,omitempty"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []OpenAIChatChoice  `json:"choices"`
}

type OpenAIChatChoice struct {
	Index   int                `json:"index"`
	Message OpenAIChatMessage  `json:"message"`
}
