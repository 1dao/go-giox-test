package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/richtext"

	"gioui.org/x/markdown"
)

var (
	regularFace *opentype.Face
	boldFace    *opentype.Face
	italicFace  *opentype.Face
)

func loadFont(fontPath string) (*opentype.Face, error) {
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取字体文件 %s, %w", fontPath, err)
	}
	face, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("无法解析字体文件: %s, %w", fontPath, err)
	}
	return &face, nil
}

func loadFonts() error {
	var err error
	// 加载思源黑体常规体
	regularFace, err = loadFont("fonts/SourceHanSansSC-Regular.otf")
	if err != nil {
		return fmt.Errorf("无法加载常规字体: %v", err)
	}

	// 加载思源黑体粗体
	boldFace, err = loadFont("fonts/SourceHanSansSC-Bold.otf")
	if err != nil {
		return fmt.Errorf("无法加载粗体: %v", err)
	}

	// 加载江城斜黑作为斜体替代
	italicFace, err = loadFont("fonts/JiangChengItalicBold400W.ttf")
	if err != nil {
		return fmt.Errorf("无法加载斜体: %v", err)
	}
	return nil
}

func configureShaper() *text.Shaper {
	return text.NewShaper(
		text.NoSystemFonts(),
		text.WithCollection([]font.FontFace{
			{
				Font: font.Font{Typeface: "Source Han Sans", Weight: font.Normal},
				Face: *regularFace,
			},
			{
				Font: font.Font{Typeface: "Source Han Sans", Weight: font.Bold},
				Face: *boldFace,
			},
			{
				Font: font.Font{Typeface: "Source Han Sans", Weight: font.Normal, Style: font.Italic},
				Face: *italicFace,
			},
		}),
	)
}

func main() {
	go func() {
		// 加载字体
		if err := loadFonts(); err != nil {
			log.Fatalf("加载字体失败: %v", err)
		}

		// 配置 Shaper
		shaper := configureShaper()

		// 创建窗口
		w := &app.Window{}
		// 初始化主题
		th := material.NewTheme()
		th.Shaper = shaper

		// 创建 Markdown 渲染器
		renderer := markdown.NewRenderer()
		renderer.Config = markdown.Config{
			DefaultFont:      font.Font{Weight: font.Normal, Style: font.Regular},
			DefaultSize:      unit.Sp(16),
			DefaultColor:     th.Palette.Fg,
			InteractiveColor: th.Palette.ContrastBg,
		}

		// 定义Markdown内容
		markdownContent := `
# Gio Markdown示例

## 基本功能展示 
- 支持**粗体**和*斜体* *test* **xxxx**文本 
- 支持[链接](https://gioui.org) 
- 支持代码块: 

` + "```go\n" + `func main() { 
    fmt.Println("Hello, Gio!") 
} 
` + "```\n" + `
> 引用内容
`

		// 渲染 Markdown 内容为 richtext.SpanStyle
		spans, err := renderer.Render([]byte(markdownContent))
		if err != nil {
			log.Fatalf("Failed to render markdown: %v", err)
		}

		// 自定义代码块的样式
		var styledSpans []richtext.SpanStyle
		for _, span := range spans {
			if span.Font.Typeface == "monospace" { // 假设代码块使用等宽字体
				// 拆分代码块内容为关键字和非关键字
				styledSpans = append(styledSpans, splitCodeContent(span, th)...)
			} else {
				styledSpans = append(styledSpans, span)
			}
		}

		// 创建 RichText 组件
		var interactiveText richtext.InteractiveText
		richText := richtext.Text(&interactiveText, th.Shaper, styledSpans...)

		var ops op.Ops
		for {
			e := w.Event()
			switch e := e.(type) {
			case app.DestroyEvent:
				return
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)

				// 处理交互事件
				for {
					span, event, ok := interactiveText.Update(gtx)
					if !ok {
						break
					}
					if event.Type == richtext.Click && event.ClickData.Kind == gesture.KindClick {
						// 获取链接URL
						if url := span.Get(markdown.MetadataURL); ok {
							if link, ok := url.(string); ok {
								// 打开链接
								openURL(link)
							}
						}
					}
				}

				// 布局窗口内容
				layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// 布局Markdown内容
						return layout.Inset{Top: 20, Left: 20, Right: 20, Bottom: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return richText.Layout(gtx)
						})
					}),
				)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

// splitCodeContent 将代码块内容拆分为关键字、符号和字符串，并为它们设置不同的样式
func splitCodeContent(span richtext.SpanStyle, th *material.Theme) []richtext.SpanStyle {
	keywords := []string{"func", "main", "Println"}
	var styledSpans []richtext.SpanStyle

	// 拆分内容
	parts := splitByKeywords(span.Content, keywords)

	for _, part := range parts {
		ns := span
		switch {
		case part.IsKeyword:
			ns.Color = th.Palette.ContrastBg // 关键字颜色
		case isSymbol(part.Text):
			ns.Color = color.NRGBA{0xFF, 0, 0, 0xFF} //"#FF0000" // 符号颜色（红色）
		case isString(part.Text):
			ns.Color = color.NRGBA{0xFF, 0x69, 0xB4, 0xFF} //"#FF69B4" // 字符串颜色（粉色）
		default:
			ns.Color = th.Palette.Fg // 默认颜色
		}
		ns.Content = part.Text
		styledSpans = append(styledSpans, ns)
	}
	return styledSpans
}

// splitByKeywords 将内容按空格或符号拆分为关键字、符号和字符串部分，同时保留分隔符
func splitByKeywords(content string, keywords []string) []struct {
	Text      string
	IsKeyword bool
} {
	var result []struct {
		Text      string
		IsKeyword bool
	}

	// 定义分隔符
	separators := " \t\n().,;{}"

	// 遍历内容，逐字符处理
	token := ""
	for _, r := range content {
		if strings.ContainsRune(separators, r) {
			// 如果遇到分隔符，先处理当前的 token
			if token != "" {
				result = append(result, classifyToken(token, keywords))
				token = ""
			}
			// 将分隔符作为单独的部分添加
			result = append(result, struct {
				Text      string
				IsKeyword bool
			}{Text: string(r), IsKeyword: false})
		} else {
			// 累积非分隔符字符
			token += string(r)
		}
	}

	// 处理最后一个 token
	if token != "" {
		result = append(result, classifyToken(token, keywords))
	}

	return result
}

// classifyToken 判断一个 token 是否是关键字
func classifyToken(token string, keywords []string) struct {
	Text      string
	IsKeyword bool
} {
	isKeyword := false
	for _, keyword := range keywords {
		if token == keyword {
			isKeyword = true
			break
		}
	}
	return struct {
		Text      string
		IsKeyword bool
	}{Text: token, IsKeyword: isKeyword}
}

// isSymbol 判断一个文本是否是符号
func isSymbol(text string) bool {
	symbols := "(),;{}"
	return len(text) == 1 && strings.ContainsRune(symbols, rune(text[0]))
}

// isString 判断一个文本是否是字符串
func isString(text string) bool {
	return strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"")
}

// openURL 打开链接，支持 Windows、Android 和 iOS
func openURL(link string) {
	switch runtime.GOOS {
	case "windows":
		// Windows 平台
		exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Start()
	case "darwin":
		// macOS/iOS 平台
		exec.Command("open", link).Start()
	case "linux", "android":
		// Linux/Android 平台
		exec.Command("xdg-open", link).Start()
	default:
		log.Printf("无法打开链接: %s", link)
	}
}
