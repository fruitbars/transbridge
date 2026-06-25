// translator/translator.go
package translator

import "context"

// Translator 定义翻译器接口
type Translator interface {
	Translate(ctx context.Context, promptTemplate, text, sourceLang, targetLang string) (string, error)
	GetAPIURL() string
	GetModel() string
	GetProvider() string
	Close() error
}
