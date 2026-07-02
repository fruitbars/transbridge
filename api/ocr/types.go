package ocr

// ElementType 是调用方声明的 OCR 元素类型。当前主要参考 PP-Structure 的枚举。
// 服务端识别的类型见 handler 里的 routing 表；未知类型退化到 "text"。
type ElementType string

const (
	ElementText          ElementType = "text"           // 普通段落
	ElementTitle         ElementType = "title"          // 标题
	ElementList          ElementType = "list"           // 列表项
	ElementTable         ElementType = "table"          // 表格（配合 content_format=html）
	ElementTableCaption  ElementType = "table_caption"  // 表题
	ElementFigureCaption ElementType = "figure_caption" // 图题
	ElementHeader        ElementType = "header"         // 页眉
	ElementFooter        ElementType = "footer"         // 页脚
	ElementFootnote      ElementType = "footnote"       // 脚注
	ElementReference     ElementType = "reference"      // 参考文献条目
	ElementFigure        ElementType = "figure"         // 图（一般无文本，跳过）
	ElementEquation      ElementType = "equation"       // 数学公式
)

// ContentFormat 声明 element.content 的格式，决定服务端的处理路径。
type ContentFormat string

const (
	FormatText     ContentFormat = "text"     // 纯文本
	FormatMarkdown ContentFormat = "markdown" // markdown（透传给模型，prompt 追加保留语法要求）
	FormatHTML     ContentFormat = "html"     // HTML（当前仅表格类型支持）
)

// OCRElement 是请求中的单个待处理块。
type OCRElement struct {
	ID            string        `json:"id"`
	Type          ElementType   `json:"type"`
	ContentFormat ContentFormat `json:"content_format,omitempty"`
	Content       string        `json:"content"`
}

// OCRRequest /ocr/translate 请求体
type OCRRequest struct {
	SourceLang string       `json:"source_lang,omitempty"`
	TargetLang string       `json:"target_lang"`
	Elements   []OCRElement `json:"elements"`
}

// OCRTranslation 单个元素的翻译结果
type OCRTranslation struct {
	ID            string        `json:"id"`
	Type          ElementType   `json:"type"`
	ContentFormat ContentFormat `json:"content_format,omitempty"`
	Content       string        `json:"content"`         // 原格式的译文；未翻译时等于原文
	Translated    bool          `json:"translated"`      // 是否真的经过了模型翻译
	Reason        string        `json:"reason,omitempty"` // 未翻译或部分翻译时的原因
	Error         string        `json:"error,omitempty"` // 翻译失败时的错误
}

// OCRResponse /ocr/translate 响应体
type OCRResponse struct {
	Code         int              `json:"code"`
	Translations []OCRTranslation `json:"translations"`
}
