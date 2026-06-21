package internal

import "strings"

// underscores escaped so Discord doesn't render them as markdown
const blankPlaceholder = `\_\_\_\_\_`

func formatAnswer(text string) string {
	return "__**" + text + "**__"
}

func displayBlack(text string) string {
	return underscoreRun.ReplaceAllString(text, blankPlaceholder)
}

func fillBlack(text string, answers []string) string {
	i := 0
	filled := underscoreRun.ReplaceAllStringFunc(text, func(string) string {
		if i < len(answers) {
			a := answers[i]
			i++
			return formatAnswer(a)
		}
		return blankPlaceholder
	})
	for ; i < len(answers); i++ {
		filled = strings.TrimRight(filled, " ") + " " + formatAnswer(answers[i])
	}
	return filled
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
