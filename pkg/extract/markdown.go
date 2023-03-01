package extract

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
)

func Markdownify(text string) string {
	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(text)
	if err != nil {
		return text
	}
	return markdown
}
